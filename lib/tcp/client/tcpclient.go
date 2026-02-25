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
	// Mutex protecting lifecycle fields (done, stopOnce, listener) during Start/Stop transitions.
	// Prevents the race where Start() resets stopOnce while Stop() is calling stopOnce.Do().
	lifeMu sync.Mutex
	// Mutex protecting the Errors slice from concurrent access
	errMu sync.Mutex
	// Mutex protecting the I2PTunnelStatus field from concurrent read/write access
	statusMu sync.RWMutex
	// dialTimeout limits how long handleConnection waits to establish an I2P stream.
	// Zero means no timeout. Default: defaultDialTimeout. Modified only under lifeMu.
	dialTimeout time.Duration
	// connSem is a counting semaphore for limiting concurrent in-flight connections.
	// nil means unlimited (maxConns == 0). Modified only under lifeMu.
	// Each accept iteration snapshots the channel reference (snapshotConnSem) so that
	// a concurrent SetOptions rebuild does not cause mismatched acquire/release pairs.
	connSem chan struct{}

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}

// maxErrors is the maximum number of errors retained in memory.
// Only the most recent errors are kept to prevent unbounded memory growth.
const maxErrors = 100

// defaultDialTimeout is the maximum time allowed to establish an I2P stream to the target.
// I2P connections traverse multiple encrypted hops and can be slower than clearnet;
// 30 seconds is a conservative bound that avoids permanently blocked goroutines while
// accommodating normal I2P routing delays.
const defaultDialTimeout = 30 * time.Second

func (t *TCPClient) recordError(err error) {
	t.errMu.Lock()
	t.Errors = append(t.Errors, i2ptunnel.NewError(t, err))
	if len(t.Errors) > maxErrors {
		// Discard oldest errors to bound memory usage.
		t.Errors = append([]i2ptunnel.I2PTunnelError(nil), t.Errors[len(t.Errors)-maxErrors:]...)
	}
	t.errMu.Unlock()
}

// setStatus updates the tunnel status with proper synchronization.
func (t *TCPClient) setStatus(s i2ptunnel.I2PTunnelStatus) {
	t.statusMu.Lock()
	t.I2PTunnelStatus = s
	t.statusMu.Unlock()
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
	t.lifeMu.Lock()
	t.done = make(chan struct{})
	t.stopOnce = sync.Once{}
	t.setStatus(i2ptunnel.I2PTunnelStatusStarting)
	listener, err := net.Listen("tcp", net.JoinHostPort(t.Interface, strconv.Itoa(t.Port)))
	if err != nil {
		t.lifeMu.Unlock()
		return err
	}
	t.listener = listener
	t.lifeMu.Unlock()
	defer listener.Close()
	defer t.Stop()
	t.setStatus(i2ptunnel.I2PTunnelStatusRunning)
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
			// Snapshot the semaphore before spawning: each goroutine holds a reference
			// to the channel it acquired from, so a SetOptions rebuild is safe.
			sem := t.snapshotConnSem()
			if sem != nil {
				select {
				case sem <- struct{}{}:
					// Acquired a slot; the goroutine releases it on exit.
				default:
					// At capacity — reject immediately so the local client gets a fast error.
					t.recordError(fmt.Errorf("connection rejected: at capacity (%d max concurrent)", cap(sem)))
					con.Close()
					continue
				}
			}
			go func() {
				if sem != nil {
					defer func() { <-sem }()
				}
				t.handleConnection(con)
			}()
		}
	}
}

// handleConnection forwards a single local connection over its own I2P stream.
// Both connections are closed when forwarding completes.
// Forwarding errors are recorded so operators and the web UI can observe them.
func (t *TCPClient) handleConnection(con net.Conn) {
	defer con.Close()
	target := t.Target()
	if target == "" {
		t.recordError(fmt.Errorf("handleConnection: no target I2P address configured"))
		return
	}
	dialCtx, dialCancel := t.dialContext()
	defer dialCancel()
	i2pConn, err := t.Garlic.DialContext(dialCtx, "tcp", target)
	if err != nil {
		t.recordError(err)
		return
	}
	defer i2pConn.Close()
	if err := stream.Forward(context.Background(), con, i2pConn, config.DefaultConfig()); err != nil {
		t.recordError(err)
	}
}

// dialContext returns a context for the outgoing I2P Dial, applying dialTimeout when set.
// A zero timeout means no deadline (context.Background is returned).
// The returned CancelFunc must always be called (via defer) to release timer resources.
func (t *TCPClient) dialContext() (context.Context, context.CancelFunc) {
	t.lifeMu.Lock()
	timeout := t.dialTimeout
	t.lifeMu.Unlock()
	if timeout > 0 {
		return context.WithTimeout(context.Background(), timeout)
	}
	return context.Background(), func() {}
}

