// Package tcpbidirectional implements a TCP bidirectional tunnel that
// combines a TCP server tunnel with a SOCKS5 proxy client on the same I2P
// keys, enabling both inbound and outbound I2P connections.
package tcpbidirectional

// TCP Bidirectional Tunnel
//
// A TCP bidirectional tunnel combines:
// 1. A TCP server tunnel (forwarding incoming I2P connections to a local service)
// 2. A SOCKS5 proxy (allowing local apps to reach arbitrary I2P destinations)
//
// Both sides share the same I2P identity (keys), meaning the tunnel's I2P address
// is reachable for inbound connections while also usable for outbound connections.

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/stream"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/onramp"
	"github.com/txthinking/socks5"
)

var implementTCPBidirectional i2ptunnel.I2PTunnel = &TCPBidirectional{}

// TCPBidirectional combines a TCP server tunnel with a SOCKS5 proxy client
// on the same I2P keys, enabling both inbound and outbound I2P connections.
type TCPBidirectional struct {
	// I2P connection (shared for both server and client sides)
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The local TCP service address (forward target for inbound I2P connections)
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
func (t *TCPBidirectional) SetTunnelMetrics(m *metrics.TunnelMetrics) {
	t.Metrics = m
}

func (t *TCPBidirectional) recordError(err error) {
	t.ErrorTracker.Record(t, err)
	if t.Metrics != nil {
		t.Metrics.RecordError()
	}
}

func (t *TCPBidirectional) setStatus(s i2ptunnel.I2PTunnelStatus) {
	t.statusMu.Lock()
	t.I2PTunnelStatus = s
	t.statusMu.Unlock()
}

// Address returns the tunnel's I2P address.
func (t *TCPBidirectional) Address() string {
	if t.Garlic != nil && t.Garlic.ServiceKeys != nil {
		return t.Garlic.ServiceKeys.Addr().Base32()
	}
	return ""
}

// Error returns the most recent error, or nil.
func (t *TCPBidirectional) Error() error {
	return t.ErrorTracker.Last()
}

// LocalAddress returns the SOCKS5 proxy listen address.
func (t *TCPBidirectional) LocalAddress() (string, error) {
	addr := net.JoinHostPort(t.TunnelConfig.Interface, strconv.Itoa(t.TunnelConfig.Port))
	return addr, nil
}

// Name returns the tunnel's configured name.
func (t *TCPBidirectional) Name() string {
	return t.TunnelConfig.Name
}

// Start launches both the server-side I2P listener (forwarding to the local
// target) and the client-side SOCKS5 proxy concurrently. It blocks until the
// tunnel is stopped.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (t *TCPBidirectional) Start() error {
	t.lifeMu.Lock()
	t.done = make(chan struct{})
	t.stopOnce = sync.Once{}
	t.setStatus(i2ptunnel.I2PTunnelStatusStarting)

	// Start the server side: listen on I2P and forward to local target
	i2pListener, err := t.Garlic.ListenStream()
	if err != nil {
		t.lifeMu.Unlock()
		return fmt.Errorf("failed to start I2P listener: %w", err)
	}
	t.listener = i2pListener
	t.lifeMu.Unlock()
	defer i2pListener.Close()
	defer t.Stop()

	// Create SOCKS5 proxy for outbound connections
	socksAddr := net.JoinHostPort(t.TunnelConfig.Interface, strconv.Itoa(t.TunnelConfig.Port))
	socksServer, err := socks5.NewClassicServer(socksAddr, "", "", "", 0, 0)
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 server: %w", err)
	}
	t.socksServer = socksServer
	t.socksServer.Handle = &socksHandler{garlic: t.Garlic}

	// Start SOCKS5 proxy in a background goroutine
	socksErrCh := make(chan error, 1)
	go func() {
		socksErrCh <- t.socksServer.ListenAndServe(t.socksServer.Handle)
	}()

	t.setStatus(i2ptunnel.I2PTunnelStatusRunning)
	if t.Metrics != nil {
		t.Metrics.RecordStart()
	}

	// Server-side accept loop with rate limiting
	limitedI2PListener := limitedlistener.NewLimitedListener(
		i2pListener,
		limitedlistener.WithMaxConnections(t.LimitedConfig.MaxConns),
		limitedlistener.WithRateLimit(t.LimitedConfig.RateLimit),
	)
	for {
		select {
		case <-t.done:
			return nil
		case err := <-socksErrCh:
			if err != nil {
				t.recordError(err)
			}
			return err
		default:
			con, err := limitedI2PListener.Accept()
			if err != nil {
				select {
				case <-t.done:
					return nil
				default:
				}
				time.Sleep(50 * time.Millisecond)
				continue
			}
			go t.handleServerConnection(con)
		}
	}
}

