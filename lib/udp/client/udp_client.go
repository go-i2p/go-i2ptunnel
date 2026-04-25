package udpclient

/**
UDP Client Tunnels
------------------

UDP Client Tunnels accept incoming UDP packets and forward them as I2P Datagrams to an I2P destination. This enables:

- Accessing remote I2P UDP services locally
- Connecting to game servers hosted on I2P
- Forwarding local UDP traffic through I2P
- Simple client-side UDP tunneling

Key features:
* Transparent UDP forwarding
* Direct destination addressing
* Local UDP socket binding
* Stateless operation

When sending UDP packets to an I2P service, the traffic flows:
- Outgoing: Local Client → UDP Packet → I2P Client → I2P Network
- Incoming: I2P Network → I2P Client → UDP Packet → Local Client
**/

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-i2p/go-forward/packet"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
	limitedlistener "github.com/go-i2p/go-limit"
	udpconst "github.com/go-i2p/go-i2ptunnel/lib/udp/const"
	"github.com/go-i2p/go-sam-go/datagram"
	"github.com/go-i2p/i2pkeys"
	"github.com/go-i2p/onramp"
)

var implementUDPClient i2ptunnel.I2PTunnel = &UDPClient{}

// UDPClient is a UDP client tunnel that reads datagrams from a local UDP port
// and forwards them to a fixed I2P destination, relaying responses back.
type UDPClient struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The remote I2P destination target
	*i2pkeys.I2PAddr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// The rate-limiting configuration.
	// Note: UDP uses net.PacketConn (datagrams), not net.Listener; MaxConns/RateLimit
	// are persisted here for configuration round-trips.
	limitedlistener.LimitedConfig
	// Channel for shutdown signaling
	done chan struct{}
	// Ensures Stop() is only executed once to prevent double-close panic
	stopOnce sync.Once
	// Mutex protecting lifecycle fields (done, stopOnce) during Start/Stop transitions.
	// Prevents the race where Start() resets stopOnce while Stop() is calling stopOnce.Do().
	lifeMu sync.Mutex
	// Mutex protecting the I2PTunnelStatus field from concurrent read/write access
	statusMu sync.RWMutex
	// ErrorTracker provides bounded error history.
	i2ptunnel.ErrorTracker
	// Metrics tracks live operational data for this tunnel.
	// Set by the webui controller after construction. May be nil.
	Metrics *metrics.TunnelMetrics
}

// SetTunnelMetrics injects a live metrics tracker. Implements metrics.MetricsBearer.
func (u *UDPClient) SetTunnelMetrics(m *metrics.TunnelMetrics) {
	u.Metrics = m
}

func (u *UDPClient) recordError(err error) {
	u.ErrorTracker.Record(u, err)
	if u.Metrics != nil {
		u.Metrics.RecordError()
	}
}

func (u *UDPClient) setStatus(s i2ptunnel.I2PTunnelStatus) {
	u.statusMu.Lock()
	u.I2PTunnelStatus = s
	u.statusMu.Unlock()
}

// Get the tunnel's I2P address
func (u *UDPClient) Address() string {
	// For UDP client, return our own I2P address if available
	if u.Garlic != nil && u.Garlic.ServiceKeys != nil {
		return u.Garlic.ServiceKeys.Addr().Base32()
	}
	return ""
}

// Get the tunnel's error message
func (u *UDPClient) Error() error {
	return u.ErrorTracker.Last()
}

// Get the tunnel's local host:port
func (u *UDPClient) LocalAddress() (string, error) {
	addr := net.JoinHostPort(u.TunnelConfig.Interface, strconv.Itoa(u.TunnelConfig.Port))
	return addr, nil
}

// Get the tunnel's name
func (u *UDPClient) Name() string {
	return u.TunnelConfig.Name
}

