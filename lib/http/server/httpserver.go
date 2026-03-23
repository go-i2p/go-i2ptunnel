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
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	httpinspector "github.com/go-i2p/go-connfilter/http"
	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/stream"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
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
	// Ensures Stop() is only executed once to prevent double-close panic
	stopOnce sync.Once
	// Listener reference for clean shutdown — closing unblocks Accept()
	listener net.Listener
	// Mutex protecting lifecycle fields (done, stopOnce, listener) during Start/Stop transitions.
	// Prevents the race where Start() resets stopOnce while Stop() is calling stopOnce.Do().
	lifeMu sync.Mutex
	// Mutex protecting the I2PTunnelStatus field from concurrent read/write access
	statusMu sync.RWMutex
	// ErrorTracker provides bounded error history.
	i2ptunnel.ErrorTracker
	// Metrics tracks live operational data for this tunnel.
	// Set by the webui controller after construction. May be nil.
	Metrics *metrics.TunnelMetrics
}

// SetTunnelMetrics injects a live metrics tracker. Implements metrics.MetricsBearer.
func (h *HTTPServer) SetTunnelMetrics(m *metrics.TunnelMetrics) {
	h.Metrics = m
}

func (h *HTTPServer) recordError(err error) {
	h.ErrorTracker.Record(h, err)
	if h.Metrics != nil {
		h.Metrics.RecordError()
	}
}

func (h *HTTPServer) setStatus(s i2ptunnel.I2PTunnelStatus) {
	h.statusMu.Lock()
	h.I2PTunnelStatus = s
	h.statusMu.Unlock()
}

// Get the tunnel's I2P address
func (h *HTTPServer) Address() string {
	// For HTTP server, return the service address if available
	if h.Garlic != nil && h.Garlic.ServiceKeys != nil {
		return h.Garlic.ServiceKeys.Addr().Base32()
	}
	return ""
}

// Get the tunnel's error message
func (h *HTTPServer) Error() error {
	return h.ErrorTracker.Last()
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

// Start the tunnel.
// Each incoming I2P connection is forwarded to the local HTTP service in a separate goroutine.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (h *HTTPServer) Start() error {
	h.lifeMu.Lock()
	h.done = make(chan struct{})
	h.stopOnce = sync.Once{}
	i2pListener, err := h.Garlic.ListenStream()
	if err != nil {
		h.lifeMu.Unlock()
		return err
	}
	h.listener = i2pListener
	h.lifeMu.Unlock()
	defer i2pListener.Close()
	defer h.Stop()
	h.setStatus(i2ptunnel.I2PTunnelStatusRunning)
	if h.Metrics != nil {
		h.Metrics.RecordStart()
	}
	limitedI2PListener := limitedlistener.NewLimitedListener(i2pListener, limitedlistener.WithMaxConnections(h.LimitedConfig.MaxConns), limitedlistener.WithRateLimit(h.LimitedConfig.RateLimit))
	httpInspectorListener := httpinspector.New(limitedI2PListener, h.Config)
	for {
		select {
		case <-h.done:
			return nil
		default:
			con, err := httpInspectorListener.Accept()
			if err != nil {
				select {
				case <-h.done:
					return nil
				default:
				}
				time.Sleep(50 * time.Millisecond)
				continue
			}
			go h.handleConnection(con)
		}
	}
}

// handleConnection forwards a single I2P connection to the local HTTP service.
// Both connections are closed when forwarding completes.
func (h *HTTPServer) handleConnection(con net.Conn) {
	if h.Metrics != nil {
		h.Metrics.RecordConnection()
	}
	wrapped := metrics.WrapConn(con, h.Metrics)
	defer wrapped.Close()
	lCon, err := net.Dial("tcp", h.Target())
	if err != nil {
		h.recordError(err)
		if h.Metrics != nil {
			h.Metrics.RecordConnectionFailed()
		}
		return
	}
	defer lCon.Close()
	ctx := context.Background()
	stream.Forward(ctx, wrapped, lCon, config.DefaultConfig())
}

// Get the tunnel's status
func (h *HTTPServer) Status() i2ptunnel.I2PTunnelStatus {
	h.statusMu.RLock()
	defer h.statusMu.RUnlock()
	return h.I2PTunnelStatus
}

// Stop the tunnel. Safe to call multiple times.
// Closes the Garlic (I2P SAM session) to release network resources.
func (h *HTTPServer) Stop() error {
	h.lifeMu.Lock()
	defer h.lifeMu.Unlock()
	h.stopOnce.Do(func() {
		close(h.done)
		if h.listener != nil {
			h.listener.Close()
		}
		if h.Garlic != nil {
			h.Garlic.Close()
		}
		h.setStatus(i2ptunnel.I2PTunnelStatusStopped)
		if h.Metrics != nil {
			h.Metrics.RecordStop()
		}
	})
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

// Get the tunnel's ID
func (h *HTTPServer) ID() string {
	return i2ptunnel.Clean(h.Name())
}

// Get the tunnel's options
func (h *HTTPServer) Options() map[string]string {
	// Return basic configuration options as a map
	options := make(map[string]string)
	options["name"] = h.TunnelConfig.Name
	options["type"] = h.TunnelConfig.Type
	options["interface"] = h.TunnelConfig.Interface
	options["port"] = strconv.Itoa(h.TunnelConfig.Port)
	options["maxconns"] = strconv.Itoa(h.LimitedConfig.MaxConns)
	options["ratelimit"] = strconv.FormatFloat(h.LimitedConfig.RateLimit, 'f', -1, 64)
	if h.Addr != nil {
		options["target"] = h.Addr.String()
	}
	i2ptunnel.MergeI2CPOptions(h.TunnelConfig.I2CP, options)
	return options
}

// Set the tunnel's options
func (h *HTTPServer) SetOptions(opts map[string]string) error {
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
	if maxconnsStr, ok := opts["maxconns"]; ok {
		maxconns, err := strconv.Atoi(maxconnsStr)
		if err != nil {
			return fmt.Errorf("invalid maxconns value: %s", maxconnsStr)
		}
		if err := validate.MaxConnections(maxconns); err != nil {
			return err
		}
		h.LimitedConfig.MaxConns = maxconns
	}
	if ratelimitStr, ok := opts["ratelimit"]; ok {
		ratelimit, err := validate.RateLimitString(ratelimitStr)
		if err != nil {
			return err
		}
		h.LimitedConfig.RateLimit = ratelimit
	}
	if target, ok := opts["target"]; ok {
		if err := validate.NetworkAddress(target); err != nil {
			return err
		}
		addr, err := net.ResolveTCPAddr("tcp", target)
		if err != nil {
			return fmt.Errorf("invalid target address %q: %w", target, err)
		}
		h.Addr = addr
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
// Why: HTTP reverse proxies need dynamic configuration updates for production deployments.
// Design: Uses go-i2ptunnel-config library for parsing. Preserves SAM connection and I2P keys.
func (h *HTTPServer) LoadConfig(path string) error {
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
	if newConfig.Type != "httpserver" {
		return fmt.Errorf("config file contains %s tunnel, expected httpserver", newConfig.Type)
	}

	// Validate target address (local service) before applying changes
	targetAddr, err := net.ResolveTCPAddr("tcp", newConfig.Target)
	if err != nil {
		return fmt.Errorf("invalid target address in config: %w", err)
	}

	// Update mutable configuration fields
	h.TunnelConfig = *newConfig
	h.Addr = targetAddr

	return nil
}
