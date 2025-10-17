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

	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/packet"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
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

// Start the tunnel
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
			defer lCon.Close()
			ctx := context.Background()
			packet.Forward(ctx, i2pConnection.(*datagram.DatagramSession), lCon, config.DefaultConfig())
		}
	}
}

// Get the tunnel's status
func (u *UDPClient) Status() i2ptunnel.I2PTunnelStatus {
	return u.I2PTunnelStatus
}

// Stop the tunnel
func (u *UDPClient) Stop() error {
	close(u.done)
	// Cleanup resources
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
	// Apply configuration options from the map
	if name, ok := opts["name"]; ok {
		u.TunnelConfig.Name = name
	}
	if iface, ok := opts["interface"]; ok {
		u.TunnelConfig.Interface = iface
	}
	if portStr, ok := opts["port"]; ok {
		if port, err := strconv.Atoi(portStr); err == nil {
			u.TunnelConfig.Port = port
		} else {
			return fmt.Errorf("invalid port value: %s", portStr)
		}
	}
	if target, ok := opts["target"]; ok {
		addr, err := i2pkeys.Lookup(target)
		if err != nil {
			return fmt.Errorf("invalid target address: %w", err)
		}
		u.I2PAddr = addr
	}
	return nil
}

// Load the tunnel config from file
func (u *UDPClient) LoadConfig(path string) error {
	// For now, return an error indicating this method needs configuration file support
	return fmt.Errorf("LoadConfig not yet implemented: would load configuration from %s", path)
}
