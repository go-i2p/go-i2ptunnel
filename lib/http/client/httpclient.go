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
	"os"
	"strconv"
	"sync"
	"time"

	httpinspector "github.com/go-i2p/go-connfilter/http"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
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
	// Ensures the done channel is only closed once to prevent panic
	stopOnce sync.Once
	// Mutex for server operations
	mu sync.Mutex
	// Mutex protecting the Errors slice from concurrent access
	errMu sync.Mutex
	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
	// Jump service client for resolving human-readable .i2p hostnames
	Jump *JumpService
	// Context for cleanup
	ctx    context.Context
	cancel context.CancelFunc
}

func (h *HTTPClient) recordError(err error) {
	h.errMu.Lock()
	h.Errors = append(h.Errors, i2ptunnel.NewError(h, err))
	h.errMu.Unlock()
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
	h.errMu.Lock()
	defer h.errMu.Unlock()
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
	h.stopOnce = sync.Once{}
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
	h.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning
	return h.Server.Serve(listenerInspector)
}

// Get the tunnel's status
func (h *HTTPClient) Status() i2ptunnel.I2PTunnelStatus {
	return h.I2PTunnelStatus
}

// shutdownTimeout is the maximum time to wait for graceful HTTP server shutdown.
// After this duration, Shutdown returns context.DeadlineExceeded and in-flight
// connections are abandoned. 30 seconds is generous for most HTTP workloads.
const shutdownTimeout = 30 * time.Second

// Stop the tunnel. Safe to call multiple times.
// Uses a bounded timeout context to prevent indefinite blocking on lingering connections.
// Closes the Garlic (I2P SAM session) to release network resources.
func (h *HTTPClient) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.Server != nil {
		h.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopping
		h.stopOnce.Do(func() {
			close(h.done)
		})
		// Always use a timeout-bounded context for shutdown.
		// Previously this used h.ctx (which could be nil if Start() never ran)
		// or fell back to context.Background() (which has no deadline).
		// Either path could block indefinitely with lingering connections.
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()
		if err := h.Server.Shutdown(shutdownCtx); err != nil {
			h.recordError(err)
			return err
		}
		if h.Garlic != nil {
			h.Garlic.Close()
		}
		if h.cancel != nil {
			h.cancel()
		}
		h.Server = nil
		h.ProxyHttpServer = nil
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
	if h.Jump != nil {
		options["jumpservice"] = h.Jump.URL()
	}
	i2ptunnel.MergeI2CPOptions(h.TunnelConfig.I2CP, options)
	return options
}

// Set the tunnel's options
func (h *HTTPClient) SetOptions(opts map[string]string) error {
	// Apply configuration options from the map with validation
	if name, ok := opts["name"]; ok {
		if err := validate.RequiredString("name", name); err != nil {
			return err
		}
		h.TunnelConfig.Name = name
	}
	if iface, ok := opts["interface"]; ok {
		if err := validate.Interface(iface); err != nil {
			return err
		}
		h.TunnelConfig.Interface = iface
	}
	if portStr, ok := opts["port"]; ok {
		port, err := validate.PortString(portStr)
		if err != nil {
			return err
		}
		h.TunnelConfig.Port = port
	}
	// Configure jump service URL for human-readable .i2p hostname resolution
	if jumpURL, ok := opts["jumpservice"]; ok {
		if jumpURL == "" {
			// Disable jump service
			h.Jump = nil
		} else {
			if h.Jump != nil {
				// Update existing jump service URL by recreating it
				h.Jump = NewJumpService(h.Jump.client, jumpURL)
			} else {
				h.Jump = NewJumpService(nil, jumpURL)
			}
		}
	}
	// Apply I2CP options (encrypted LeaseSet, authentication, etc.)
	if i2cpOpts := i2ptunnel.ExtractI2CPOptions(opts); i2cpOpts != nil {
		if h.TunnelConfig.I2CP == nil {
			h.TunnelConfig.I2CP = make(map[string]interface{})
		}
		for k, v := range i2cpOpts {
			h.TunnelConfig.I2CP[k] = v
		}
	}
	return nil
}

// LoadConfig loads tunnel configuration from a file and updates the tunnel settings.
// The tunnel must be stopped before calling LoadConfig to prevent inconsistent state.
// Supported formats: .properties, .ini, .yaml/.yml
//
// Why: HTTP proxies need dynamic configuration updates for production deployments.
// Design: Uses go-i2ptunnel-config library for parsing. Preserves SAM connection and I2P keys.
func (h *HTTPClient) LoadConfig(path string) error {
	// Prevent config changes while tunnel is running to avoid race conditions
	if h.I2PTunnelStatus == i2ptunnel.I2PTunnelStatusRunning ||
		h.I2PTunnelStatus == i2ptunnel.I2PTunnelStatusStarting {
		return fmt.Errorf("cannot load config while tunnel is %s - stop tunnel first", h.I2PTunnelStatus)
	}

	// Parse config file using the converter library
	conv := i2pconv.Converter{}
	format, err := conv.DetectFormat(path)
	if err != nil {
		return fmt.Errorf("failed to detect config format: %w", err)
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	newConfig, err := conv.ParseInput(bytes, format)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Type safety: ensure loaded config matches expected tunnel type
	if newConfig.Type != "httpclient" {
		return fmt.Errorf("config file contains %s tunnel, expected httpclient", newConfig.Type)
	}

	// Update mutable configuration fields
	// The Garlic connection (SAM) is preserved to maintain tunnel identity and keys
	h.TunnelConfig = *newConfig

	return nil
}
