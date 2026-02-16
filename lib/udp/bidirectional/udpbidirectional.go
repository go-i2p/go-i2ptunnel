package udpbidirectional

// UDP Bidirectional Tunnel
//
// A UDP bidirectional tunnel combines:
// 1. A UDP server tunnel (forwarding incoming I2P datagrams to a local service)
// 2. A SOCKS5 proxy (allowing local apps to reach arbitrary I2P destinations)
//
// Both sides share the same I2P identity (keys). This is a non-standard mode
// described in the README as using onramp "hybrid2" mode.

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/packet"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	"github.com/go-i2p/onramp"
	"github.com/txthinking/socks5"
)

var implementUDPBidirectional i2ptunnel.I2PTunnel = &UDPBidirectional{}

// UDPBidirectional combines a UDP server tunnel with a SOCKS5 proxy client
// on the same I2P keys, enabling both inbound and outbound I2P connections.
type UDPBidirectional struct {
	// I2P connection (shared for both server and client sides)
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The local UDP service address (forward target for inbound I2P datagrams)
	net.Addr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// SOCKS5 server instance for outbound connections
	socksServer *socks5.Server
	// Channel for shutdown signaling
	done chan struct{}
	// Ensures Stop() is only executed once to prevent double-close panic
	stopOnce sync.Once
	// Mutex protecting lifecycle fields (done, stopOnce) during Start/Stop transitions.
	// Prevents the race where Start() resets stopOnce while Stop() is calling stopOnce.Do().
	lifeMu sync.Mutex
	// Mutex protecting the Errors slice from concurrent access
	errMu sync.Mutex
	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}

func (u *UDPBidirectional) recordError(err error) {
	u.errMu.Lock()
	u.Errors = append(u.Errors, i2ptunnel.NewError(u, err))
	u.errMu.Unlock()
}

// Address returns the tunnel's I2P address.
func (u *UDPBidirectional) Address() string {
	if u.Garlic != nil && u.Garlic.ServiceKeys != nil {
		return u.Garlic.ServiceKeys.Addr().Base32()
	}
	return ""
}

// Error returns the most recent error, or nil.
func (u *UDPBidirectional) Error() error {
	u.errMu.Lock()
	defer u.errMu.Unlock()
	if len(u.Errors) > 0 {
		return u.Errors[len(u.Errors)-1]
	}
	return nil
}

// LocalAddress returns the SOCKS5 proxy listen address.
func (u *UDPBidirectional) LocalAddress() (string, error) {
	addr := net.JoinHostPort(u.TunnelConfig.Interface, strconv.Itoa(u.TunnelConfig.Port))
	return addr, nil
}

// Name returns the tunnel's configured name.
func (u *UDPBidirectional) Name() string {
	return u.TunnelConfig.Name
}

