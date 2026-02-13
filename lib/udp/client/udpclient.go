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

	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/packet"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	"github.com/go-i2p/go-sam-go/datagram"
	"github.com/go-i2p/i2pkeys"
	"github.com/go-i2p/onramp"
)

var implementUDPClient i2ptunnel.I2PTunnel = &UDPClient{}

type UDPClient struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The remote I2P destination target
	*i2pkeys.I2PAddr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// Channel for shutdown signaling
	done chan struct{}
	// Ensures Stop() is only executed once to prevent double-close panic
	stopOnce sync.Once

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}

func (u *UDPClient) recordError(err error) {
	u.Errors = append(u.Errors, i2ptunnel.NewError(u, err))
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
	if len(u.Errors) > 0 {
		return u.Errors[len(u.Errors)-1]
	}
	return nil
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
func (u *UDPClient) Start() error {
	i2pConnection, err := u.Garlic.Dial("udp", u.Target())
	if err != nil {
		return err
	}
	defer i2pConnection.Close()
	defer u.Stop()
	u.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning
	for {
		select {
		case <-u.done:
			return nil
		default:
			raddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(u.TunnelConfig.Interface, strconv.Itoa(u.TunnelConfig.Port)))
			if err != nil {
				continue
			}
			lCon, err := net.DialUDP("udp", nil, raddr)
			if err != nil {
				continue
			}
			func() {
				defer lCon.Close()
				ctx := context.Background()
				packet.Forward(ctx, i2pConnection.(*datagram.DatagramSession), lCon, config.DefaultConfig())
			}()
		}
	}
}

// Get the tunnel's status
func (u *UDPClient) Status() i2ptunnel.I2PTunnelStatus {
	return u.I2PTunnelStatus
}

// Stop the tunnel. Safe to call multiple times.
func (u *UDPClient) Stop() error {
	u.stopOnce.Do(func() {
		close(u.done)
	})
	u.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped
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
	// Return basic configuration options as a map
	options := make(map[string]string)
	options["name"] = u.TunnelConfig.Name
	options["type"] = u.TunnelConfig.Type
	options["interface"] = u.TunnelConfig.Interface
	options["port"] = strconv.Itoa(u.TunnelConfig.Port)
	if u.I2PAddr != nil {
		options["target"] = u.I2PAddr.Base32()
	}
	return options
}

// Set the tunnel's options
func (u *UDPClient) SetOptions(opts map[string]string) error {
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
