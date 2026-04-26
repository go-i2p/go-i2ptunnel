package udpserver

/**
UDP Server Tunnels
------------------

UDP Server Tunnels accept incoming I2P datagrams and forward the UDP packets to a specified local port. This enables:

- Running UDP services accessible through I2P
- Hosting game servers that use UDP protocols
- Providing access to local UDP services via I2P
- Simple packet forwarding without protocol awareness

Key features:
* One-to-one UDP packet forwarding
* No packet inspection or modification
* Stateless operation
* Local port binding for service

When an I2P peer connects to the tunnel's destination, the traffic flows:
- Incoming: I2P Network → I2P Service → UDP Packet → Local Service
- Outgoing: Local Service → UDP Packet → I2P Service → I2P Network
**/

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/go-i2p/go-forward/packet"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
	udpconst "github.com/go-i2p/go-i2ptunnel/lib/udp/const"
	"github.com/go-i2p/onramp"
	// github.com/go-i2p/go-forward/packet
)

var implementUDPServer i2ptunnel.I2PTunnel = &UDPServer{}

// UDPServer is a UDP server tunnel that accepts datagrams from the I2P network
// and forwards them to a local UDP service, relaying replies back.
type UDPServer struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// TunnelBase provides Name, ID, Type, Status, Error, SetTunnelMetrics, SetStatus, RecordError, and the common Options/SetOptions keys.
	i2ptunnel.TunnelBase
	// The local UDP service address
	net.Addr
	// Channel for shutdown signaling
	done chan struct{}
	// Ensures Stop() is only executed once to prevent double-close panic
	stopOnce sync.Once
	// Mutex protecting lifecycle fields (done, stopOnce) during Start/Stop transitions.
	// Prevents the race where Start() resets stopOnce while Stop() is calling stopOnce.Do().
	lifeMu sync.Mutex
}

// Get the tunnel's I2P address
func (u *UDPServer) Address() string {
	// For UDP server, return the service address if available
	if u.Garlic != nil && u.Garlic.ServiceKeys != nil {
		return u.Garlic.ServiceKeys.Addr().Base32()
	}
	return ""
}

// Get the tunnel's local host:port
func (u *UDPServer) LocalAddress() (string, error) {
	addr := net.JoinHostPort(u.TunnelConfig.Interface, strconv.Itoa(u.TunnelConfig.Port))
	return addr, nil
}

// Get the tunnel's name
func (u *UDPServer) Name() string {
	return u.TunnelConfig.Name
}

// maxConsecutiveErrors is the number of consecutive DialUDP failures before
// the tunnel transitions to I2PTunnelStatusFailed.
const maxConsecutiveErrors = 10

// Start the tunnel.
// Forwards incoming I2P datagrams to the local UDP service.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (u *UDPServer) Start() error {
	u.lifeMu.Lock()
	u.done = make(chan struct{})
	u.stopOnce = sync.Once{}
	done := u.done
	u.SetStatus(i2ptunnel.I2PTunnelStatusStarting)
	u.lifeMu.Unlock()
	i2pListener, err := u.Garlic.ListenPacket()
	if err != nil {
		return err
	}
	defer i2pListener.Close()
	defer u.Stop()

	raddr, err := net.ResolveUDPAddr("udp", u.Target())
	if err != nil {
		return fmt.Errorf("failed to resolve target UDP address: %w", err)
	}

	u.SetStatus(i2ptunnel.I2PTunnelStatusRunning)
	if u.Metrics != nil {
		u.Metrics.RecordStart()
	}
	return u.runForwardLoop(i2pListener, raddr, done)
}

// runForwardLoop runs the main dial-and-forward cycle for inbound I2P datagrams.
func (u *UDPServer) runForwardLoop(i2pListener net.PacketConn, raddr *net.UDPAddr, done chan struct{}) error {
	backoff := udpconst.MinBackoff
	consecutiveErrors := 0
	for {
		select {
		case <-done:
			return nil
		default:
			if cont, fatal := u.dialAndForward(i2pListener, raddr, done, &consecutiveErrors, &backoff); !cont {
				return fatal
			}
		}
	}
}

