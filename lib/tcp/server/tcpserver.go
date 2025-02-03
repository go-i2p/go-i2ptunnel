package tcpserver

/**
TCP Server Tunnel
-----------------

A TCP Server tunnel connects a local TCP service to the I2P network through:
1. A TCP Client component that interfaces with the local service
2. An I2P Service component that maintains a persistent destination address

When an I2P peer connects to the tunnel's destination, the traffic flows:
- Incoming: I2P Network → I2P Service → TCP Client → Local Service
- Outgoing: Local Service → TCP Client → I2P Service → I2P Network
**/

import i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"

var implementTCPServer i2ptunnel.I2PTunnel = &TCPServer{}

type TCPServer struct {
}

// Address implements i2ptunnel.I2PTunnel.
func (t *TCPServer) Address() string {
	panic("unimplemented")
}

// Error implements i2ptunnel.I2PTunnel.
func (t *TCPServer) Error() error {
	panic("unimplemented")
}

// LocalAddress implements i2ptunnel.I2PTunnel.
func (t *TCPServer) LocalAddress() (string, string, error) {
	panic("unimplemented")
}

// Name implements i2ptunnel.I2PTunnel.
func (t *TCPServer) Name() string {
	panic("unimplemented")
}

// Options implements i2ptunnel.I2PTunnel.
func (t *TCPServer) Options() map[string]string {
	panic("unimplemented")
}

// Start implements i2ptunnel.I2PTunnel.
func (t *TCPServer) Start() error {
	panic("unimplemented")
}

// Status implements i2ptunnel.I2PTunnel.
func (t *TCPServer) Status() i2ptunnel.I2PTunnelStatus {
	panic("unimplemented")
}

// Stop implements i2ptunnel.I2PTunnel.
func (t *TCPServer) Stop() error {
	panic("unimplemented")
}

// Target implements i2ptunnel.I2PTunnel.
func (t *TCPServer) Target() string {
	panic("unimplemented")
}

// Type implements i2ptunnel.I2PTunnel.
func (t *TCPServer) Type() string {
	panic("unimplemented")
}