// handleServerConnection forwards a single inbound I2P connection to the local target.
func (t *TCPBidirectional) handleServerConnection(con net.Conn) {
	if t.Metrics != nil {
		t.Metrics.RecordConnection()
	}
	wrapped := metrics.WrapConn(con, t.Metrics)
	defer wrapped.Close()
	lCon, err := net.Dial("tcp", t.Target())
	if err != nil {
		t.recordError(err)
		if t.Metrics != nil {
			t.Metrics.RecordConnectionFailed()
		}
		return
	}
	defer lCon.Close()
	ctx := context.Background()
	stream.Forward(ctx, wrapped, lCon, config.DefaultConfig())
}

// Status returns the current tunnel status.
func (t *TCPBidirectional) Status() i2ptunnel.I2PTunnelStatus {
	t.statusMu.RLock()
	defer t.statusMu.RUnlock()
	return t.I2PTunnelStatus
}

// Stop gracefully shuts down both the server and SOCKS5 proxy sides.
// Safe to call multiple times.
// Closes the Garlic (I2P SAM session) to release network resources.
func (t *TCPBidirectional) Stop() error {
	t.lifeMu.Lock()
	defer t.lifeMu.Unlock()
	t.stopOnce.Do(func() {
		close(t.done)
		if t.listener != nil {
			t.listener.Close()
		}
		if t.socksServer != nil {
			t.socksServer.Shutdown()
		}
		if t.Garlic != nil {
			t.Garlic.Close()
		}
		t.setStatus(i2ptunnel.I2PTunnelStatusStopped)
		if t.Metrics != nil {
			t.Metrics.RecordStop()
		}
	})
	return nil
}

// Target returns the local service address for inbound I2P forwarding.
func (t *TCPBidirectional) Target() string {
	return t.Addr.String()
}

// Type returns the tunnel type identifier.
func (t *TCPBidirectional) Type() string {
	return t.TunnelConfig.Type
}

// ID returns a clean identifier derived from the tunnel name.
func (t *TCPBidirectional) ID() string {
	return i2ptunnel.Clean(t.Name())
}

// Options returns the tunnel's configuration as a string map.
func (t *TCPBidirectional) Options() map[string]string {
	options := i2ptunnel.BuildCommonOptions(t.TunnelConfig)
	i2ptunnel.AddRateLimitOptions(options, t.LimitedConfig.MaxConns, t.LimitedConfig.RateLimit)
	if t.Addr != nil {
		options["target"] = t.Addr.String()
	}
	return options
}

// SetOptions applies configuration options from a string map with validation.
func (t *TCPBidirectional) SetOptions(opts map[string]string) error {
	if err := i2ptunnel.ApplyCommonOptions(opts, &t.TunnelConfig); err != nil {
		return err
	}
	if err := i2ptunnel.ApplyRateLimitOptions(opts, &t.LimitedConfig.MaxConns, &t.LimitedConfig.RateLimit); err != nil {
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
		t.Addr = addr
	}
	return nil
}

// LoadConfig loads tunnel configuration from a file. The tunnel must be stopped first.
func (t *TCPBidirectional) LoadConfig(path string) error {
	status := t.Status()
	if status == i2ptunnel.I2PTunnelStatusRunning ||
		status == i2ptunnel.I2PTunnelStatusStarting {
		return fmt.Errorf("cannot load config while tunnel is %s - stop tunnel first", status)
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

	if newConfig.Type != "tcpbidirectional" {
		return fmt.Errorf("config file contains %s tunnel, expected tcpbidirectional", newConfig.Type)
	}

	targetAddr, err := net.ResolveTCPAddr("tcp", newConfig.Target)
	if err != nil {
		return fmt.Errorf("invalid target address in config: %w", err)
	}

	t.TunnelConfig = *newConfig
	t.Addr = targetAddr
	return nil
}
