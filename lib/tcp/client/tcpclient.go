package tcpclient

/**
TCP Client Tunnel
-----------------

A TCP Client tunnel operates by:
1. Running a TCP Server that listens on a local port
2. Maintaining an I2P Client connected to a specific destination

When activated:
- Local applications connect to the TCP Server
- Traffic routes through the I2P Client to the target I2P destination
- Creates a secure point-to-point connection

Both tunnel types preserve the original TCP traffic while adding I2P's anonymity and encryption layers.

When a local client connects to the I2P tunnel's destination, the traffic flows:
- Outgoing: Local Client → TCP Server → I2P Client → I2P Network
- Incoming: I2P Network → I2P Client → TCP Server → Local Client
**/

import i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"

var implementTCPClient i2ptunnel.I2PTunnel = &TCPClient{}

type TCPClient struct {
}

// Address implements i2ptunnel.I2PTunnel.
func (t *TCPClient) Address() string {
	panic("unimplemented")
}

// Error implements i2ptunnel.I2PTunnel.
func (t *TCPClient) Error() error {
	panic("unimplemented")
}

// LocalAddress implements i2ptunnel.I2PTunnel.
func (t *TCPClient) LocalAddress() (string, string, error) {
	panic("unimplemented")
}

// Name implements i2ptunnel.I2PTunnel.
func (t *TCPClient) Name() string {
	panic("unimplemented")
}

// Options implements i2ptunnel.I2PTunnel.
func (t *TCPClient) Options() map[string]string {
	panic("unimplemented")
}

// Start implements i2ptunnel.I2PTunnel.
func (t *TCPClient) Start() error {
	panic("unimplemented")
}

// Status implements i2ptunnel.I2PTunnel.
func (t *TCPClient) Status() i2ptunnel.I2PTunnelStatus {
	panic("unimplemented")
}

// Stop implements i2ptunnel.I2PTunnel.
func (t *TCPClient) Stop() error {
	panic("unimplemented")
}

// Target implements i2ptunnel.I2PTunnel.
func (t *TCPClient) Target() string {
	panic("unimplemented")
}

// Type implements i2ptunnel.I2PTunnel.
func (t *TCPClient) Type() string {
	panic("unimplemented")
}