// Start the tunnel.
// Forwards UDP packets between local sockets and the I2P datagram session.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (u *UDPClient) Start() error {
	u.lifeMu.Lock()
	u.done = make(chan struct{})
	u.stopOnce = sync.Once{}
	done := u.done // capture local ref before unlock to avoid data race with restart
	u.lifeMu.Unlock()
	i2pConnection, err := u.Garlic.Dial("udp", u.Target())
	if err != nil {
		return err
	}
	defer i2pConnection.Close()
	defer u.Stop()

	// Resolve local address once before entering the loop
	raddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(u.TunnelConfig.Interface, strconv.Itoa(u.TunnelConfig.Port)))
	if err != nil {
		return fmt.Errorf("failed to resolve local UDP address: %w", err)
	}

	// Listen on the configured local address for packets from local applications.
	// net.ListenUDP creates an unconnected socket that can receive from any sender,
	// unlike net.DialUDP which restricts to a single remote address.
	lCon, err := net.ListenUDP("udp", raddr)
	if err != nil {
		return fmt.Errorf("failed to listen on local UDP address: %w", err)
	}
	defer lCon.Close()

	u.setStatus(i2ptunnel.I2PTunnelStatusRunning)
	if u.Metrics != nil {
		u.Metrics.RecordStart()
	}
	for {
		select {
		case <-done:
			return nil
		default:
			fwdCfg := udpconst.NewDatagramForwardConfig()
			fwdCfg.ShutdownSignal = done
			packet.Forward(context.Background(), i2pConnection.(*datagram.DatagramSession), lCon, fwdCfg)
			// packet.Forward returned (error or idle timeout). Retry unless stopped.
			select {
			case <-done:
				return nil
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
}

// Get the tunnel's status
func (u *UDPClient) Status() i2ptunnel.I2PTunnelStatus {
	u.statusMu.RLock()
	defer u.statusMu.RUnlock()
	return u.I2PTunnelStatus
}

// Stop the tunnel. Safe to call multiple times.
// Closes the Garlic (I2P SAM session) to release network resources.
func (u *UDPClient) Stop() error {
	u.lifeMu.Lock()
	defer u.lifeMu.Unlock()
	u.stopOnce.Do(func() {
		close(u.done)
		if u.Garlic != nil {
			u.Garlic.Close()
		}
		u.setStatus(i2ptunnel.I2PTunnelStatusStopped)
		if u.Metrics != nil {
			u.Metrics.RecordStop()
		}
	})
	return nil
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (u *UDPClient) Target() string {
	return u.I2PAddr.Base32()
}

// Get the tunnel's type
func (u *UDPClient) Type() string {
	return u.TunnelConfig.Type
}

// Get the tunnel's ID
func (u *UDPClient) ID() string {
	return i2ptunnel.Clean(u.Name())
}

// Get the tunnel's options
func (u *UDPClient) Options() map[string]string {
	options := i2ptunnel.BuildCommonOptions(u.TunnelConfig)
	i2ptunnel.AddRateLimitOptions(options, u.LimitedConfig.MaxConns, u.LimitedConfig.RateLimit)
	if u.I2PAddr != nil {
		options["target"] = u.I2PAddr.Base32()
	}
	return options
}

// Set the tunnel's options
func (u *UDPClient) SetOptions(opts map[string]string) error {
	if err := i2ptunnel.ApplyCommonOptions(opts, &u.TunnelConfig); err != nil {
		return err
	}
	if err := i2ptunnel.ApplyRateLimitOptions(opts, &u.LimitedConfig.MaxConns, &u.LimitedConfig.RateLimit); err != nil {
		return err
	}
	if target, ok := opts["target"]; ok {
		if err := validate.I2PAddress(target); err != nil {
			return err
		}
		addr, err := i2pkeys.Lookup(target)
		if err != nil {
			return fmt.Errorf("invalid target address: %w", err)
		}
		u.I2PAddr = addr
	}
	return nil
}

// LoadConfig loads tunnel configuration from a file and updates the tunnel settings.
// The tunnel must be stopped before calling LoadConfig to prevent inconsistent state.
// Supported formats: .properties, .ini, .yaml/.yml
func (u *UDPClient) LoadConfig(path string) error {
	// Prevent config changes while tunnel is running to avoid race conditions
	status := u.Status()
	if status == i2ptunnel.I2PTunnelStatusRunning ||
		status == i2ptunnel.I2PTunnelStatusStarting {
		return fmt.Errorf("cannot load config while tunnel is %s - stop tunnel first", status)
	}

	// Parse config file using the converter library
	conv := i2pconv.Converter{}
	format, err := conv.DetectFormat(path)
	if err != nil {
		return fmt.Errorf("failed to detect config format: %w", err)
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	newConfig, err := conv.ParseInput(bytes, format)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Type safety: ensure loaded config matches expected tunnel type
	if newConfig.Type != "udpclient" {
		return fmt.Errorf("config file contains %s tunnel, expected udpclient", newConfig.Type)
	}

	// Validate target address before applying changes
	addr, err := i2pkeys.Lookup(newConfig.Target)
	if err != nil {
		return fmt.Errorf("invalid target address in config: %w", err)
	}

	// Update mutable configuration fields
	u.TunnelConfig = *newConfig
	u.I2PAddr = addr

	return nil
}
