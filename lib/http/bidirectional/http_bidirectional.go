// Package httpbidirectional implements an HTTP bidirectional tunnel that
// combines an HTTP server tunnel with an HTTP proxy client on the same I2P
// keys, enabling both inbound and outbound I2P HTTP connections.
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
	"strconv"
	"sync"
	"time"

	httpinspector "github.com/go-i2p/go-connfilter/http"
	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/stream"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	httpclient "github.com/go-i2p/go-i2ptunnel/lib/http/client"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
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
	// TunnelBase provides Name, ID, Type, Status, Error, SetTunnelMetrics, SetStatus, RecordError, and the common Options/SetOptions keys.
	i2ptunnel.TunnelBase
	// The local HTTP service address (forward target for inbound I2P connections)
	net.Addr
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
	// Mutex protecting lifecycle fields (done, stopOnce, listener) during Start/Stop transitions.
	// Prevents the race where Start() resets stopOnce while Stop() is calling stopOnce.Do().
	lifeMu sync.Mutex
	// Jump service client for resolving human-readable .i2p hostnames.
	// Uses an I2P-routed HTTP client so stats.i2p is reachable.
	// Initialized in NewHTTPBidirectional; nil disables jump service lookup.
	Jump *httpclient.JumpService
	// Outproxy routes clearnet (non-.i2p) HTTP requests through an I2P exit node.
	// Nil or inactive means clearnet requests are rejected with an error.
	Outproxy *httpclient.Outproxy
	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// Address returns the tunnel's I2P address.
func (h *HTTPBidirectional) Address() string {
	if h.Garlic != nil && h.Garlic.ServiceKeys != nil {
		return h.Garlic.ServiceKeys.Addr().Base32()
	}
	return ""
}

// LocalAddress returns the HTTP proxy listen address.
func (h *HTTPBidirectional) LocalAddress() (string, error) {
	addr := net.JoinHostPort(h.TunnelConfig.Interface, strconv.Itoa(h.TunnelConfig.Port))
	return addr, nil
}

// maxConsecutiveErrors is the number of consecutive Accept() failures before
// the tunnel transitions to I2PTunnelStatusFailed.
const maxConsecutiveErrors = 10

// Start launches both the server-side I2P listener (forwarding to the local
// HTTP service) and the client-side HTTP proxy concurrently.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (h *HTTPBidirectional) Start() error {
	h.lifeMu.Lock()
	h.done = make(chan struct{})
	h.stopOnce = sync.Once{}
	h.SetStatus(i2ptunnel.I2PTunnelStatusStarting)
	h.ctx, h.cancel = context.WithCancel(context.Background())

	// Server side: listen on I2P and forward to local HTTP service
	i2pListener, err := h.Garlic.ListenStream()
	if err != nil {
		h.lifeMu.Unlock()
		return fmt.Errorf("failed to start I2P listener: %w", err)
	}
	h.listener = i2pListener
	h.lifeMu.Unlock()
	defer i2pListener.Close()
	defer h.Stop()

	proxyErrCh, err := h.startHTTPProxy()
	if err != nil {
		return err
	}

	h.SetStatus(i2ptunnel.I2PTunnelStatusRunning)
	if h.Metrics != nil {
		h.Metrics.RecordStart()
	}

	return h.runAcceptLoop(i2pListener, proxyErrCh)
}

// startHTTPProxy creates and starts the outbound HTTP proxy goroutine.
// Returns an error channel that delivers the server's exit error.
func (h *HTTPBidirectional) startHTTPProxy() (chan error, error) {
	proxy := goproxy.NewProxyHttpServer()
	h.proxyServer = proxy
	proxy.Tr.DialContext = h.DialContext

	proxyAddr := net.JoinHostPort(h.TunnelConfig.Interface, strconv.Itoa(h.TunnelConfig.Port))
	proxyListener, err := net.Listen("tcp", proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", proxyAddr, err)
	}
	filteredProxyListener := httpinspector.New(proxyListener, h.ClientConfig)
	h.httpServer = &http.Server{Handler: proxy}
	proxyErrCh := make(chan error, 1)
	go func() {
		proxyErrCh <- h.httpServer.Serve(filteredProxyListener)
	}()
	return proxyErrCh, nil
}

// runAcceptLoop runs the server-side inbound I2P accept loop.
func (h *HTTPBidirectional) runAcceptLoop(i2pListener net.Listener, proxyErrCh chan error) error {
	limitedI2PListener := h.wrapListener(i2pListener)
	consecutiveErrors := 0
	for {
		select {
		case <-h.done:
			return nil
		case err := <-proxyErrCh:
			if err != nil && err != http.ErrServerClosed {
				h.RecordError(err)
				h.SetStatus(i2ptunnel.I2PTunnelStatusFailed)
			}
			return err
		default:
			con, err := limitedI2PListener.Accept()
			if err != nil {
				if cont, fatal := h.handleAcceptError(err, &consecutiveErrors, h.done); !cont {
					return fatal
				}
				continue
			}
			consecutiveErrors = 0
			go h.handleServerConnection(con)
		}
	}
}

