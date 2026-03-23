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
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/onramp"
)

var implementTCPServer i2ptunnel.I2PTunnel = &TCPServer{}

type TCPServer struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The local TCP service address
	net.Addr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// The rate-limiting configuration
	limitedlistener.LimitedConfig
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
func (t *TCPServer) SetTunnelMetrics(m *metrics.TunnelMetrics) {
	t.Metrics = m
}

func (t *TCPServer) recordError(err error) {
	t.ErrorTracker.Record(t, err)
	if t.Metrics != nil {
		t.Metrics.RecordError()
	}
}

func (t *TCPServer) setStatus(s i2ptunnel.I2PTunnelStatus) {
	t.statusMu.Lock()
	t.I2PTunnelStatus = s
	t.statusMu.Unlock()
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

// Get the tunnel's error message
func (t *TCPServer) Error() error {
	return t.ErrorTracker.Last()
}

// Get the tunnel's local host:port
func (t *TCPServer) LocalAddress() (string, error) {
	addr := net.JoinHostPort(t.TunnelConfig.Interface, strconv.Itoa(t.TunnelConfig.Port))
	return addr, nil
}

// Get the tunnel's name
func (t *TCPServer) Name() string {
	return t.TunnelConfig.Name
}

// Start the tunnel.
// Each incoming I2P connection is forwarded to the local target service in a separate goroutine.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (t *TCPServer) Start() error {
	t.lifeMu.Lock()
	t.done = make(chan struct{})
	t.stopOnce = sync.Once{}
	i2pListener, err := t.Garlic.ListenStream()
	if err != nil {
		t.lifeMu.Unlock()
		return err
	}
	t.listener = i2pListener
	t.lifeMu.Unlock()
	defer i2pListener.Close()
	defer t.Stop()
	t.setStatus(i2ptunnel.I2PTunnelStatusRunning)
	if t.Metrics != nil {
		t.Metrics.RecordStart()
	}
	limitedI2PListener := limitedlistener.NewLimitedListener(i2pListener, limitedlistener.WithMaxConnections(t.LimitedConfig.MaxConns), limitedlistener.WithRateLimit(t.LimitedConfig.RateLimit))
	for {
		select {
		case <-t.done:
			return nil
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

// Get the tunnel's status
func (t *TCPServer) Status() i2ptunnel.I2PTunnelStatus {
	t.statusMu.RLock()
	defer t.statusMu.RUnlock()
	return t.I2PTunnelStatus
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
		t.setStatus(i2ptunnel.I2PTunnelStatusStopped)
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

// Get the tunnel's type
func (t *TCPServer) Type() string {
	return t.TunnelConfig.Type
}

// Get the tunnel's ID
func (t *TCPServer) ID() string {
	return i2ptunnel.Clean(t.Name())
}

// Get the tunnel's options
func (t *TCPServer) Options() map[string]string {
	// Return basic configuration options as a map
	options := make(map[string]string)
	options["name"] = t.TunnelConfig.Name
	options["type"] = t.TunnelConfig.Type
	options["interface"] = t.TunnelConfig.Interface
	options["port"] = strconv.Itoa(t.TunnelConfig.Port)
	options["maxconns"] = strconv.Itoa(t.LimitedConfig.MaxConns)
	options["ratelimit"] = strconv.FormatFloat(t.LimitedConfig.RateLimit, 'f', -1, 64)
	if t.Addr != nil {
		options["target"] = t.Addr.String()
	}
	i2ptunnel.MergeI2CPOptions(t.TunnelConfig.I2CP, options)
	return options
}

// Set the tunnel's options
func (t *TCPServer) SetOptions(opts map[string]string) error {
	// Apply configuration options from the map with validation
	if name, ok := opts["name"]; ok {
		if err := validate.RequiredString("name", name); err != nil {
			return err
		}
		t.TunnelConfig.Name = name
	}
	if iface, ok := opts["interface"]; ok {
		if err := validate.Interface(iface); err != nil {
			return err
		}
		t.TunnelConfig.Interface = iface
	}
	if portStr, ok := opts["port"]; ok {
		port, err := validate.PortString(portStr)
		if err != nil {
			return err
		}
		t.TunnelConfig.Port = port
	}
	if maxconnsStr, ok := opts["maxconns"]; ok {
		maxconns, err := strconv.Atoi(maxconnsStr)
		if err != nil {
			return fmt.Errorf("invalid maxconns value: %s", maxconnsStr)
		}
		if err := validate.MaxConnections(maxconns); err != nil {
			return err
		}
		t.LimitedConfig.MaxConns = maxconns
	}
	if ratelimitStr, ok := opts["ratelimit"]; ok {
		ratelimit, err := validate.RateLimitString(ratelimitStr)
		if err != nil {
			return err
		}
		t.LimitedConfig.RateLimit = ratelimit
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
	// Apply I2CP options (encrypted LeaseSet, authentication, etc.)
	if i2cpOpts := i2ptunnel.ExtractI2CPOptions(opts); i2cpOpts != nil {
		if t.TunnelConfig.I2CP == nil {
			t.TunnelConfig.I2CP = make(map[string]interface{})
		}
		for k, v := range i2cpOpts {
			t.TunnelConfig.I2CP[k] = v
		}
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
