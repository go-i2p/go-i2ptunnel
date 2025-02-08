package httpserver

/**
HTTP Server
-----------

The HTTP Server implements a reverse proxy that forwards traffic between local services and I2P clients. It acts as an intermediary, providing access control and traffic management.

```
[Local Service] <-> [I2P HTTP Server] <-> [I2P Network] <-> [I2P Clients]
    :8080            (Reverse Proxy)        Encrypted        Browser/App
                     |
                - Header filtering
                - Rate limiting
                - Access control
```

Key features:
- Forwards requests between local services and I2P network
- Filters and modifies HTTP headers
- Rate limits incoming requests
- Provides access control for I2P clients
**/

import (
	"context"
	"net"
	"strconv"

	httpinspector "github.com/go-i2p/go-connfilter/http"
	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/stream"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/onramp"
)

var implementHTTPServer i2ptunnel.I2PTunnel = &HTTPServer{}

type HTTPServer struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The local HTTP service address
	net.Addr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// The rate-limiting configuration
	limitedlistener.LimitedConfig
	// The http filtering configuration
	httpinspector.Config
	// Channel for shutdown signaling
	done chan struct{}

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}

func (h *HTTPServer) recordError(err error) {
	h.Errors = append(h.Errors, i2ptunnel.NewError(h, err))
}

// Get the tunnel's I2P address
func (h *HTTPServer) Address() string {
	return h.Garlic.B32()
}

// Get the tunnel's error message
func (h *HTTPServer) Error() error {
	if len(h.Errors) > 0 {
		return h.Errors[len(h.Errors)-1]
	}
	return nil
}

// Get the tunnel's local host:port
func (h *HTTPServer) LocalAddress() (string, error) {
	addr := net.JoinHostPort(h.TunnelConfig.Interface, strconv.Itoa(h.TunnelConfig.Port))
	return addr, nil
}

// Get the tunnel's name
func (h *HTTPServer) Name() string {
	return h.TunnelConfig.Name
}

// Start the tunnel
func (h *HTTPServer) Start() error {
	i2pListener, err := h.Garlic.Listen()
	if err != nil {
		return err
	}
	defer i2pListener.Close()
	defer h.Stop()
	h.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning
	limitedI2PListener := limitedlistener.NewLimitedListener(i2pListener, limitedlistener.WithMaxConnections(h.LimitedConfig.MaxConns), limitedlistener.WithRateLimit(h.LimitedConfig.RateLimit))
	httpInspectorListener := httpinspector.New(limitedI2PListener, h.Config)
	for {
		select {
		case <-h.done:
			return nil
		default:
			con, err := httpInspectorListener.Accept()
			if err != nil {
				continue
			}

			defer con.Close()
			lCon, err := net.Dial("tcp", h.Target())
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
func (h *HTTPServer) Status() i2ptunnel.I2PTunnelStatus {
	return h.I2PTunnelStatus
}

// Stop the tunnel
func (h *HTTPServer) Stop() error {
	close(h.done)
	// Cleanup resources
	h.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped
	return nil
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (h *HTTPServer) Target() string {
	return h.Addr.String()
}

// Get the tunnel's type
func (h *HTTPServer) Type() string {
	return h.TunnelConfig.Type
}
