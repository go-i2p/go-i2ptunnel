// Package tcpclient implements a TCP client tunnel that listens on a local
// port and forwards connections to a fixed I2P destination.
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
	"strconv"
	"sync"
	"time"

	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/stream"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
	"github.com/go-i2p/i2pkeys"
	"github.com/go-i2p/onramp"
)

var implementTCPClient i2ptunnel.I2PTunnel = &TCPClient{}

// TCPClient is a TCP client tunnel that listens on a local TCP port and forwards
// each accepted connection to a fixed I2P destination. It enforces a configurable
// concurrency limit (connSem) and per-connection dial timeout.
type TCPClient struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// TunnelBase provides Name, ID, Type, Status, Error, SetTunnelMetrics,
	// SetStatus, RecordError, and the common Options/SetOptions keys.
	i2ptunnel.TunnelBase
	// The remote I2P destination target
	*i2pkeys.I2PAddr
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
	// forwarded connection. Nil means no filtering (passthrough).
	*i2ptunnel.TCPFilterConfig
	// dialTimeout limits how long handleConnection waits to establish an I2P stream.
	// Zero means no timeout. Default: defaultDialTimeout. Modified only under lifeMu.
	dialTimeout time.Duration
	// connSem is a counting semaphore for limiting concurrent in-flight connections.
	// nil means unlimited (maxConns == 0). Modified only under lifeMu.
	// Each accept iteration snapshots the channel reference (snapshotConnSem) so that
	// a concurrent SetOptions rebuild does not cause mismatched acquire/release pairs.
	connSem chan struct{}
}

// maxConsecutiveAcceptErrors is the number of consecutive Accept() failures before
// the tunnel transitions to I2PTunnelStatusFailed. This lets operators monitoring
// Status() distinguish a degraded listener from a healthy one.
const maxConsecutiveAcceptErrors = 10

// defaultDialTimeout is the maximum time allowed to establish an I2P stream to the target.
// I2P connections traverse multiple encrypted hops and can be slower than clearnet;
// 30 seconds is a conservative bound that avoids permanently blocked goroutines while
// accommodating normal I2P routing delays.
const defaultDialTimeout = 30 * time.Second

// Get the tunnel's I2P address
func (t *TCPClient) Address() string {
	// Return the target I2P address for client tunnels
	if t.I2PAddr != nil {
		return t.I2PAddr.Base32()
	}
	return ""
}

// ErrorHistory returns a snapshot of all recorded errors, delegating to ErrorTracker.
func (t *TCPClient) ErrorHistory() []i2ptunnel.I2PTunnelError {
	return t.ErrorTracker.All()
}

// Get the tunnel's local host:port
func (t *TCPClient) LocalAddress() (string, error) {
	addr := net.JoinHostPort(t.TunnelConfig.Interface, strconv.Itoa(t.TunnelConfig.Port))
	return addr, nil
}

