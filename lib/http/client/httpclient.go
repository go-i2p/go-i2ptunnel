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
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"

	httpinspector "github.com/go-i2p/go-connfilter/http"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/onramp"

	"github.com/elazarl/goproxy"
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
	// The proxy server
	*goproxy.ProxyHttpServer
	// The http server
	*http.Server
	// Channel for shutdown signaling
	done chan struct{}
	// Mutex for server operations
	mu sync.Mutex
	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
	// Context for cleanup
	ctx    context.Context
	cancel context.CancelFunc
}

func (h *HTTPClient) recordError(err error) {
	h.Errors = append(h.Errors, i2ptunnel.NewError(h, err))
}

// Get the tunnel's I2P address
func (h *HTTPClient) Address() string {
	// For HTTP client proxy, return the service address if available
	if h.Garlic != nil && h.Garlic.ServiceKeys != nil {
		return h.Garlic.ServiceKeys.Addr().Base32()
	}
	return ""
}

// Get the tunnel's error message
func (h *HTTPClient) Error() error {
	if len(h.Errors) > 0 {
		return h.Errors[len(h.Errors)-1]
	}
	return nil
}

// Get the tunnel's local host:port
func (h *HTTPClient) LocalAddress() (string, error) {
	addr := net.JoinHostPort(h.TunnelConfig.Interface, strconv.Itoa(h.TunnelConfig.Port))
	return addr, nil
}

// Get the tunnel's name
func (h *HTTPClient) Name() string {
	return h.TunnelConfig.Name
}

// Start the tunnel
func (h *HTTPClient) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.ProxyHttpServer != nil {
		return nil // Already started
	}

	// Create context for managing goroutines
	h.ctx, h.cancel = context.WithCancel(context.Background())
	h.done = make(chan struct{})
	proxy := goproxy.NewProxyHttpServer()
	h.ProxyHttpServer = proxy
	h.ProxyHttpServer.Tr.DialContext = h.DialContext
	// set up local listener
	listener, err := net.Listen("tcp", net.JoinHostPort(h.Interface, strconv.Itoa(h.Port)))
	if err != nil {
		return err
	}
	// set up httpinspector listener
	listenerInspector := httpinspector.New(listener, h.Config)
	h.Server = &http.Server{}
	h.Server.Handler = h.ProxyHttpServer
	return h.Server.Serve(listenerInspector)
}

// Get the tunnel's status
func (h *HTTPClient) Status() i2ptunnel.I2PTunnelStatus {
	return h.I2PTunnelStatus
}

// Stop the tunnel
func (h *HTTPClient) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.Server != nil {
		h.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopping
		close(h.done)
		if err := h.Server.Shutdown(h.ctx); err != nil {
			h.recordError(err)
			return err
		}
		h.cancel()
		h.Server = nil
		h.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped
	}
	return nil
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (h *HTTPClient) Target() string {
	return ""
}

// Get the tunnel's type
func (h *HTTPClient) Type() string {
	return h.TunnelConfig.Type
}

// Get the tunnel's ID
func (h *HTTPClient) ID() string {
	return i2ptunnel.Clean(h.Name())
}

// Get the tunnel's options
func (h *HTTPClient) Options() map[string]string {
	// Return basic configuration options as a map
	options := make(map[string]string)
	options["name"] = h.TunnelConfig.Name
	options["type"] = h.TunnelConfig.Type
	options["interface"] = h.TunnelConfig.Interface
	options["port"] = strconv.Itoa(h.TunnelConfig.Port)
	return options
}

// Set the tunnel's options
func (h *HTTPClient) SetOptions(opts map[string]string) error {
	// Apply configuration options from the map
	if name, ok := opts["name"]; ok {
		h.TunnelConfig.Name = name
	}
	if iface, ok := opts["interface"]; ok {
		h.TunnelConfig.Interface = iface
	}
	if portStr, ok := opts["port"]; ok {
		if port, err := strconv.Atoi(portStr); err == nil {
			h.TunnelConfig.Port = port
		} else {
			return fmt.Errorf("invalid port value: %s", portStr)
		}
	}
	return nil
}

// Load the tunnel config from file
func (h *HTTPClient) LoadConfig(path string) error {
	// For now, return an error indicating this method needs configuration file support
	return fmt.Errorf("LoadConfig not yet implemented: would load configuration from %s", path)
}
