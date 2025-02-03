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

import (
	"context"
	"net"
	"strconv"

	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/stream"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/i2pkeys"
	"github.com/go-i2p/onramp"
)

var implementTCPClient i2ptunnel.I2PTunnel = &TCPClient{}

type TCPClient struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The remote I2P destination target
	*i2pkeys.I2PAddr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// Channel for shutdown signaling
	done chan struct{}

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}

func (t *TCPClient) recordError(err error) {
	t.Errors = append(t.Errors, i2ptunnel.NewError(t, err))
}

// Get the tunnel's I2P address
func (t *TCPClient) Address() string {
	return t.Garlic.B32()
}

// Get the tunnel's error message
func (t *TCPClient) Error() error {
	if len(t.Errors) > 0 {
		return t.Errors[len(t.Errors)-1]
	}
	return nil
}

// Get the tunnel's local host:port
func (t *TCPClient) LocalAddress() (string, string, error) {
	return t.TunnelConfig.Interface, strconv.Itoa(t.TunnelConfig.Port), nil
}

// Get the tunnel's name
func (t *TCPClient) Name() string {
	return t.TunnelConfig.Name
}

// Get the tunnel's options
func (t *TCPClient) Options() map[string]string {
	return t.TunnelConfig.Options()
}

// Start the tunnel
func (t *TCPClient) Start() error {
	i2pConn, err := t.Garlic.Dial("tcp", t.Target())
	if err != nil {
		return err
	}
	t.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStarting
	defer i2pConn.Close()
	defer t.Stop()
	listener, err := net.Listen("tcp", net.JoinHostPort(t.Interface, strconv.Itoa(t.Port)))
	if err != nil {
		return err
	}
	defer listener.Close()
	t.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning
	for {
		select {
		case <-t.done:
			return nil
		default:
			con, err := listener.Accept()
			if err != nil {
				continue
			}
			defer con.Close()
			ctx := context.Background()
			stream.Forward(ctx, con, i2pConn, config.DefaultConfig())
		}
	}
}

// Get the tunnel's status
func (t *TCPClient) Status() i2ptunnel.I2PTunnelStatus {
	return t.I2PTunnelStatus
}

// Stop the tunnel
func (t *TCPClient) Stop() error {
	close(t.done)
	// Cleanup resources
	t.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped
	return nil
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (t *TCPClient) Target() string {
	return t.I2PAddr.Base32()
}

// Get the tunnel's type
func (t *TCPClient) Type() string {
	return t.TunnelConfig.Type
}
