package httpclient

/**
HTTP Client
-----------

The HTTP Client implements a proxy server that enables HTTP/S traffic between local applications and I2P network services. It acts as an intermediary, handling all standard HTTP methods and CONNECT requests.

```
[Browser/App] <-> [I2P HTTP Client] <-> [I2P Network] <-> [I2P Services]
    :8118           (HTTP Proxy)         Encrypted        Web Servers
                    |
               - Protocol handling
               - Header management
               - Connection routing
```

Key features:
- Supports HTTP, HTTPS and CONNECT methods
- Proxies requests between local clients and I2P services
- Manages HTTP headers and connection states
- Handles protocol negotiation and routing
**/

import (
	"strconv"

	httpinspector "github.com/go-i2p/go-connfilter/http"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/onramp"
)

var implementHTTPClient i2ptunnel.I2PTunnel = &HTTPClient{}

type HTTPClient struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// The http filtering configuration
	httpinspector.Config
	// Channel for shutdown signaling
	done chan struct{}

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}

func (h *HTTPClient) recordError(err error) {
	h.Errors = append(h.Errors, i2ptunnel.NewError(h, err))
}

// Get the tunnel's I2P address
func (h *HTTPClient) Address() string {
	return h.Garlic.B32()
}

// Get the tunnel's error message
func (h *HTTPClient) Error() error {
	if len(h.Errors) > 0 {
		return h.Errors[len(h.Errors)-1]
	}
	return nil
}

// Get the tunnel's local host:port
func (h *HTTPClient) LocalAddress() (string, string, error) {
	return h.TunnelConfig.Interface, strconv.Itoa(h.TunnelConfig.Port), nil
}

// Get the tunnel's name
func (h *HTTPClient) Name() string {
	return h.TunnelConfig.Name
}

// Get the tunnel's options
func (h *HTTPClient) Options() map[string]string {
	return h.TunnelConfig.Options()
}

// Start the tunnel
func (h *HTTPClient) Start() error {
	panic("unimplemented")
}

// Get the tunnel's status
func (h *HTTPClient) Status() i2ptunnel.I2PTunnelStatus {
	return h.I2PTunnelStatus
}

// Stop the tunnel
func (h *HTTPClient) Stop() error {
	panic("unimplemented")
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (h *HTTPClient) Target() string {
	return ""
}

// Get the tunnel's type
func (h *HTTPClient) Type() string {
	return h.TunnelConfig.Type
}
