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

import (
	"net"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/onramp"
	// github.com/go-i2p/go-forward/stream
)

var implementTCPServer i2ptunnel.I2PTunnel = &TCPServer{}

type TCPServer struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// TCP Connection to the local service
	net.Conn
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The local TCP service address
	net.Addr
}

// Address implements i2ptunnel.I2PTunnel.
func (t *TCPServer) Address() string {
	return t.Garlic.B32()
}

// Error implements i2ptunnel.I2PTunnel.
func (t *TCPServer) Error() error {
	panic("unimplemented")
}

// LocalAddress implements i2ptunnel.I2PTunnel.
func (t *TCPServer) LocalAddress() (string, string, error) {
	addr := t.Conn.LocalAddr().String()
	return net.SplitHostPort(addr)
}

// Name implements i2ptunnel.I2PTunnel.
func (t *TCPServer) Name() string {
	return t.TunnelConfig.Name
}

// Options implements i2ptunnel.I2PTunnel.
func (t *TCPServer) Options() map[string]string {
	return t.Options()
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
	return t.Addr.String()
}

// Type implements i2ptunnel.I2PTunnel.
func (t *TCPServer) Type() string {
	return t.TunnelConfig.Type
}
