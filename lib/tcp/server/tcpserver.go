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
	"context"
	"net"

	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/stream"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/onramp"
)

var implementTCPServer i2ptunnel.I2PTunnel = &TCPServer{}

type TCPServer struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The local TCP service address
	net.Addr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// The rate-limiting configuration
	limitedlistener.LimitedConfig
	// Channel for shutdown signaling
	done chan struct{}

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}

func (t *TCPServer) recordError(err error) {
	t.Errors = append(t.Errors, i2ptunnel.NewError(t, err))
}

// Get the tunnel's I2P address
func (t *TCPServer) Address() string {
	return t.Garlic.B32()
}

// Get the tunnel's error message
func (t *TCPServer) Error() error {
	if len(t.Errors) > 0 {
		return t.Errors[len(t.Errors)-1]
	}
	return nil
}

// Get the tunnel's local host:port
func (t *TCPServer) LocalAddress() (string, string, error) {
	addr := t.Addr.String()
	return net.SplitHostPort(addr)
}

// Get the tunnel's name
func (t *TCPServer) Name() string {
	return t.TunnelConfig.Name
}

// Get the tunnel's options
func (t *TCPServer) Options() map[string]string {
	return t.Options()
}

// Start the tunnel
func (t *TCPServer) Start() error {
	i2pListener, err := t.Garlic.Listen()
	if err != nil {
		return err
	}
	defer i2pListener.Close()
	defer t.Stop()
	t.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning
	limitedI2PListener := limitedlistener.NewLimitedListener(i2pListener, limitedlistener.WithMaxConnections(t.LimitedConfig.MaxConns), limitedlistener.WithRateLimit(t.LimitedConfig.RateLimit))
	for {
		select {
		case <-t.done:
			return nil
		default:
			con, err := limitedI2PListener.Accept()
			if err != nil {
				continue
			}

			defer con.Close()
			lCon, err := net.Dial("tcp", t.Target())
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
func (t *TCPServer) Status() i2ptunnel.I2PTunnelStatus {
	return t.I2PTunnelStatus
}

// Stop the tunnel
func (t *TCPServer) Stop() error {
	close(t.done)
	// Cleanup resources
	t.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped
	return nil
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (t *TCPServer) Target() string {
	return t.Addr.String()
}

// Get the tunnel's type
func (t *TCPServer) Type() string {
	return t.TunnelConfig.Type
}
