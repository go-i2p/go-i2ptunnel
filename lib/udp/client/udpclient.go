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
	"strconv"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/i2pkeys"
	"github.com/go-i2p/onramp"
	// github.com/go-i2p/go-forward/packet
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
}

// Get the tunnel's I2P address
func (u *UDPClient) Address() string {
	return u.Garlic.B32()
}

// Get the tunnel's error message
func (u *UDPClient) Error() error {
	panic("unimplemented")
}

// Get the tunnel's local host:port
func (u *UDPClient) LocalAddress() (string, string, error) {
	return u.TunnelConfig.Interface, strconv.Itoa(u.TunnelConfig.Port), nil
}

// Get the tunnel's name
func (u *UDPClient) Name() string {
	return u.TunnelConfig.Name
}

// Get the tunnel's options
func (u *UDPClient) Options() map[string]string {
	return u.TunnelConfig.Options()
}

// Start the tunnel
func (u *UDPClient) Start() error {
	panic("unimplemented")
}

// Get the tunnel's status
func (u *UDPClient) Status() i2ptunnel.I2PTunnelStatus {
	return u.I2PTunnelStatus
}

// Stop the tunnel
func (u *UDPClient) Stop() error {
	panic("unimplemented")
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (u *UDPClient) Target() string {
	return u.I2PAddr.Base32()
}

// Get the tunnel's type
func (u *UDPClient) Type() string {
	return u.TunnelConfig.Type
}
