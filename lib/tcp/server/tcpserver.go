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
	// Mutex protecting the Errors slice from concurrent access
	errMu sync.Mutex

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}

func (t *TCPServer) recordError(err error) {
	t.errMu.Lock()
	t.Errors = append(t.Errors, i2ptunnel.NewError(t, err))
	t.errMu.Unlock()
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
	t.errMu.Lock()
	defer t.errMu.Unlock()
	if len(t.Errors) > 0 {
		return t.Errors[len(t.Errors)-1]
	}
	return nil
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
	t.done = make(chan struct{})
	t.stopOnce = sync.Once{}
	i2pListener, err := t.Garlic.ListenStream()
	if err != nil {
		return err
	}
	t.listener = i2pListener
	defer i2pListener.Close()
	defer t.Stop()
	t.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning
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
	defer con.Close()
	lCon, err := net.Dial("tcp", t.Target())
	if err != nil {
		t.recordError(err)
		return
	}
	defer lCon.Close()
	ctx := context.Background()
	stream.Forward(ctx, con, lCon, config.DefaultConfig())
}

// Get the tunnel's status
func (t *TCPServer) Status() i2ptunnel.I2PTunnelStatus {
	return t.I2PTunnelStatus
}

// Stop the tunnel. Safe to call multiple times.
// Closes the Garlic (I2P SAM session) to release network resources.
func (t *TCPServer) Stop() error {
	t.stopOnce.Do(func() {
		close(t.done)
		if t.listener != nil {
			t.listener.Close()
		}
		if t.Garlic != nil {
			t.Garlic.Close()
		}
	})
	t.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped
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
	if t.I2PTunnelStatus == i2ptunnel.I2PTunnelStatusRunning ||
		t.I2PTunnelStatus == i2ptunnel.I2PTunnelStatusStarting {
		return fmt.Errorf("cannot load config while tunnel is %s - stop tunnel first", t.I2PTunnelStatus)
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
