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
	limitedI2PListener := limitedlistener.NewLimitedListener(i2pListener, limitedlistener.WithMaxConnections(t.LimitedConfig.MaxConns), limitedlistener.WithRateLimit(t.LimitedConfig.RateLimit))
	consecutiveErrors := 0
	for {
		select {
		case <-t.done:
			return nil
		default:
			con, err := limitedI2PListener.Accept()
			if err != nil {
				if (err == limitedlistener.ErrMaxConnsReached || err == limitedlistener.ErrRateLimitExceeded) && t.Metrics != nil {
					t.Metrics.RecordRateLimitHit()
				}
				select {
				case <-t.done:
					return nil
				default:
				}
				if err != limitedlistener.ErrMaxConnsReached && err != limitedlistener.ErrRateLimitExceeded {
					consecutiveErrors++
					t.RecordError(fmt.Errorf("accept error (%d consecutive): %w", consecutiveErrors, err))
					if consecutiveErrors >= maxConsecutiveErrors {
						t.SetStatus(i2ptunnel.I2PTunnelStatusFailed)
						return fmt.Errorf("listener failed after %d consecutive accept errors", consecutiveErrors)
					}
				}
				time.Sleep(50 * time.Millisecond)
				continue
			}
			consecutiveErrors = 0
			go t.handleConnection(con)
		}
	}
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
// Supported formats: .properties, .ini, .yaml/.yml
//
// Why: Production deployments need to reload configuration without recreating tunnel objects.
// Design: Uses go-i2ptunnel-config library for parsing. Preserves SAM connection and I2P keys.
func (t *TCPServer) LoadConfig(path string) error {
	// Prevent config changes while tunnel is running to avoid race conditions
	status := t.Status()
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
	if newConfig.Type != "tcpserver" {
		return fmt.Errorf("config file contains %s tunnel, expected tcpserver", newConfig.Type)
	}

	// Validate target address (local service) before applying changes
	targetAddr, err := net.ResolveTCPAddr("tcp", newConfig.Target)
	if err != nil {
		return fmt.Errorf("invalid target address in config: %w", err)
	}

	// Update mutable configuration fields
	t.TunnelConfig = *newConfig
	t.Addr = targetAddr

	return nil
}
