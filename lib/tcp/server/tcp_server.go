package tcpserver

/**
TCP Server Tunnel
-----------------

A TCP Server tunnel connects a local TCP service to the I2P network through:
1. A TCP Client component that interfaces with the local service
2. An I2P Service component that maintains a persistent destination address

When an I2P peer connects to the tunnel's destination, the traffic flows:
- Incoming: I2P Network → I2P Service → TCP Client → Local Service
- Outgoing: Local Service → TCP Client → I2P Service → I2P Network
**/

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
)

var implementTCPServer i2ptunnel.I2PTunnel = &TCPServer{}

// TCPServer is a TCP server tunnel that accepts connections from the I2P network
// and forwards them to a local TCP service. It applies rate-limiting via
// LimitedListener and optional byte-level content filtering via TCPFilterConfig.
type TCPServer struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// TunnelBase provides Name, ID, Type, Status, Error, SetTunnelMetrics,
	// SetStatus, RecordError, Options and SetOptions for the common keys.
	i2ptunnel.TunnelBase
	// The local TCP service address
	net.Addr
	// Optional byte-level content filter applied per-connection. Nil means no filtering.
	*i2ptunnel.TCPFilterConfig
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
func (t *TCPServer) Address() string {
	// For server tunnels, return the service address if available
	if t.Garlic != nil {
		// Use service keys to identify the tunnel's I2P address
		if t.Garlic.ServiceKeys != nil {
			return t.Garlic.ServiceKeys.Addr().Base32()
		}
	}
	return ""
}

// Get the tunnel's local host:port
func (t *TCPServer) LocalAddress() (string, error) {
	addr := net.JoinHostPort(t.TunnelConfig.Interface, strconv.Itoa(t.TunnelConfig.Port))
	return addr, nil
}

// maxConsecutiveErrors is the number of consecutive Accept() failures before
// the tunnel transitions to I2PTunnelStatusFailed.
const maxConsecutiveErrors = 10

// Start the tunnel.
// Each incoming I2P connection is forwarded to the local target service in a separate goroutine.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (t *TCPServer) Start() error {
	t.lifeMu.Lock()
	t.done = make(chan struct{})
	t.stopOnce = sync.Once{}
	t.SetStatus(i2ptunnel.I2PTunnelStatusStarting)
	i2pListener, err := t.Garlic.ListenStream()
	if err != nil {
		t.lifeMu.Unlock()
		return err
	}
	t.listener = i2pListener
	t.lifeMu.Unlock()
	defer i2pListener.Close()
	defer t.Stop()
	t.SetStatus(i2ptunnel.I2PTunnelStatusRunning)
	if t.Metrics != nil {
		t.Metrics.RecordStart()
	}
	return t.runAcceptLoop(i2pListener)
}

// runAcceptLoop wraps the I2P listener with rate limiting and runs the accept loop.
func (t *TCPServer) runAcceptLoop(i2pListener net.Listener) error {
	limitedI2PListener := limitedlistener.NewLimitedListener(i2pListener, limitedlistener.WithMaxConnections(t.LimitedConfig.MaxConns), limitedlistener.WithRateLimit(t.LimitedConfig.RateLimit))
	return t.TunnelBase.RunAcceptDispatch(limitedI2PListener, maxConsecutiveErrors, t.done, t.handleConnection)
}

// handleAcceptError processes an Accept() failure and returns whether to continue.
// cont=true means sleep-and-retry; cont=false with nil fatal means shutting down.
func (t *TCPServer) handleAcceptError(err error, consecutiveErrors *int, done <-chan struct{}) (cont bool, fatal error) {
	return t.TunnelBase.HandleAcceptError(err, consecutiveErrors, maxConsecutiveErrors, done)
}

// handleConnection forwards a single I2P connection to the local target service.
// Both connections are closed when forwarding completes.
func (t *TCPServer) handleConnection(con net.Conn) {
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

// Stop the tunnel. Safe to call multiple times.
// Closes the Garlic (I2P SAM session) to release network resources.
func (t *TCPServer) Stop() error {
	t.lifeMu.Lock()
	defer t.lifeMu.Unlock()
	t.stopOnce.Do(func() {
		close(t.done)
		if t.listener != nil {
			t.listener.Close()
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

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (t *TCPServer) Target() string {
	return t.Addr.String()
}

// Get the tunnel's options
func (t *TCPServer) Options() map[string]string {
	options := t.TunnelBase.Options()
	if t.Addr != nil {
		options["target"] = t.Addr.String()
	}
	return options
}

// Set the tunnel's options
func (t *TCPServer) SetOptions(opts map[string]string) error {
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

// LoadConfig loads tunnel configuration from a file and updates the tunnel settings.
// The tunnel must be stopped before calling LoadConfig to prevent inconsistent state.
func (t *TCPServer) LoadConfig(path string) error {
	if err := i2ptunnel.CheckTunnelStopped(t.Status()); err != nil {
		return err
	}
	newConfig, err := i2ptunnel.ParseConfigFile(path)
	if err != nil {
		return err
	}
	if newConfig.Type != "tcpserver" {
		return fmt.Errorf("config file contains %s tunnel, expected tcpserver", newConfig.Type)
	}
	targetAddr, err := net.ResolveTCPAddr("tcp", newConfig.Target)
	if err != nil {
		return fmt.Errorf("invalid target address in config: %w", err)
	}
	t.TunnelConfig = *newConfig
	t.Addr = targetAddr
	return nil
}
