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
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	// github.com/go-i2p/go-forward/packet
)

var implementUDPClient i2ptunnel.I2PTunnel = &UDPClient{}

type UDPClient struct{}

// Address implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Address() string {
	panic("unimplemented")
}

// Error implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Error() error {
	panic("unimplemented")
}

// LocalAddress implements i2ptunnel.I2PTunnel.
func (u *UDPClient) LocalAddress() (string, string, error) {
	panic("unimplemented")
}

// Name implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Name() string {
	panic("unimplemented")
}

// Options implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Options() map[string]string {
	panic("unimplemented")
}

// Start implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Start() error {
	panic("unimplemented")
}

// Status implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Status() i2ptunnel.I2PTunnelStatus {
	panic("unimplemented")
}

// Stop implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Stop() error {
	panic("unimplemented")
}

// Target implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Target() string {
	panic("unimplemented")
}

// Type implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Type() string {
	panic("unimplemented")
}
