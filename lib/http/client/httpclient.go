// Package httpclient implements an HTTP proxy that enables local applications
// to access I2P services via standard HTTP and CONNECT methods.
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
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
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
	// Mutex protecting the I2PTunnelStatus field from concurrent read/write access
	statusMu sync.RWMutex
	// Mutex protecting Jump and Outproxy fields from concurrent read/write access.
	// Separate from mu (held for the entire Start→Serve lifetime) to avoid deadlocks
	// when dial handlers read these fields during active request serving.
	fieldsMu sync.RWMutex
	// ErrorTracker provides bounded error history.
	i2ptunnel.ErrorTracker
	// Metrics tracks live operational data for this tunnel.
	// Set by the webui controller after construction. May be nil.
	Metrics *metrics.TunnelMetrics
	// Jump service client for resolving human-readable .i2p hostnames
	Jump *JumpService
	// Outproxy for routing clearnet HTTP requests through I2P
	Outproxy *Outproxy
	// Context for cleanup
	ctx    context.Context
	cancel context.CancelFunc
}

// SetTunnelMetrics injects a live metrics tracker. Implements metrics.MetricsBearer.
func (h *HTTPClient) SetTunnelMetrics(m *metrics.TunnelMetrics) {
	h.Metrics = m
}

func (h *HTTPClient) recordError(err error) {
	h.ErrorTracker.Record(h, err)
	if h.Metrics != nil {
		h.Metrics.RecordError()
	}
}

func (h *HTTPClient) setStatus(s i2ptunnel.I2PTunnelStatus) {
	h.statusMu.Lock()
	h.I2PTunnelStatus = s
	h.statusMu.Unlock()
}

// connectDial handles HTTPS CONNECT method requests.
// Routes I2P addresses directly and clearnet addresses through the outproxy.
// Used as goproxy's ConnectDial callback.
func (h *HTTPClient) connectDial(network, addr string) (net.Conn, error) {
	host, _, _ := net.SplitHostPort(addr)
	if host == "" {
		host = addr
	}
	// Snapshot fields under fieldsMu to prevent races with SetOptions.
	h.fieldsMu.RLock()
	jump := h.Jump
	outproxy := h.Outproxy
	h.fieldsMu.RUnlock()
	var (
		conn net.Conn
		err  error
	)
	if !IsI2PAddress(host) {
		conn, err = dialOutproxySnapshot(outproxy, h.Garlic, network, addr)
	} else {
		addr = resolveJumpSnapshot(jump, addr)
		conn, err = h.Garlic.Dial(network, addr)
	}
	if err != nil {
		if h.Metrics != nil {
			h.Metrics.RecordConnectionFailed()
		}
		return nil, err
	}
	if h.Metrics != nil {
		h.Metrics.RecordConnection()
	}
	return metrics.WrapConn(conn, h.Metrics), nil
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
	return h.ErrorTracker.Last()
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
	// Route HTTPS CONNECT requests through outproxy for clearnet addresses.
	// Without this, only plain HTTP requests would be outproxied.
	h.ProxyHttpServer.ConnectDial = h.connectDial
	// set up local listener
	listener, err := net.Listen("tcp", net.JoinHostPort(h.Interface, strconv.Itoa(h.Port)))
	if err != nil {
		return err
	}
	// set up httpinspector listener
	listenerInspector := httpinspector.New(listener, h.Config)
	h.Server = &http.Server{}
	h.Server.Handler = h.ProxyHttpServer
	h.setStatus(i2ptunnel.I2PTunnelStatusRunning)
	if h.Metrics != nil {
		h.Metrics.RecordStart()
	}
	return h.Server.Serve(listenerInspector)
}

// Get the tunnel's status
func (h *HTTPClient) Status() i2ptunnel.I2PTunnelStatus {
	h.statusMu.RLock()
	defer h.statusMu.RUnlock()
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
		h.setStatus(i2ptunnel.I2PTunnelStatusStopping)
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
		h.setStatus(i2ptunnel.I2PTunnelStatusStopped)
		if h.Metrics != nil {
			h.Metrics.RecordStop()
		}
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
	options := i2ptunnel.BuildCommonOptions(h.TunnelConfig)
	h.fieldsMu.RLock()
	if h.Jump != nil {
		options["jumpservice"] = h.Jump.URL()
	}
	if h.Outproxy != nil && h.Outproxy.Address != "" {
		options["outproxy"] = h.Outproxy.Address
		if h.Outproxy.Enabled {
			options["outproxy.enabled"] = "true"
		} else {
			options["outproxy.enabled"] = "false"
		}
	}
	h.fieldsMu.RUnlock()
	return options
}

// Set the tunnel's options
func (h *HTTPClient) SetOptions(opts map[string]string) error {
	if err := i2ptunnel.ApplyCommonOptions(opts, &h.TunnelConfig); err != nil {
		return err
	}
	h.fieldsMu.Lock()
	defer h.fieldsMu.Unlock()
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
	// Configure outproxy for clearnet HTTP access via I2P
	if outAddr, ok := opts["outproxy"]; ok {
		if h.Outproxy == nil {
			h.Outproxy = &Outproxy{}
		}
		if outAddr == "" {
			// Disable outproxy
			h.Outproxy.Address = ""
			h.Outproxy.Enabled = false
		} else {
			if !IsI2PAddress(outAddr) {
				return fmt.Errorf("outproxy address must be an I2P address (.i2p), got %q", outAddr)
			}
			h.Outproxy.Address = outAddr
			h.Outproxy.Enabled = true
		}
	}
	if enabledStr, ok := opts["outproxy.enabled"]; ok {
		if h.Outproxy == nil {
			h.Outproxy = &Outproxy{}
		}
		h.Outproxy.Enabled = enabledStr == "true" || enabledStr == "1" || enabledStr == "yes"
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
	status := h.Status()
	if status == i2ptunnel.I2PTunnelStatusRunning ||
		status == i2ptunnel.I2PTunnelStatusStarting {
		return fmt.Errorf("cannot load config while tunnel is %s - stop tunnel first", status)
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
