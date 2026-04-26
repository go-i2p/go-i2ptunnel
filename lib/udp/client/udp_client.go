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
	"strconv"
	"sync"
	"time"

	"github.com/go-i2p/go-forward/packet"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
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
	// TunnelBase provides Name, ID, Type, Status, Error, SetTunnelMetrics, SetStatus, RecordError, and the common Options/SetOptions keys.
	i2ptunnel.TunnelBase
	// The remote I2P destination target
	*i2pkeys.I2PAddr
	// Channel for shutdown signaling
	done chan struct{}
	// Ensures Stop() is only executed once to prevent double-close panic
	stopOnce sync.Once
	// Mutex protecting lifecycle fields (done, stopOnce) during Start/Stop transitions.
	// Prevents the race where Start() resets stopOnce while Stop() is calling stopOnce.Do().
	lifeMu sync.Mutex
}

// Get the tunnel's I2P address
func (u *UDPClient) Address() string {
	// For UDP client, return our own I2P address if available
	if u.Garlic != nil && u.Garlic.ServiceKeys != nil {
		return u.Garlic.ServiceKeys.Addr().Base32()
	}
	return ""
}

// Get the tunnel's local host:port
func (u *UDPClient) LocalAddress() (string, error) {
	addr := net.JoinHostPort(u.TunnelConfig.Interface, strconv.Itoa(u.TunnelConfig.Port))
	return addr, nil
}

// maxConsecutiveForwardErrors is the number of consecutive packet.Forward failures
// before the tunnel transitions to I2PTunnelStatusFailed.
const maxConsecutiveForwardErrors = 10

// Start the tunnel.
// Forwards UDP packets between local sockets and the I2P datagram session.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (u *UDPClient) Start() error {
	u.lifeMu.Lock()
	u.done = make(chan struct{})
	u.stopOnce = sync.Once{}
	done := u.done
	u.SetStatus(i2ptunnel.I2PTunnelStatusStarting)
	u.lifeMu.Unlock()
	i2pConnection, err := u.Garlic.Dial("udp", u.Target())
	if err != nil {
		return err
	}
	defer i2pConnection.Close()
	defer u.Stop()

	raddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(u.TunnelConfig.Interface, strconv.Itoa(u.TunnelConfig.Port)))
	if err != nil {
		return fmt.Errorf("failed to resolve local UDP address: %w", err)
	}
	lCon, err := net.ListenUDP("udp", raddr)
	if err != nil {
		return fmt.Errorf("failed to listen on local UDP address: %w", err)
	}
	defer lCon.Close()

	u.SetStatus(i2ptunnel.I2PTunnelStatusRunning)
	if u.Metrics != nil {
		u.Metrics.RecordStart()
	}
	consecutiveErrors := 0
	for {
		select {
		case <-done:
			return nil
		default:
			if cont, fatal := u.forwardOnce(i2pConnection, lCon, &consecutiveErrors, done); !cont {
				return fatal
			}
		}
	}
}

// forwardOnce runs one iteration of packet forwarding and handles errors.
func (u *UDPClient) forwardOnce(i2pConnection net.Conn, lCon *net.UDPConn, consecutiveErrors *int, done chan struct{}) (cont bool, fatal error) {
	fwdCfg := udpconst.NewDatagramForwardConfig()
	fwdCfg.ShutdownSignal = done
	if err := packet.Forward(context.Background(), i2pConnection.(*datagram.DatagramSession), metrics.WrapPacketConn(lCon, u.Metrics), fwdCfg); err != nil {
		*consecutiveErrors++
		u.RecordError(fmt.Errorf("forward error (%d consecutive): %w", *consecutiveErrors, err))
		if *consecutiveErrors >= maxConsecutiveForwardErrors {
			u.SetStatus(i2ptunnel.I2PTunnelStatusFailed)
			return false, fmt.Errorf("tunnel failed after %d consecutive forward errors", *consecutiveErrors)
		}
	} else {
		*consecutiveErrors = 0
	}
	select {
	case <-done:
		return false, nil
	case <-time.After(100 * time.Millisecond):
	}
	return true, nil
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
		u.SetStatus(i2ptunnel.I2PTunnelStatusStopped)
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

// Options returns the tunnel's configuration as a string map.
// Note: the "max-conns" and "rate-limit" keys are included for config
// round-trip compatibility only — they are not enforced at runtime because
// UDP tunnels operate on net.PacketConn (datagrams), not net.Listener.
func (u *UDPClient) Options() map[string]string {
	options := u.TunnelBase.Options()
	if u.I2PAddr != nil {
		options["target"] = u.I2PAddr.Base32()
	}
	return options
}

// SetOptions applies configuration options from a string map with validation.
// Note: the "max-conns" and "rate-limit" keys are stored for config round-trip
// compatibility but are not enforced — see Options() for details.
func (u *UDPClient) SetOptions(opts map[string]string) error {
	if err := u.TunnelBase.SetOptions(opts); err != nil {
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
func (u *UDPClient) LoadConfig(path string) error {
	if err := i2ptunnel.CheckTunnelStopped(u.Status()); err != nil {
		return err
	}
	newConfig, err := i2ptunnel.ParseConfigFile(path)
	if err != nil {
		return err
	}
	if newConfig.Type != "udpclient" {
		return fmt.Errorf("config file contains %s tunnel, expected udpclient", newConfig.Type)
	}
	addr, err := i2pkeys.Lookup(newConfig.Target)
	if err != nil {
		return fmt.Errorf("invalid target address in config: %w", err)
	}
	u.TunnelConfig = *newConfig
	u.I2PAddr = addr
	return nil
}