// snapshotConnSem returns the current connSem under the lifecycle lock.
// Callers must use the returned channel directly — not re-read t.connSem — so that
// a concurrent SetOptions rebuild does not cause mismatched acquire/release pairs.
func (t *TCPClient) snapshotConnSem() chan struct{} {
	t.lifeMu.Lock()
	defer t.lifeMu.Unlock()
	return t.connSem
}

// Get the tunnel's status
func (t *TCPClient) Status() i2ptunnel.I2PTunnelStatus {
	t.statusMu.RLock()
	defer t.statusMu.RUnlock()
	return t.I2PTunnelStatus
}

// Stop the tunnel. Safe to call multiple times.
// Closes the Garlic (I2P SAM session) to release network resources.
func (t *TCPClient) Stop() error {
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
	})
	return nil
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP.
// Returns empty string when no target address has been set, matching the nil-safe
// behaviour of Address() and avoiding a panic in handleConnection goroutines.
func (t *TCPClient) Target() string {
	if t.I2PAddr == nil {
		return ""
	}
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
	t.lifeMu.Lock()
	options["maxconns"] = strconv.Itoa(cap(t.connSem)) // 0 means unlimited
	options["dialtimeout"] = t.dialTimeout.String()
	t.lifeMu.Unlock()
	i2ptunnel.MergeI2CPOptions(t.TunnelConfig.I2CP, options)
	return options
}

// Set the tunnel's options
//
// Design: All values are validated first without holding any lock, since i2pkeys.Lookup
// may perform network I/O. Only after all validation passes are the writes applied under
// lifeMu to prevent a data race with Start() reading Interface/Port to bind the listener.
func (t *TCPClient) SetOptions(opts map[string]string) error {
	// Phase 1 — validate everything before acquiring the mutex.
	var (
		newName                             string
		newIface                            string
		newPort                             int
		newAddr                             *i2pkeys.I2PAddr
		i2cpOpts                            map[string]interface{}
		setName, setIface, setPort, setAddr bool
	)
	if v, ok := opts["name"]; ok {
		if err := validate.RequiredString("name", v); err != nil {
			return err
		}
		newName, setName = v, true
	}
	if v, ok := opts["interface"]; ok {
		if err := validate.Interface(v); err != nil {
			return err
		}
		newIface, setIface = v, true
	}
	if v, ok := opts["port"]; ok {
		port, err := validate.PortString(v)
		if err != nil {
			return err
		}
		newPort, setPort = port, true
	}
	if v, ok := opts["target"]; ok {
		if err := validate.I2PAddress(v); err != nil {
			return err
		}
		addr, err := i2pkeys.Lookup(v)
		if err != nil {
			return fmt.Errorf("invalid target address: %w", err)
		}
		newAddr, setAddr = addr, true
	}
	i2cpOpts = i2ptunnel.ExtractI2CPOptions(opts)
	var (
		newMaxConns    int
		newDialTimeout time.Duration
		setMaxConns    bool
		setDialTimeout bool
	)
	if v, ok := opts["maxconns"]; ok {
		mc, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid maxconns %q: must be a non-negative integer", v)
		}
		if err := validate.MaxConnections(mc); err != nil {
			return err
		}
		newMaxConns, setMaxConns = mc, true
	}
	if v, ok := opts["dialtimeout"]; ok {
		d, err := time.ParseDuration(v)
		if err != nil || d < 0 {
			return fmt.Errorf("invalid dialtimeout %q: must be a non-negative duration (e.g. 30s, 1m30s)", v)
		}
		newDialTimeout, setDialTimeout = d, true
	}

	// Phase 2 — apply validated values under lifeMu to prevent races with Start().
	t.lifeMu.Lock()
	defer t.lifeMu.Unlock()
	if setName {
		t.TunnelConfig.Name = newName
	}
	if setIface {
		t.TunnelConfig.Interface = newIface
	}
	if setPort {
		t.TunnelConfig.Port = newPort
	}
	if setAddr {
		t.I2PAddr = newAddr
	}
	if i2cpOpts != nil {
		if t.TunnelConfig.I2CP == nil {
			t.TunnelConfig.I2CP = make(map[string]interface{})
		}
		for k, v := range i2cpOpts {
			t.TunnelConfig.I2CP[k] = v
		}
	}
	if setMaxConns {
		if newMaxConns > 0 {
			t.connSem = make(chan struct{}, newMaxConns)
		} else {
			t.connSem = nil // 0 means unlimited
		}
	}
	if setDialTimeout {
		t.dialTimeout = newDialTimeout
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
	// Prevent config changes while tunnel is running to avoid race conditions.
	// Use the value returned by the mutex-protected Status() call rather than accessing
	// t.I2PTunnelStatus directly, which would be a data race.
	status := t.Status()
	if status == i2ptunnel.I2PTunnelStatusRunning ||
		status == i2ptunnel.I2PTunnelStatusStarting {
		return fmt.Errorf("cannot load config while tunnel is %s - stop tunnel first", status)
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
