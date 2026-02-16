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
	// github.com/go-i2p/go-forward/packet
)

var implementUDPServer i2ptunnel.I2PTunnel = &UDPServer{}

type UDPServer struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The local UDP service address
	net.Addr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// Channel for shutdown signaling
	done chan struct{}
	// Ensures Stop() is only executed once to prevent double-close panic
	stopOnce sync.Once
	// Mutex protecting the Errors slice from concurrent access
	errMu sync.Mutex

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}

func (u *UDPServer) recordError(err error) {
	u.errMu.Lock()
	u.Errors = append(u.Errors, i2ptunnel.NewError(u, err))
	u.errMu.Unlock()
}

// Get the tunnel's I2P address
func (u *UDPServer) Address() string {
	// For UDP server, return the service address if available
	if u.Garlic != nil && u.Garlic.ServiceKeys != nil {
		return u.Garlic.ServiceKeys.Addr().Base32()
	}
	return ""
}

// Get the tunnel's error message
func (u *UDPServer) Error() error {
	u.errMu.Lock()
	defer u.errMu.Unlock()
	if len(u.Errors) > 0 {
		return u.Errors[len(u.Errors)-1]
	}
	return nil
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

// Start the tunnel.
// Forwards incoming I2P datagrams to the local UDP service.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (u *UDPServer) Start() error {
	u.done = make(chan struct{})
	u.stopOnce = sync.Once{}
	i2pListener, err := u.Garlic.ListenPacket()
	if err != nil {
		return err
	}
	defer i2pListener.Close()
	defer u.Stop()

	// Resolve target address once before entering the loop
	raddr, err := net.ResolveUDPAddr("udp", u.Target())
	if err != nil {
		return fmt.Errorf("failed to resolve target UDP address: %w", err)
	}

	u.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning
	for {
		select {
		case <-u.done:
			return nil
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

// Get the tunnel's status
func (u *UDPServer) Status() i2ptunnel.I2PTunnelStatus {
	return u.I2PTunnelStatus
}

// Stop the tunnel. Safe to call multiple times.
// Closes the Garlic (I2P SAM session) to release network resources.
func (u *UDPServer) Stop() error {
	u.stopOnce.Do(func() {
		close(u.done)
		if u.Garlic != nil {
			u.Garlic.Close()
		}
	})
	u.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped
	return nil
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (u *UDPServer) Target() string {
	return u.Addr.String()
}

// Get the tunnel's type
func (u *UDPServer) Type() string {
	return u.TunnelConfig.Type
}

// Get the tunnel's ID
func (u *UDPServer) ID() string {
	return i2ptunnel.Clean(u.Name())
}

// Get the tunnel's options
func (u *UDPServer) Options() map[string]string {
	// Return basic configuration options as a map
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

// Set the tunnel's options
func (u *UDPServer) SetOptions(opts map[string]string) error {
	// Apply configuration options from the map with validation
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

// LoadConfig loads tunnel configuration from a file and updates the tunnel settings.
// The tunnel must be stopped before calling LoadConfig to prevent inconsistent state.
// Supported formats: .properties, .ini, .yaml/.yml
func (u *UDPServer) LoadConfig(path string) error {
	// Prevent config changes while tunnel is running to avoid race conditions
	if u.I2PTunnelStatus == i2ptunnel.I2PTunnelStatusRunning ||
		u.I2PTunnelStatus == i2ptunnel.I2PTunnelStatusStarting {
		return fmt.Errorf("cannot load config while tunnel is %s - stop tunnel first", u.I2PTunnelStatus)
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
	if newConfig.Type != "udpserver" {
		return fmt.Errorf("config file contains %s tunnel, expected udpserver", newConfig.Type)
	}

	// Validate target address (local service) before applying changes
	targetAddr, err := net.ResolveUDPAddr("udp", newConfig.Target)
	if err != nil {
		return fmt.Errorf("invalid target address in config: %w", err)
	}

	// Update mutable configuration fields
	u.TunnelConfig = *newConfig
	u.Addr = targetAddr

	return nil
}
