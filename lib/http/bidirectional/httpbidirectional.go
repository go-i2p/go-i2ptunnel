package httpbidirectional

// HTTP Bidirectional Tunnel
//
// An HTTP bidirectional tunnel combines:
// 1. An HTTP server tunnel (forwarding incoming I2P connections to a local HTTP service)
// 2. A SOCKS5 proxy (allowing local apps to reach arbitrary I2P destinations)
//
// Both sides share the same I2P identity (keys), meaning the tunnel's I2P address
// is reachable for inbound HTTP connections while also providing a SOCKS5 proxy
// for outbound I2P access.

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"

	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/stream"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/onramp"
	"github.com/txthinking/socks5"
)

var implementHTTPBidirectional i2ptunnel.I2PTunnel = &HTTPBidirectional{}

// HTTPBidirectional combines an HTTP server tunnel with a SOCKS5 proxy client
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
	// SOCKS5 server instance for outbound connections
	socksServer *socks5.Server
	// Channel for shutdown signaling
	done chan struct{}
	// Ensures Stop() is only executed once to prevent double-close panic
	stopOnce sync.Once
	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}

func (h *HTTPBidirectional) recordError(err error) {
	h.Errors = append(h.Errors, i2ptunnel.NewError(h, err))
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
	if len(h.Errors) > 0 {
		return h.Errors[len(h.Errors)-1]
	}
	return nil
}

// LocalAddress returns the SOCKS5 proxy listen address.
func (h *HTTPBidirectional) LocalAddress() (string, error) {
	addr := net.JoinHostPort(h.TunnelConfig.Interface, strconv.Itoa(h.TunnelConfig.Port))
	return addr, nil
}

// Name returns the tunnel's configured name.
func (h *HTTPBidirectional) Name() string {
	return h.TunnelConfig.Name
}

// Start launches both the server-side I2P listener (forwarding to the local
// HTTP service) and the client-side SOCKS5 proxy concurrently.
func (h *HTTPBidirectional) Start() error {
	h.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStarting

	// Start the server side: listen on I2P and forward to local HTTP service
	i2pListener, err := h.Garlic.ListenStream()
	if err != nil {
		return fmt.Errorf("failed to start I2P listener: %w", err)
	}
	defer i2pListener.Close()
	defer h.Stop()

	// Create SOCKS5 proxy for outbound connections
	socksAddr := net.JoinHostPort(h.TunnelConfig.Interface, strconv.Itoa(h.TunnelConfig.Port))
	socksServer, err := socks5.NewClassicServer(socksAddr, "", "", "", 0, 0)
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 server: %w", err)
	}
	h.socksServer = socksServer
	h.socksServer.Handle = &socksHandler{garlic: h.Garlic}

	// Start SOCKS5 proxy in a background goroutine
	socksErrCh := make(chan error, 1)
	go func() {
		socksErrCh <- h.socksServer.ListenAndServe(h.socksServer.Handle)
	}()

	h.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning

	// Server-side accept loop with rate limiting
	limitedI2PListener := limitedlistener.NewLimitedListener(
		i2pListener,
		limitedlistener.WithMaxConnections(h.LimitedConfig.MaxConns),
		limitedlistener.WithRateLimit(h.LimitedConfig.RateLimit),
	)
	for {
		select {
		case <-h.done:
			return nil
		case err := <-socksErrCh:
			if err != nil {
				h.recordError(err)
			}
			return err
		default:
			con, err := limitedI2PListener.Accept()
			if err != nil {
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

// Stop gracefully shuts down both the server and SOCKS5 proxy sides.
// Safe to call multiple times.
func (h *HTTPBidirectional) Stop() error {
	h.stopOnce.Do(func() {
		close(h.done)
		if h.socksServer != nil {
			h.socksServer.Shutdown()
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
