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
	"strconv"
	"sync"

	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/stream"
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
	// TunnelBase provides Name, ID, Type, Status, Error, SetTunnelMetrics,
	// SetStatus, RecordError, and the common Options/SetOptions keys.
	i2ptunnel.TunnelBase
	// The local TCP service address (forward target for inbound I2P connections)
	net.Addr
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
	// TCPFilterConfig holds optional byte-level read/write filters applied to each
	// inbound I2P connection. Nil means no filtering (passthrough).
	*i2ptunnel.TCPFilterConfig
}

// Address returns the tunnel's I2P address.
func (t *TCPBidirectional) Address() string {
	if t.Garlic != nil && t.Garlic.ServiceKeys != nil {
		return t.Garlic.ServiceKeys.Addr().Base32()
	}
	return ""
}

// LocalAddress returns the SOCKS5 proxy listen address.
func (t *TCPBidirectional) LocalAddress() (string, error) {
	addr := net.JoinHostPort(t.TunnelConfig.Interface, strconv.Itoa(t.TunnelConfig.Port))
	return addr, nil
}

// maxConsecutiveErrors is the number of consecutive Accept() failures before
// the tunnel transitions to I2PTunnelStatusFailed.
const maxConsecutiveErrors = 10

// Start launches both the server-side I2P listener (forwarding to the local
// target) and the client-side SOCKS5 proxy concurrently. It blocks until the
// tunnel is stopped.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (t *TCPBidirectional) Start() error {
	t.lifeMu.Lock()
	t.done = make(chan struct{})
	t.stopOnce = sync.Once{}
	t.SetStatus(i2ptunnel.I2PTunnelStatusStarting)

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

	socksErrCh, err := t.startSOCKS5Proxy()
	if err != nil {
		return err
	}

	t.SetStatus(i2ptunnel.I2PTunnelStatusRunning)
	if t.Metrics != nil {
		t.Metrics.RecordStart()
	}

	return t.runAcceptLoop(i2pListener, socksErrCh)
}

// runAcceptLoop wraps the I2P listener and runs the server-side accept loop.
func (t *TCPBidirectional) runAcceptLoop(i2pListener net.Listener, socksErrCh <-chan error) error {
	limitedI2PListener := limitedlistener.NewLimitedListener(
		i2pListener,
		limitedlistener.WithMaxConnections(t.LimitedConfig.MaxConns),
		limitedlistener.WithRateLimit(t.LimitedConfig.RateLimit),
	)
	return t.runBidirectionalAcceptLoop(limitedI2PListener, socksErrCh)
}

// runBidirectionalAcceptLoop is the core accept loop with socks error monitoring.
func (t *TCPBidirectional) runBidirectionalAcceptLoop(l net.Listener, socksErrCh <-chan error) error {
	consecutiveErrors := 0
	for {
		select {
		case <-t.done:
			return nil
		case err := <-socksErrCh:
			return t.handleSocksError(err)
		default:
			if cont, fatal := t.acceptOne(l, &consecutiveErrors); !cont {
				return fatal
			}
		}
	}
}

// handleSocksError records a non-nil SOCKS error and transitions to failed status.
func (t *TCPBidirectional) handleSocksError(err error) error {
	if err != nil {
		t.RecordError(err)
		t.SetStatus(i2ptunnel.I2PTunnelStatusFailed)
	}
	return err
}

// acceptOne accepts one connection and dispatches it, returning (cont, fatal).
func (t *TCPBidirectional) acceptOne(l net.Listener, consecutiveErrors *int) (cont bool, fatal error) {
	con, err := l.Accept()
	if err != nil {
		return t.handleAcceptError(err, consecutiveErrors, t.done)
	}
	*consecutiveErrors = 0
	go t.handleServerConnection(con)
	return true, nil
}

// startSOCKS5Proxy creates and starts the outbound SOCKS5 proxy goroutine.
// Returns the error channel for monitoring the proxy.
func (t *TCPBidirectional) startSOCKS5Proxy() (<-chan error, error) {
	socksAddr := net.JoinHostPort(t.TunnelConfig.Interface, strconv.Itoa(t.TunnelConfig.Port))
	socksServer, err := socks5.NewClassicServer(socksAddr, "", "", "", 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCKS5 server: %w", err)
	}
	t.socksServer = socksServer
	t.socksServer.Handle = &socksHandler{garlic: t.Garlic}

	socksErrCh := make(chan error, 1)
	go func() {
		socksErrCh <- t.socksServer.ListenAndServe(t.socksServer.Handle)
	}()
	return socksErrCh, nil
}

// handleAcceptError processes an Accept() failure and returns whether to continue.
// cont=true means sleep-and-retry; cont=false with nil fatal means shutting down;
// cont=false with non-nil fatal means the tunnel should fail.
func (t *TCPBidirectional) handleAcceptError(err error, consecutiveErrors *int, done <-chan struct{}) (cont bool, fatal error) {
	return t.TunnelBase.HandleAcceptError(err, consecutiveErrors, maxConsecutiveErrors, done)
}

// handleServerConnection forwards a single inbound I2P connection to the local target.
func (t *TCPBidirectional) handleServerConnection(con net.Conn) {
	if t.Metrics != nil {
		t.Metrics.RecordConnection()
	}
	wrapped := metrics.WrapConn(con, t.Metrics)
	defer wrapped.Close()
	filtered, err := i2ptunnel.ApplyTCPFilter(wrapped, t.TCPFilterConfig)
	if err != nil {
		t.RecordError(err)
		return
	}
	lCon, err := net.Dial("tcp", t.Target())
	if err != nil {
		t.RecordError(err)
		if t.Metrics != nil {
			t.Metrics.RecordConnectionFailed()
		}
		return
	}
	defer lCon.Close()
	ctx := context.Background()
	stream.Forward(ctx, filtered, lCon, config.DefaultConfig())
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
		t.SetStatus(i2ptunnel.I2PTunnelStatusStopped)
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

// Options returns the tunnel's configuration as a string map.
func (t *TCPBidirectional) Options() map[string]string {
	options := t.TunnelBase.Options()
	if t.Addr != nil {
		options["target"] = t.Addr.String()
	}
	return options
}

// SetOptions applies configuration options from a string map with validation.
func (t *TCPBidirectional) SetOptions(opts map[string]string) error {
	if err := t.TunnelBase.SetOptions(opts); err != nil {
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
	if err := i2ptunnel.CheckTunnelStopped(t.Status()); err != nil {
		return err
	}
	newConfig, err := i2ptunnel.ParseConfigFile(path)
	if err != nil {
		return err
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
