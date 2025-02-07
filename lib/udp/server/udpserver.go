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
	"net"
	"strconv"

	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/packet"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
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

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}

func (u *UDPServer) recordError(err error) {
	u.Errors = append(u.Errors, i2ptunnel.NewError(u, err))
}

// Get the tunnel's I2P address
func (u *UDPServer) Address() string {
	return u.Garlic.B32()
}

// Get the tunnel's error message
func (u *UDPServer) Error() error {
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

// Get the tunnel's options
func (u *UDPServer) Options() map[string]string {
	return u.TunnelConfig.Options()
}

// Start the tunnel
func (u *UDPServer) Start() error {
	i2pListener, err := u.Garlic.ListenPacket()
	if err != nil {
		return err
	}
	defer i2pListener.Close()
	defer u.Stop()
	u.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning
	for {
		select {
		case <-u.done:
			return nil
		default:
			raddr, err := net.ResolveUDPAddr("udp", u.Target())
			if err != nil {
				continue
			}
			lCon, err := net.DialUDP("udp", nil, raddr)
			if err != nil {
				continue
			}
			defer lCon.Close()
			ctx := context.Background()
			packet.Forward(ctx, i2pListener, lCon, config.DefaultConfig())
		}
	}
}

// Get the tunnel's status
func (u *UDPServer) Status() i2ptunnel.I2PTunnelStatus {
	return u.I2PTunnelStatus
}

// Stop the tunnel
func (u *UDPServer) Stop() error {
	close(u.done)
	// Cleanup resources
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