// Start the tunnel.
// Each accepted local connection gets its own I2P stream to the target destination.
// Connections are handled concurrently in separate goroutines.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (t *TCPClient) Start() error {
	t.lifeMu.Lock()
	t.done = make(chan struct{})
	t.stopOnce = sync.Once{}
	done := t.done // capture local ref before unlock to avoid data race with restart
	t.SetStatus(i2ptunnel.I2PTunnelStatusStarting)
	listener, err := net.Listen("tcp", net.JoinHostPort(t.Interface, strconv.Itoa(t.Port)))
	if err != nil {
		t.lifeMu.Unlock()
		return err
	}
	t.listener = listener
	t.lifeMu.Unlock()
	defer listener.Close()
	defer t.Stop()
	t.SetStatus(i2ptunnel.I2PTunnelStatusRunning)
	if t.Metrics != nil {
		t.Metrics.RecordStart()
	}
	consecutiveAcceptErrors := 0
	for {
		select {
		case <-done:
			return nil
		default:
			con, err := listener.Accept()
			if err != nil {
				// Check if tunnel is shutting down
				select {
				case <-done:
					return nil
				default:
				}
				// Record the error and transition to Failed after too many consecutive
				// errors so operators see I2PTunnelStatusFailed rather than a silently
				// looping tunnel that claims to be running.
				consecutiveAcceptErrors++
				t.RecordError(fmt.Errorf("listener.Accept error (%d consecutive): %w", consecutiveAcceptErrors, err))
				if consecutiveAcceptErrors >= maxConsecutiveAcceptErrors {
					t.SetStatus(i2ptunnel.I2PTunnelStatusFailed)
					return fmt.Errorf("listener failed after %d consecutive accept errors", consecutiveAcceptErrors)
				}
				// Backoff to prevent CPU-burning tight loop on persistent errors
				time.Sleep(50 * time.Millisecond)
				continue
			}
			consecutiveAcceptErrors = 0 // reset on successful accept
			// Snapshot the semaphore before spawning: each goroutine holds a reference
			// to the channel it acquired from, so a SetOptions rebuild is safe.
			sem := t.snapshotConnSem()
			if sem != nil {
				select {
				case sem <- struct{}{}:
					// Acquired a slot; the goroutine releases it on exit.
				default:
					// At capacity — reject immediately so the local client gets a fast error.
					t.RecordError(fmt.Errorf("connection rejected: at capacity (%d max concurrent)", cap(sem)))
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
	target := t.Target()
	if target == "" {
		t.RecordError(fmt.Errorf("handleConnection: no target I2P address configured"))
		if t.Metrics != nil {
			t.Metrics.RecordConnectionFailed()
		}
		return
	}
	dialCtx, dialCancel := t.dialContext()
	defer dialCancel()
	i2pConn, err := t.Garlic.DialContext(dialCtx, "tcp", target)
	if err != nil {
		t.RecordError(err)
		if t.Metrics != nil {
			t.Metrics.RecordConnectionFailed()
		}
		return
	}
	defer i2pConn.Close()
	// Derive a context from the tunnel's done channel so that forwarding is
	// cancelled when the tunnel stops, preventing indefinitely stalled goroutines.
	fwdCtx, fwdCancel := context.WithCancel(context.Background())
	defer fwdCancel()
	go func() {
		select {
		case <-t.done:
			fwdCancel()
		case <-fwdCtx.Done():
		}
	}()
	if err := stream.Forward(fwdCtx, filtered, i2pConn, config.DefaultConfig()); err != nil {
		t.RecordError(err)
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
		t.SetStatus(i2ptunnel.I2PTunnelStatusStopped)
		if t.Metrics != nil {
			t.Metrics.RecordStop()
		}
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

// Get the tunnel's options
func (t *TCPClient) Options() map[string]string {
	options := i2ptunnel.BuildCommonOptions(t.TunnelConfig)
	if t.I2PAddr != nil {
		options["target"] = t.I2PAddr.Base32()
	}
	t.lifeMu.Lock()
	options["maxconns"] = strconv.Itoa(cap(t.connSem)) // 0 means unlimited
	options["dialtimeout"] = t.dialTimeout.String()
	t.lifeMu.Unlock()
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
		// Note: replacing connSem orphans the old channel. In-flight goroutines
		// hold snapshots of the old channel (via snapshotConnSem at line 193),
		// so they will correctly release their slots to the old channel when done.
		// During the transition window, cap(t.connSem) may not reflect the true
		// in-flight count. This is acceptable because the snapshot pattern
		// guarantees no mismatched acquire/release, and the old channel is GC'd
		// once all in-flight goroutines complete.
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
// All I/O (file reads, address lookups) occurs before acquiring lifeMu to avoid
// priority inversions. The status check and struct update are applied under lifeMu.
func (t *TCPClient) LoadConfig(path string) error {
	// Quick pre-check before I/O — authoritative check is repeated under lifeMu below.
	if err := i2ptunnel.CheckTunnelStopped(t.Status()); err != nil {
		return err
	}

	// Phase 1 — all I/O outside any lock.
	newConfig, err := i2ptunnel.ParseConfigFile(path)
	if err != nil {
		return err
	}
	if newConfig.Type != "tcpclient" {
		return fmt.Errorf("config file contains %s tunnel, expected tcpclient", newConfig.Type)
	}
	addr, err := i2pkeys.Lookup(newConfig.Target)
	if err != nil {
		return fmt.Errorf("invalid target address in config: %w", err)
	}

	// Phase 2 — atomically check status and apply under lifeMu.
	t.lifeMu.Lock()
	defer t.lifeMu.Unlock()
	if err := i2ptunnel.CheckTunnelStopped(t.Status()); err != nil {
		return err
	}
	// Preserve I2CP options when the config file omits the i2cp section.
	if len(newConfig.I2CP) == 0 && len(t.TunnelConfig.I2CP) > 0 {
		newConfig.I2CP = t.TunnelConfig.I2CP
	}
	t.TunnelConfig = *newConfig
	t.I2PAddr = addr
	return nil
}
