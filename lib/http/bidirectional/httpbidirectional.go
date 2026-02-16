package httpbidirectional

// HTTP Bidirectional Tunnel
//
// An HTTP bidirectional tunnel combines:
// 1. An HTTP server tunnel (forwarding incoming I2P connections to a local HTTP service)
// 2. An HTTP proxy (allowing local apps to reach arbitrary I2P destinations via HTTP)
//
// Both sides share the same I2P identity (keys), meaning the tunnel's I2P address
// is reachable for inbound HTTP connections while also providing an HTTP proxy
// for outbound I2P access.

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
	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/stream"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/onramp"

	"github.com/elazarl/goproxy"
)

var implementHTTPBidirectional i2ptunnel.I2PTunnel = &HTTPBidirectional{}

// HTTPBidirectional combines an HTTP server tunnel with an HTTP proxy client
// on the same I2P keys, enabling both inbound and outbound I2P connections.
type HTTPBidirectional struct {
	// I2P connection (shared for both server and client sides)
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The local HTTP service address (forward target for inbound I2P connections)
	net.Addr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// The rate-limiting configuration for the server side
	limitedlistener.LimitedConfig
	// Server-side HTTP filtering for inbound I2P connections
	ServerConfig httpinspector.Config
	// Client-side HTTP filtering for the outbound HTTP proxy
	ClientConfig httpinspector.Config
	// HTTP proxy server for outbound I2P connections
	proxyServer *goproxy.ProxyHttpServer
	// HTTP server wrapping the proxy
	httpServer *http.Server
	// Channel for shutdown signaling
	done chan struct{}
	// Ensures Stop() is only executed once to prevent double-close panic
	stopOnce sync.Once
	// Listener reference for clean shutdown — closing unblocks Accept()
	listener net.Listener
	// Mutex for server operations
	mu sync.Mutex
	// Mutex protecting the Errors slice from concurrent access
	errMu sync.Mutex
	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

func (h *HTTPBidirectional) recordError(err error) {
	h.errMu.Lock()
	h.Errors = append(h.Errors, i2ptunnel.NewError(h, err))
	h.errMu.Unlock()
}

// Address returns the tunnel's I2P address.
func (h *HTTPBidirectional) Address() string {
	if h.Garlic != nil && h.Garlic.ServiceKeys != nil {
		return h.Garlic.ServiceKeys.Addr().Base32()
	}
	return ""
}

// Error returns the most recent error, or nil.
func (h *HTTPBidirectional) Error() error {
	h.errMu.Lock()
	defer h.errMu.Unlock()
	if len(h.Errors) > 0 {
		return h.Errors[len(h.Errors)-1]
	}
	return nil
}

// LocalAddress returns the HTTP proxy listen address.
func (h *HTTPBidirectional) LocalAddress() (string, error) {
	addr := net.JoinHostPort(h.TunnelConfig.Interface, strconv.Itoa(h.TunnelConfig.Port))
	return addr, nil
}

// Name returns the tunnel's configured name.
func (h *HTTPBidirectional) Name() string {
	return h.TunnelConfig.Name
}

// Start launches both the server-side I2P listener (forwarding to the local
// HTTP service) and the client-side HTTP proxy concurrently.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (h *HTTPBidirectional) Start() error {
	h.done = make(chan struct{})
	h.stopOnce = sync.Once{}
	h.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStarting
	h.ctx, h.cancel = context.WithCancel(context.Background())

	// Server side: listen on I2P and forward to local HTTP service
	i2pListener, err := h.Garlic.ListenStream()
	if err != nil {
		return fmt.Errorf("failed to start I2P listener: %w", err)
	}
	h.listener = i2pListener
	defer i2pListener.Close()
	defer h.Stop()

	// Client side: create HTTP proxy for outbound I2P connections
	proxy := goproxy.NewProxyHttpServer()
	h.proxyServer = proxy
	proxy.Tr.DialContext = h.DialContext

	proxyAddr := net.JoinHostPort(h.TunnelConfig.Interface, strconv.Itoa(h.TunnelConfig.Port))
	proxyListener, err := net.Listen("tcp", proxyAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", proxyAddr, err)
	}

	// Wrap proxy listener with HTTP client-side filtering
	filteredProxyListener := httpinspector.New(proxyListener, h.ClientConfig)

	h.httpServer = &http.Server{Handler: proxy}

	// Start HTTP proxy in background
	proxyErrCh := make(chan error, 1)
	go func() {
		proxyErrCh <- h.httpServer.Serve(filteredProxyListener)
	}()

	h.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning

	// Server side: wrap I2P listener with filtering and rate limiting
	filteredI2PListener := httpinspector.New(i2pListener, h.ServerConfig)
	limitedI2PListener := limitedlistener.NewLimitedListener(
		filteredI2PListener,
		limitedlistener.WithMaxConnections(h.LimitedConfig.MaxConns),
		limitedlistener.WithRateLimit(h.LimitedConfig.RateLimit),
	)

	for {
		select {
		case <-h.done:
			return nil
		case err := <-proxyErrCh:
			if err != nil && err != http.ErrServerClosed {
				h.recordError(err)
			}
			return err
		default:
			con, err := limitedI2PListener.Accept()
			if err != nil {
				select {
				case <-h.done:
					return nil
				default:
				}
				time.Sleep(50 * time.Millisecond)
				continue
			}
			go h.handleServerConnection(con)
		}
	}
}

