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
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	// github.com/go-i2p/go-forward/packet
)

var implementUDPServer i2ptunnel.I2PTunnel = &UDPServer{}

type UDPServer struct{}

// Address implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Address() string {
	panic("unimplemented")
}

// Error implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Error() error {
	panic("unimplemented")
}

// LocalAddress implements i2ptunnel.I2PTunnel.
func (u *UDPServer) LocalAddress() (string, string, error) {
	panic("unimplemented")
}

// Name implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Name() string {
	panic("unimplemented")
}

// Options implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Options() map[string]string {
	panic("unimplemented")
}

// Start implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Start() error {
	panic("unimplemented")
}

// Status implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Status() i2ptunnel.I2PTunnelStatus {
	panic("unimplemented")
}

// Stop implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Stop() error {
	panic("unimplemented")
}

// Target implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Target() string {
	panic("unimplemented")
}

// Type implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Type() string {
	panic("unimplemented")
}
