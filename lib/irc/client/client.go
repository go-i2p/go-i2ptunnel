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

import (
	"strconv"

	ircinspector "github.com/go-i2p/go-connfilter/irc"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/i2pkeys"
	"github.com/go-i2p/onramp"
)

var implementIRCClient i2ptunnel.I2PTunnel = &IRCClient{}

type IRCClient struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The remote I2P destination target
	*i2pkeys.I2PAddr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// The IRC filtering configuration
	ircinspector.Config
	// Channel for shutdown signaling
	done chan struct{}

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}

func (t *IRCClient) recordError(err error) {
	t.Errors = append(t.Errors, i2ptunnel.NewError(t, err))
}

// Get the tunnel's I2P address
func (i *IRCClient) Address() string {
	return i.Garlic.B32()
}

// Get the tunnel's error message
func (i *IRCClient) Error() error {
	if len(i.Errors) > 0 {
		return i.Errors[len(i.Errors)-1]
	}
	return nil
}

// Get the tunnel's local host:port
func (i *IRCClient) LocalAddress() (string, string, error) {
	return i.TunnelConfig.Interface, strconv.Itoa(i.TunnelConfig.Port), nil
}

// Get the tunnel's name
func (i *IRCClient) Name() string {
	return i.TunnelConfig.Name
}

// Get the tunnel's options
func (i *IRCClient) Options() map[string]string {
	return i.TunnelConfig.Options()
}

// Start the tunnel
func (i *IRCClient) Start() error {
	panic("unimplemented")
}

// Get the tunnel's status
func (i *IRCClient) Status() i2ptunnel.I2PTunnelStatus {
	return i.I2PTunnelStatus
}

// Stop the tunnel
func (i *IRCClient) Stop() error {
	panic("unimplemented")
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (i *IRCClient) Target() string {
	return i.I2PAddr.Base32()
}

// Get the tunnel's type
func (i *IRCClient) Type() string {
	return i.TunnelConfig.Type
}
