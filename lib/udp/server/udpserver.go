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
	"net"
	"strconv"

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
}

// Get the tunnel's I2P address
func (u *UDPServer) Address() string {
	return u.Garlic.B32()
}

// Get the tunnel's error message
func (u *UDPServer) Error() error {
	panic("unimplemented")
}

// Get the tunnel's local host:port
func (u *UDPServer) LocalAddress() (string, string, error) {
	return u.TunnelConfig.Interface, strconv.Itoa(u.TunnelConfig.Port), nil
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
	panic("unimplemented")
}

// Get the tunnel's status
func (u *UDPServer) Status() i2ptunnel.I2PTunnelStatus {
	return u.I2PTunnelStatus
}

// Stop the tunnel
func (u *UDPServer) Stop() error {
	panic("unimplemented")
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (u *UDPServer) Target() string {
	return u.Addr.String()
}

// Get the tunnel's type
func (u *UDPServer) Type() string {
	return u.TunnelConfig.Type
}
