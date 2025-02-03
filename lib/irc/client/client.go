package ircclient

/**
IRC Client
----------

The IRC Client implements a SOCKS-compatible proxy that enables local IRC clients to connect to services on the I2P network. It provides:

- Transparent proxying between local IRC clients and I2P servers
- Command filtering for enhanced security
- Connection management and automatic reconnection
- Bandwidth usage monitoring
**/

import i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"

var implementIRCClient i2ptunnel.I2PTunnel = &IRCClient{}

type IRCClient struct{}

// Get the tunnel's I2P address
func (i *IRCClient) Address() string {
	panic("unimplemented")
}

// Get the tunnel's error message
func (i *IRCClient) Error() error {
	panic("unimplemented")
}

// Get the tunnel's local host:port
func (i *IRCClient) LocalAddress() (string, string, error) {
	panic("unimplemented")
}

// Get the tunnel's name
func (i *IRCClient) Name() string {
	panic("unimplemented")
}

// Get the tunnel's options
func (i *IRCClient) Options() map[string]string {
	panic("unimplemented")
}

// Start the tunnel
func (i *IRCClient) Start() error {
	panic("unimplemented")
}

// Get the tunnel's status
func (i *IRCClient) Status() i2ptunnel.I2PTunnelStatus {
	panic("unimplemented")
}

// Stop the tunnel
func (i *IRCClient) Stop() error {
	panic("unimplemented")
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (i *IRCClient) Target() string {
	panic("unimplemented")
}

// Get the tunnel's type
func (i *IRCClient) Type() string {
	panic("unimplemented")
}