// Start launches both the server-side I2P datagram listener and the client-side
// SOCKS5 proxy concurrently. It blocks until the tunnel is stopped.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (u *UDPBidirectional) Start() error {
	u.lifeMu.Lock()
	u.done = make(chan struct{})
	u.stopOnce = sync.Once{}
	u.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStarting
	u.lifeMu.Unlock()

	// Start the server side: listen for I2P datagrams
	i2pListener, err := u.Garlic.ListenPacket()
	if err != nil {
		return fmt.Errorf("failed to start I2P datagram listener: %w", err)
	}
	defer i2pListener.Close()
	defer u.Stop()

	// Create SOCKS5 proxy for outbound connections
	socksAddr := net.JoinHostPort(u.TunnelConfig.Interface, strconv.Itoa(u.TunnelConfig.Port))
	socksServer, err := socks5.NewClassicServer(socksAddr, "", "", "", 0, 0)
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 server: %w", err)
	}
	u.socksServer = socksServer
	u.socksServer.Handle = &socksHandler{garlic: u.Garlic}

	// Start SOCKS5 proxy in a background goroutine
	socksErrCh := make(chan error, 1)
	go func() {
		socksErrCh <- u.socksServer.ListenAndServe(u.socksServer.Handle)
	}()

	u.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning

	// Resolve target address once before entering the loop
	raddr, err := net.ResolveUDPAddr("udp", u.Target())
	if err != nil {
		return fmt.Errorf("failed to resolve target UDP address: %w", err)
	}

	// Server-side datagram forwarding loop
	for {
		select {
		case <-u.done:
			return nil
		case err := <-socksErrCh:
			if err != nil {
				u.recordError(err)
			}
			return err
		default:
			lCon, err := net.DialUDP("udp", nil, raddr)
			if err != nil {
				select {
				case <-u.done:
					return nil
				default:
				}
				time.Sleep(50 * time.Millisecond)
				continue
			}
			func() {
				defer lCon.Close()
				ctx := context.Background()
				packet.Forward(ctx, i2pListener, lCon, config.DefaultConfig())
			}()
			// Brief pause between forwarding attempts to prevent rapid socket
			// churn when packet.Forward returns quickly (e.g., on error or timeout).
			select {
			case <-u.done:
				return nil
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
}

// Status returns the current tunnel status.
func (u *UDPBidirectional) Status() i2ptunnel.I2PTunnelStatus {
	return u.I2PTunnelStatus
}

// Stop gracefully shuts down both the server and SOCKS5 proxy sides.
// Safe to call multiple times.
// Closes the Garlic (I2P SAM session) to release network resources.
func (u *UDPBidirectional) Stop() error {
	u.lifeMu.Lock()
	defer u.lifeMu.Unlock()
	u.stopOnce.Do(func() {
		close(u.done)
		if u.socksServer != nil {
			u.socksServer.Shutdown()
		}
		if u.Garlic != nil {
			u.Garlic.Close()
		}
	})
	u.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped
	return nil
}

// Target returns the local service address for inbound I2P forwarding.
func (u *UDPBidirectional) Target() string {
	return u.Addr.String()
}

// Type returns the tunnel type identifier.
func (u *UDPBidirectional) Type() string {
	return u.TunnelConfig.Type
}

// ID returns a clean identifier derived from the tunnel name.
func (u *UDPBidirectional) ID() string {
	return i2ptunnel.Clean(u.Name())
}

// Options returns the tunnel's configuration as a string map.
func (u *UDPBidirectional) Options() map[string]string {
	options := make(map[string]string)
	options["name"] = u.TunnelConfig.Name
	options["type"] = u.TunnelConfig.Type
	options["interface"] = u.TunnelConfig.Interface
	options["port"] = strconv.Itoa(u.TunnelConfig.Port)
	if u.Addr != nil {
		options["target"] = u.Addr.String()
	}
	return options
}

// SetOptions applies configuration options from a string map with validation.
func (u *UDPBidirectional) SetOptions(opts map[string]string) error {
	if name, ok := opts["name"]; ok {
		if err := validate.RequiredString("name", name); err != nil {
			return err
		}
		u.TunnelConfig.Name = name
	}
	if iface, ok := opts["interface"]; ok {
		if err := validate.Interface(iface); err != nil {
			return err
		}
		u.TunnelConfig.Interface = iface
	}
	if portStr, ok := opts["port"]; ok {
		port, err := validate.PortString(portStr)
		if err != nil {
			return err
		}
		u.TunnelConfig.Port = port
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

// LoadConfig loads tunnel configuration from a file. The tunnel must be stopped first.
func (u *UDPBidirectional) LoadConfig(path string) error {
	if u.I2PTunnelStatus == i2ptunnel.I2PTunnelStatusRunning ||
		u.I2PTunnelStatus == i2ptunnel.I2PTunnelStatusStarting {
		return fmt.Errorf("cannot load config while tunnel is %s - stop tunnel first", u.I2PTunnelStatus)
	}

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

	if newConfig.Type != "udpbidirectional" {
		return fmt.Errorf("config file contains %s tunnel, expected udpbidirectional", newConfig.Type)
	}

	targetAddr, err := net.ResolveUDPAddr("udp", newConfig.Target)
	if err != nil {
		return fmt.Errorf("invalid target address in config: %w", err)
	}

	u.TunnelConfig = *newConfig
	u.Addr = targetAddr
	return nil
}
