// Package httpserver implements a reverse proxy that forwards HTTP traffic
// from I2P clients to a local HTTP service.
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
	"strconv"
	"sync"

	httpinspector "github.com/go-i2p/go-connfilter/http"
	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/stream"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/onramp"
)

var implementHTTPServer i2ptunnel.I2PTunnel = &HTTPServer{}

// HTTPServer is an HTTP reverse-proxy tunnel that accepts connections from the I2P
// network and forwards them to a local HTTP service. It applies rate-limiting via
// LimitedListener and request filtering via httpinspector.
type HTTPServer struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// TunnelBase provides Name, ID, Type, Status, Error, SetTunnelMetrics, SetStatus, RecordError, and the common Options/SetOptions keys.
	i2ptunnel.TunnelBase
	// The local HTTP service address
	net.Addr
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
}

// Get the tunnel's I2P address
func (h *HTTPServer) Address() string {
	// For HTTP server, return the service address if available
	if h.Garlic != nil && h.Garlic.ServiceKeys != nil {
		return h.Garlic.ServiceKeys.Addr().Base32()
	}
	return ""
}

// Get the tunnel's local host:port
func (h *HTTPServer) LocalAddress() (string, error) {
	addr := net.JoinHostPort(h.TunnelConfig.Interface, strconv.Itoa(h.TunnelConfig.Port))
	return addr, nil
}

// maxConsecutiveErrors is the number of consecutive Accept() failures before
// the tunnel transitions to I2PTunnelStatusFailed.
const maxConsecutiveErrors = 10

// Start the tunnel.
// Each incoming I2P connection is forwarded to the local HTTP service in a separate goroutine.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (h *HTTPServer) Start() error {
	h.lifeMu.Lock()
	h.done = make(chan struct{})
	h.stopOnce = sync.Once{}
	h.SetStatus(i2ptunnel.I2PTunnelStatusStarting)
	i2pListener, err := h.Garlic.ListenStream()
	if err != nil {
		h.lifeMu.Unlock()
		return err
	}
	h.listener = i2pListener
	h.lifeMu.Unlock()
	defer i2pListener.Close()
	defer h.Stop()
	h.SetStatus(i2ptunnel.I2PTunnelStatusRunning)
	if h.Metrics != nil {
		h.Metrics.RecordStart()
	}
	return h.runAcceptLoop(i2pListener)
}

// runAcceptLoop wraps the I2P listener and runs the inbound connection accept loop.
func (h *HTTPServer) runAcceptLoop(i2pListener net.Listener) error {
	limitedI2PListener := limitedlistener.NewLimitedListener(i2pListener, limitedlistener.WithMaxConnections(h.LimitedConfig.MaxConns), limitedlistener.WithRateLimit(h.LimitedConfig.RateLimit))
	httpInspectorListener := httpinspector.New(limitedI2PListener, h.Config)
	return h.TunnelBase.RunAcceptDispatch(httpInspectorListener, maxConsecutiveErrors, h.done, h.handleConnection)
}

// handleAcceptError processes an Accept() failure and returns whether to continue.
func (h *HTTPServer) handleAcceptError(err error, consecutiveErrors *int, done <-chan struct{}) (cont bool, fatal error) {
	return h.TunnelBase.HandleAcceptError(err, consecutiveErrors, maxConsecutiveErrors, done)
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
		h.SetStatus(i2ptunnel.I2PTunnelStatusStopped)
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

// Get the tunnel's options
func (h *HTTPServer) Options() map[string]string {
	options := h.TunnelBase.Options()
	if h.Addr != nil {
		options["target"] = h.Addr.String()
	}
	return options
}

// Set the tunnel's options
func (h *HTTPServer) SetOptions(opts map[string]string) error {
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
	return nil
}

// LoadConfig loads tunnel configuration from a file and updates the tunnel settings.
// The tunnel must be stopped before calling LoadConfig to prevent inconsistent state.
func (h *HTTPServer) LoadConfig(path string) error {
	if err := i2ptunnel.CheckTunnelStopped(h.Status()); err != nil {
		return err
	}
	newConfig, err := i2ptunnel.ParseConfigFile(path)
	if err != nil {
		return err
	}
	if newConfig.Type != "httpserver" {
		return fmt.Errorf("config file contains %s tunnel, expected httpserver", newConfig.Type)
	}
	targetAddr, err := net.ResolveTCPAddr("tcp", newConfig.Target)
	if err != nil {
		return fmt.Errorf("invalid target address in config: %w", err)
	}
	h.TunnelConfig = *newConfig
	h.Addr = targetAddr
	return nil
}