// handleServerConnection forwards a single inbound I2P connection to the local HTTP service.
func (h *HTTPBidirectional) handleServerConnection(con net.Conn) {
	defer con.Close()
	lCon, err := net.Dial("tcp", h.Target())
	if err != nil {
		h.recordError(err)
		return
	}
	defer lCon.Close()
	ctx := context.Background()
	stream.Forward(ctx, con, lCon, config.DefaultConfig())
}

// Status returns the current tunnel status.
func (h *HTTPBidirectional) Status() i2ptunnel.I2PTunnelStatus {
	return h.I2PTunnelStatus
}

// shutdownTimeout is the maximum time to wait for graceful HTTP server shutdown.
// After this duration, Shutdown returns context.DeadlineExceeded and in-flight
// connections are abandoned. 30 seconds is generous for most HTTP workloads.
const shutdownTimeout = 30 * time.Second

// Stop gracefully shuts down both the server and HTTP proxy sides.
// Safe to call multiple times.
// Uses a bounded timeout context to prevent indefinite blocking on lingering connections.
// Closes the Garlic (I2P SAM session) to release network resources.
func (h *HTTPBidirectional) Stop() error {
	h.stopOnce.Do(func() {
		close(h.done)
		if h.listener != nil {
			h.listener.Close()
		}
		if h.httpServer != nil {
			// Always use a timeout-bounded context for shutdown.
			// Previously this used h.ctx (nil if Start() never ran) or
			// fell back to context.Background() (no deadline), either of
			// which could block indefinitely with lingering connections.
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer shutdownCancel()
			h.httpServer.Shutdown(shutdownCtx)
		}
		if h.Garlic != nil {
			h.Garlic.Close()
		}
		if h.cancel != nil {
			h.cancel()
		}
	})
	h.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped
	return nil
}

// Target returns the local HTTP service address for inbound I2P forwarding.
func (h *HTTPBidirectional) Target() string {
	return h.Addr.String()
}

// Type returns the tunnel type identifier.
func (h *HTTPBidirectional) Type() string {
	return h.TunnelConfig.Type
}

// ID returns a clean identifier derived from the tunnel name.
func (h *HTTPBidirectional) ID() string {
	return i2ptunnel.Clean(h.Name())
}

// Options returns the tunnel's configuration as a string map.
func (h *HTTPBidirectional) Options() map[string]string {
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
	return options
}

// SetOptions applies configuration options from a string map with validation.
func (h *HTTPBidirectional) SetOptions(opts map[string]string) error {
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
	return nil
}

// LoadConfig loads tunnel configuration from a file. The tunnel must be stopped first.
func (h *HTTPBidirectional) LoadConfig(path string) error {
	if h.I2PTunnelStatus == i2ptunnel.I2PTunnelStatusRunning ||
		h.I2PTunnelStatus == i2ptunnel.I2PTunnelStatusStarting {
		return fmt.Errorf("cannot load config while tunnel is %s - stop tunnel first", h.I2PTunnelStatus)
	}

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

	if newConfig.Type != "httpbidirectional" {
		return fmt.Errorf("config file contains %s tunnel, expected httpbidirectional", newConfig.Type)
	}

	targetAddr, err := net.ResolveTCPAddr("tcp", newConfig.Target)
	if err != nil {
		return fmt.Errorf("invalid target address in config: %w", err)
	}

	h.TunnelConfig = *newConfig
	h.Addr = targetAddr
	return nil
}
