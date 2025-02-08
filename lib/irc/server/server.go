package ircserver

/**
IRC Server
----------

The IRC Server implements a reverse proxy that enables IRC servers hosted on the local machine to be accessible from the I2P network. It provides:

- Secure traffic forwarding between local IRC services and I2P clients
- Access control and connection management
- Command filtering and security policies
- Bandwidth and resource monitoring
**/

import (
	"context"
	"net"
	"strconv"

	ircinspector "github.com/go-i2p/go-connfilter/irc"
	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/stream"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/onramp"
)

var implementIRCServer i2ptunnel.I2PTunnel = &IRCServer{}

type IRCServer struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The local IRC service address
	net.Addr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// The rate-limiting configuration
	limitedlistener.LimitedConfig
	// The IRC filtering configuration
	ircinspector.Config
	// Channel for shutdown signaling
	done chan struct{}

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}

func (t *IRCServer) recordError(err error) {
	t.Errors = append(t.Errors, i2ptunnel.NewError(t, err))
}

// Get the tunnel's I2P address
func (i *IRCServer) Address() string {
	return i.Garlic.B32()
}

// Get the tunnel's error message
func (i *IRCServer) Error() error {
	if len(i.Errors) > 0 {
		return i.Errors[len(i.Errors)-1]
	}
	return nil
}

// Get the tunnel's local host:port
func (i *IRCServer) LocalAddress() (string, error) {
	addr := net.JoinHostPort(i.TunnelConfig.Interface, strconv.Itoa(i.TunnelConfig.Port))
	return addr, nil
}

// Get the tunnel's name
func (i *IRCServer) Name() string {
	return i.TunnelConfig.Name
}

// Start the tunnel
func (i *IRCServer) Start() error {
	i2pListener, err := i.Garlic.Listen()
	if err != nil {
		return err
	}
	defer i2pListener.Close()
	defer i.Stop()
	i.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning
	limitedI2PListener := limitedlistener.NewLimitedListener(i2pListener, limitedlistener.WithMaxConnections(i.LimitedConfig.MaxConns), limitedlistener.WithRateLimit(i.LimitedConfig.RateLimit))
	ircInspectorListener := ircinspector.New(limitedI2PListener, i.Config)
	for {
		select {
		case <-i.done:
			return nil
		default:
			con, err := ircInspectorListener.Accept()
			if err != nil {
				continue
			}

			defer con.Close()
			lCon, err := net.Dial("tcp", i.Target())
			if err != nil {
				continue
			}
			defer lCon.Close()
			ctx := context.Background()
			stream.Forward(ctx, con, lCon, config.DefaultConfig())
		}
	}
}

// Get the tunnel's status
func (i *IRCServer) Status() i2ptunnel.I2PTunnelStatus {
	return i.I2PTunnelStatus
}

// Stop the tunnel
func (i *IRCServer) Stop() error {
	close(i.done)
	// Cleanup resources
	i.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped
	return nil
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (i *IRCServer) Target() string {
	return i.Addr.String()
}

// Get the tunnel's type
func (i *IRCServer) Type() string {
	return i.TunnelConfig.Type
}
