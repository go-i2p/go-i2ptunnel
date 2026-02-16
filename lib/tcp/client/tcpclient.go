package tcpclient

/**
TCP Client Tunnel
-----------------

A TCP Client tunnel operates by:
1. Running a TCP Server that listens on a local port
2. Maintaining an I2P Client connected to a specific destination

When activated:
- Local applications connect to the TCP Server
- Traffic routes through the I2P Client to the target I2P destination
- Creates a secure point-to-point connection

Both tunnel types preserve the original TCP traffic while adding I2P's anonymity and encryption layers.

When a local client connects to the I2P tunnel's destination, the traffic flows:
- Outgoing: Local Client → TCP Server → I2P Client → I2P Network
- Incoming: I2P Network → I2P Client → TCP Server → Local Client
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
	"github.com/go-i2p/i2pkeys"
	"github.com/go-i2p/onramp"
)

var implementTCPClient i2ptunnel.I2PTunnel = &TCPClient{}

type TCPClient struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The remote I2P destination target
	*i2pkeys.I2PAddr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
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

func (t *TCPClient) recordError(err error) {
	t.errMu.Lock()
	t.Errors = append(t.Errors, i2ptunnel.NewError(t, err))
	t.errMu.Unlock()
}

// Get the tunnel's I2P address
func (t *TCPClient) Address() string {
	// Return the target I2P address for client tunnels
	if t.I2PAddr != nil {
		return t.I2PAddr.Base32()
	}
	return ""
}

// Get the tunnel's error message
func (t *TCPClient) Error() error {
	t.errMu.Lock()
	defer t.errMu.Unlock()
	if len(t.Errors) > 0 {
		return t.Errors[len(t.Errors)-1]
	}
	return nil
}

// Get the tunnel's local host:port
func (t *TCPClient) LocalAddress() (string, error) {
	addr := net.JoinHostPort(t.TunnelConfig.Interface, strconv.Itoa(t.TunnelConfig.Port))
	return addr, nil
}

// Get the tunnel's name
func (t *TCPClient) Name() string {
	return t.TunnelConfig.Name
}

// Start the tunnel.
// Each accepted local connection gets its own I2P stream to the target destination.
// Connections are handled concurrently in separate goroutines.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (t *TCPClient) Start() error {
	t.done = make(chan struct{})
	t.stopOnce = sync.Once{}
	t.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStarting
	listener, err := net.Listen("tcp", net.JoinHostPort(t.Interface, strconv.Itoa(t.Port)))
	if err != nil {
		return err
	}
	t.listener = listener
	defer listener.Close()
	defer t.Stop()
	t.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning
	for {
		select {
		case <-t.done:
			return nil
		default:
			con, err := listener.Accept()
			if err != nil {
				// Check if tunnel is shutting down
				select {
				case <-t.done:
					return nil
				default:
				}
				// Backoff to prevent CPU-burning tight loop on persistent errors
				time.Sleep(50 * time.Millisecond)
				continue
			}
			go t.handleConnection(con)
		}
	}
}

// handleConnection forwards a single local connection over its own I2P stream.
// Both connections are closed when forwarding completes.
func (t *TCPClient) handleConnection(con net.Conn) {
	defer con.Close()
	i2pConn, err := t.Garlic.Dial("tcp", t.Target())
	if err != nil {
		t.recordError(err)
		return
	}
	defer i2pConn.Close()
	ctx := context.Background()
	stream.Forward(ctx, con, i2pConn, config.DefaultConfig())
}

// Get the tunnel's status
func (t *TCPClient) Status() i2ptunnel.I2PTunnelStatus {
	return t.I2PTunnelStatus
}

// Stop the tunnel. Safe to call multiple times.
// Closes the Garlic (I2P SAM session) to release network resources.
func (t *TCPClient) Stop() error {
	t.stopOnce.Do(func() {
		close(t.done)
		if t.Garlic != nil {
			t.Garlic.Close()
		}
	})
	t.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped
	return nil
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (t *TCPClient) Target() string {
	return t.I2PAddr.Base32()
}

// Get the tunnel's type
func (t *TCPClient) Type() string {
	return t.TunnelConfig.Type
}

// Get the tunnel's ID
func (t *TCPClient) ID() string {
	return i2ptunnel.Clean(t.Name())
}

// Get the tunnel's options
func (t *TCPClient) Options() map[string]string {
	// Return basic configuration options as a map
	options := make(map[string]string)
	options["name"] = t.TunnelConfig.Name
	options["type"] = t.TunnelConfig.Type
	options["interface"] = t.TunnelConfig.Interface
	options["port"] = strconv.Itoa(t.TunnelConfig.Port)
	if t.I2PAddr != nil {
		options["target"] = t.I2PAddr.Base32()
	}
	return options
}

// Set the tunnel's options
func (t *TCPClient) SetOptions(opts map[string]string) error {
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
	if target, ok := opts["target"]; ok {
		if err := validate.I2PAddress(target); err != nil {
			return err
		}
		addr, err := i2pkeys.Lookup(target)
		if err != nil {
			return fmt.Errorf("invalid target address: %w", err)
		}
		t.I2PAddr = addr
	}
	return nil
}

// LoadConfig loads tunnel configuration from a file and updates the tunnel settings.
// The tunnel must be stopped before calling LoadConfig to prevent inconsistent state.
// Supported formats: .properties, .ini, .yaml/.yml
//
// Why: Production deployments need to reload configuration without recreating tunnel objects.
// This enables configuration management tools and web UIs to persist changes.
//
// Design: Uses the go-i2ptunnel-config library to parse config files in multiple formats,
// then updates only the mutable fields. SAM connection is preserved to maintain tunnel identity.
// The Garlic (I2P connection) is NOT reloaded - it maintains the existing keys and SAM session.
func (t *TCPClient) LoadConfig(path string) error {
	// Prevent config changes while tunnel is running to avoid race conditions
	if t.I2PTunnelStatus == i2ptunnel.I2PTunnelStatusRunning ||
		t.I2PTunnelStatus == i2ptunnel.I2PTunnelStatusStarting {
		return fmt.Errorf("cannot load config while tunnel is %s - stop tunnel first", t.I2PTunnelStatus)
	}

	// Parse config file using the converter library
	// This handles format detection and validation for .properties, .ini, .yaml
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
	if newConfig.Type != "tcpclient" {
		return fmt.Errorf("config file contains %s tunnel, expected tcpclient", newConfig.Type)
	}

	// Validate target address before applying changes
	addr, err := i2pkeys.Lookup(newConfig.Target)
	if err != nil {
		return fmt.Errorf("invalid target address in config: %w", err)
	}

	// Update mutable configuration fields
	// The Garlic connection (SAM) is preserved to maintain tunnel identity and keys
	t.TunnelConfig = *newConfig
	t.I2PAddr = addr

	return nil
}