// dialAndForward dials the upstream UDP target and forwards one session of packets.
func (u *UDPServer) dialAndForward(i2pListener net.PacketConn, raddr *net.UDPAddr, done chan struct{}, consecutiveErrors *int, backoff *time.Duration) (cont bool, fatal error) {
	lCon, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return u.handleDialError(err, consecutiveErrors, backoff, done)
	}
	*backoff = udpconst.MinBackoff
	*consecutiveErrors = 0
	func() {
		defer lCon.Close()
		fwdCfg := udpconst.NewDatagramForwardConfig()
		fwdCfg.ShutdownSignal = done
		packet.Forward(context.Background(), i2pListener, metrics.WrapPacketConn(lCon, u.Metrics), fwdCfg)
	}()
	select {
	case <-done:
		return false, nil
	case <-time.After(100 * time.Millisecond):
		return true, nil
	}
}

// handleDialError processes a DialUDP failure and returns whether to continue.
func (u *UDPServer) handleDialError(err error, consecutiveErrors *int, backoff *time.Duration, done <-chan struct{}) (cont bool, fatal error) {
	select {
	case <-done:
		return false, nil
	default:
	}
	*consecutiveErrors++
	u.RecordError(fmt.Errorf("dial error (%d consecutive): %w", *consecutiveErrors, err))
	if *consecutiveErrors >= maxConsecutiveErrors {
		u.SetStatus(i2ptunnel.I2PTunnelStatusFailed)
		return false, fmt.Errorf("tunnel failed after %d consecutive dial errors", *consecutiveErrors)
	}
	time.Sleep(*backoff)
	*backoff = udpconst.NextBackoff(*backoff)
	return true, nil
}

// Stop the tunnel. Safe to call multiple times.
// Closes the Garlic (I2P SAM session) to release network resources.
func (u *UDPServer) Stop() error {
	u.lifeMu.Lock()
	defer u.lifeMu.Unlock()
	u.stopOnce.Do(func() {
		close(u.done)
		if u.Garlic != nil {
			u.Garlic.Close()
		}
		u.SetStatus(i2ptunnel.I2PTunnelStatusStopped)
		if u.Metrics != nil {
			u.Metrics.RecordStop()
		}
	})
	return nil
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (u *UDPServer) Target() string {
	return u.Addr.String()
}

// Options returns the tunnel's configuration as a string map.
// Note: the "max-conns" and "rate-limit" keys are included for config
// round-trip compatibility only — they are not enforced at runtime because
// UDP tunnels operate on net.PacketConn (datagrams), not net.Listener.
func (u *UDPServer) Options() map[string]string {
	options := u.TunnelBase.Options()
	if u.Addr != nil {
		options["target"] = u.Addr.String()
	}
	return options
}

// SetOptions applies configuration options from a string map with validation.
// Note: the "max-conns" and "rate-limit" keys are stored for config round-trip
// compatibility but are not enforced — see Options() for details.
func (u *UDPServer) SetOptions(opts map[string]string) error {
	if err := u.TunnelBase.SetOptions(opts); err != nil {
		return err
	}
	if target, ok := opts["target"]; ok {
		if err := validate.NetworkAddress(target); err != nil {
			return err
		}
		addr, err := net.ResolveUDPAddr("udp", target)
		if err != nil {
			return fmt.Errorf("invalid target address %q: %w", target, err)
		}
		u.Addr = addr
	}
	return nil
}

// LoadConfig loads tunnel configuration from a file and updates the tunnel settings.
// The tunnel must be stopped before calling LoadConfig to prevent inconsistent state.
func (u *UDPServer) LoadConfig(path string) error {
	if err := i2ptunnel.CheckTunnelStopped(u.Status()); err != nil {
		return err
	}
	newConfig, err := i2ptunnel.ParseConfigFile(path)
	if err != nil {
		return err
	}
	if newConfig.Type != "udpserver" {
		return fmt.Errorf("config file contains %s tunnel, expected udpserver", newConfig.Type)
	}
	targetAddr, err := net.ResolveUDPAddr("udp", newConfig.Target)
	if err != nil {
		return fmt.Errorf("invalid target address in config: %w", err)
	}
	u.TunnelConfig = *newConfig
	u.Addr = targetAddr
	return nil
}