// wrapListener wraps a raw listener with HTTP inspection and rate/connection limiting.
func (h *HTTPBidirectional) wrapListener(l net.Listener) net.Listener {
	filtered := httpinspector.New(l, h.ServerConfig)
	return limitedlistener.NewLimitedListener(
		filtered,
		limitedlistener.WithMaxConnections(h.LimitedConfig.MaxConns),
		limitedlistener.WithRateLimit(h.LimitedConfig.RateLimit),
	)
}

// handleAcceptError processes an Accept() failure and returns whether to continue.
func (h *HTTPBidirectional) handleAcceptError(err error, consecutiveErrors *int, done <-chan struct{}) (cont bool, fatal error) {
	if err == limitedlistener.ErrMaxConnsReached || err == limitedlistener.ErrRateLimitExceeded {
		if h.Metrics != nil {
			h.Metrics.RecordRateLimitHit()
		}
	}
	select {
	case <-done:
		return false, nil
	default:
	}
	if err != limitedlistener.ErrMaxConnsReached && err != limitedlistener.ErrRateLimitExceeded {
		*consecutiveErrors++
		h.RecordError(fmt.Errorf("accept error (%d consecutive): %w", *consecutiveErrors, err))
		if *consecutiveErrors >= maxConsecutiveErrors {
			h.SetStatus(i2ptunnel.I2PTunnelStatusFailed)
			return false, fmt.Errorf("listener failed after %d consecutive accept errors", *consecutiveErrors)
		}
	}
	time.Sleep(50 * time.Millisecond)
	return true, nil
}

// handleServerConnection forwards a single inbound I2P connection to the local HTTP service.
func (h *HTTPBidirectional) handleServerConnection(con net.Conn) {
	if h.Metrics != nil {
		h.Metrics.RecordConnection()
	}
	wrapped := metrics.WrapConn(con, h.Metrics)
	defer wrapped.Close()
	lCon, err := net.Dial("tcp", h.Target())
	if err != nil {
		h.RecordError(err)
		if h.Metrics != nil {
			h.Metrics.RecordConnectionFailed()
		}
		return
	}
	defer lCon.Close()
	ctx := context.Background()
	stream.Forward(ctx, wrapped, lCon, config.DefaultConfig())
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
	h.lifeMu.Lock()
	defer h.lifeMu.Unlock()
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
		h.SetStatus(i2ptunnel.I2PTunnelStatusStopped)
		if h.Metrics != nil {
			h.Metrics.RecordStop()
		}
	})
	return nil
}

// Target returns the local HTTP service address for inbound I2P forwarding.
func (h *HTTPBidirectional) Target() string {
	return h.Addr.String()
}

// Options returns the tunnel's configuration as a string map.
func (h *HTTPBidirectional) Options() map[string]string {
	options := h.TunnelBase.Options()
	if h.Addr != nil {
		options["target"] = h.Addr.String()
	}
	if h.Outproxy != nil {
		options["outproxy"] = h.Outproxy.Address
		if h.Outproxy.Enabled {
			options["outproxy.enabled"] = "true"
		} else {
			options["outproxy.enabled"] = "false"
		}
	}
	return options
}

// SetOptions applies configuration options from a string map with validation.
func (h *HTTPBidirectional) SetOptions(opts map[string]string) error {
	if err := h.TunnelBase.SetOptions(opts); err != nil {
		return err
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
	h.applyOutproxyOpts(opts)
	return nil
}

// applyOutproxyOpts updates the outproxy config from opts if present.
func (h *HTTPBidirectional) applyOutproxyOpts(opts map[string]string) {
	if outproxy, ok := opts["outproxy"]; ok {
		if h.Outproxy == nil {
			h.Outproxy = &httpclient.Outproxy{}
		}
		h.Outproxy.Address = outproxy
	}
	if enabledStr, ok := opts["outproxy.enabled"]; ok {
		if h.Outproxy == nil {
			h.Outproxy = &httpclient.Outproxy{}
		}
		h.Outproxy.Enabled = enabledStr == "true" || enabledStr == "1"
	}
}

// LoadConfig loads tunnel configuration from a file. The tunnel must be stopped first.
func (h *HTTPBidirectional) LoadConfig(path string) error {
	if err := i2ptunnel.CheckTunnelStopped(h.Status()); err != nil {
		return err
	}
	newConfig, err := i2ptunnel.ParseConfigFile(path)
	if err != nil {
		return err
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
