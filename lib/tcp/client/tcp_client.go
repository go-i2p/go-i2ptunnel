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
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
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
	done := t.done
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
	return t.runAcceptLoop(listener, done)
}

// runAcceptLoop runs the main TCP accept loop until done is closed or a fatal error occurs.
func (t *TCPClient) runAcceptLoop(listener net.Listener, done <-chan struct{}) error {
	consecutiveAcceptErrors := 0
	for {
		select {
		case <-done:
			return nil
		default:
			con, err := listener.Accept()
			if err != nil {
				cont, fatal := t.handleAcceptError(err, &consecutiveAcceptErrors, done)
				if fatal != nil {
					return fatal
				}
				if !cont {
					return nil
				}
				continue
			}
			consecutiveAcceptErrors = 0
			t.spawnConnection(con)
		}
	}
}

// handleAcceptError processes a listener.Accept() error and returns (cont, fatal).
// cont=false means the accept loop should exit cleanly; fatal!=nil means exit with error.
func (t *TCPClient) handleAcceptError(err error, consecutiveErrors *int, done <-chan struct{}) (cont bool, fatal error) {
	return t.TunnelBase.HandleAcceptError(err, consecutiveErrors, maxConsecutiveAcceptErrors, done)
}

// spawnConnection acquires a semaphore slot (if any) and dispatches handleConnection in a goroutine.
func (t *TCPClient) spawnConnection(con net.Conn) {
	sem := t.snapshotConnSem()
	if sem != nil {
		select {
		case sem <- struct{}{}:
		default:
			t.RecordError(fmt.Errorf("connection rejected: at capacity (%d max concurrent)", cap(sem)))
			con.Close()
			return
		}
	}
	go func() {
		if sem != nil {
			defer func() { <-sem }()
		}
		t.handleConnection(con)
	}()
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
	i2pConn, err := t.dialI2P()
	if err != nil {
		t.RecordError(err)
		if t.Metrics != nil {
			t.Metrics.RecordConnectionFailed()
		}
		return
	}
	defer i2pConn.Close()
	t.forwardStream(filtered, i2pConn)
}

// dialI2P dials the configured I2P target address with an optional timeout.
func (t *TCPClient) dialI2P() (net.Conn, error) {
	target := t.Target()
	if target == "" {
		return nil, fmt.Errorf("handleConnection: no target I2P address configured")
	}
	dialCtx, dialCancel := t.dialContext()
	defer dialCancel()
	return t.Garlic.DialContext(dialCtx, "tcp", target)
}

// forwardStream forwards between filtered local conn and an I2P connection,
// cancelling on tunnel stop.
func (t *TCPClient) forwardStream(filtered, i2pConn net.Conn) {
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
// tcpClientOpts holds the validated (but not yet applied) option values from SetOptions Phase 1.
type tcpClientOpts struct {
	newName                             string
	newIface                            string
	newPort                             int
	newAddr                             *i2pkeys.I2PAddr
	i2cpOpts                            map[string]interface{}
	newMaxConns                         int
	newDialTimeout                      time.Duration
	setName, setIface, setPort, setAddr bool
	setMaxConns, setDialTimeout         bool
}

// validateTCPClientOptions performs Phase 1 of SetOptions: validate all values
// before acquiring any lock. Returns a populated tcpClientOpts or an error.
func validateTCPClientOptions(opts map[string]string) (tcpClientOpts, error) {
	var o tcpClientOpts
	if err := validateCommonFields(opts, &o); err != nil {
		return o, err
	}
	if err := validateTCPSpecificFields(opts, &o); err != nil {
		return o, err
	}
	o.i2cpOpts = i2ptunnel.ExtractI2CPOptions(opts)
	return o, nil
}

// validateCommonFields validates name, interface, port, and target options.
func validateCommonFields(opts map[string]string, o *tcpClientOpts) error {
	if v, ok := opts["name"]; ok {
		if err := validate.RequiredString("name", v); err != nil {
			return err
		}
		o.newName, o.setName = v, true
	}
	if v, ok := opts["interface"]; ok {
		if err := validate.Interface(v); err != nil {
			return err
		}
		o.newIface, o.setIface = v, true
	}
	if v, ok := opts["port"]; ok {
		port, err := validate.PortString(v)
		if err != nil {
			return err
		}
		o.newPort, o.setPort = port, true
	}
	if v, ok := opts["target"]; ok {
		return validateTarget(v, o)
	}
	return nil
}

// validateTarget validates an I2P target address and resolves it to a destination.
func validateTarget(v string, o *tcpClientOpts) error {
	if err := validate.I2PAddress(v); err != nil {
		return err
	}
	addr, err := i2pkeys.Lookup(v)
	if err != nil {
		return fmt.Errorf("invalid target address: %w", err)
	}
	o.newAddr, o.setAddr = addr, true
	return nil
}

// validateTCPSpecificFields validates maxconns and dialtimeout options.
func validateTCPSpecificFields(opts map[string]string, o *tcpClientOpts) error {
	if v, ok := opts["maxconns"]; ok {
		mc, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid maxconns %q: must be a non-negative integer", v)
		}
		if err := validate.MaxConnections(mc); err != nil {
			return err
		}
		o.newMaxConns, o.setMaxConns = mc, true
	}
	if v, ok := opts["dialtimeout"]; ok {
		d, err := time.ParseDuration(v)
		if err != nil || d < 0 {
			return fmt.Errorf("invalid dialtimeout %q: must be a non-negative duration (e.g. 30s, 1m30s)", v)
		}
		o.newDialTimeout, o.setDialTimeout = d, true
	}
	return nil
}

// Design: All values are validated first without holding any lock, since i2pkeys.Lookup
// may perform network I/O. Only after all validation passes are the writes applied under
// lifeMu to prevent a data race with Start() reading Interface/Port to bind the listener.
func (t *TCPClient) SetOptions(opts map[string]string) error {
	o, err := validateTCPClientOptions(opts)
	if err != nil {
		return err
	}
	t.lifeMu.Lock()
	defer t.lifeMu.Unlock()
	t.applyTCPClientOpts(&o)
	return nil
}

// applyTCPClientOpts applies validated option values under the caller's lifeMu.
func (t *TCPClient) applyTCPClientOpts(o *tcpClientOpts) {
	t.applyTCPConfigFields(o)
	t.applyTCPConnOpts(o)
}

// applyTCPConfigFields applies name/interface/port/address and I2CP map changes.
func (t *TCPClient) applyTCPConfigFields(o *tcpClientOpts) {
	if o.setName {
		t.TunnelConfig.Name = o.newName
	}
	if o.setIface {
		t.TunnelConfig.Interface = o.newIface
	}
	if o.setPort {
		t.TunnelConfig.Port = o.newPort
	}
	if o.setAddr {
		t.I2PAddr = o.newAddr
	}
	if o.i2cpOpts != nil {
		if t.TunnelConfig.I2CP == nil {
			t.TunnelConfig.I2CP = make(map[string]interface{})
		}
		for k, v := range o.i2cpOpts {
			t.TunnelConfig.I2CP[k] = v
		}
	}
}

// applyTCPConnOpts applies connection semaphore and dial timeout changes.
func (t *TCPClient) applyTCPConnOpts(o *tcpClientOpts) {
	if o.setMaxConns {
		if o.newMaxConns > 0 {
			t.connSem = make(chan struct{}, o.newMaxConns)
		} else {
			t.connSem = nil
		}
	}
	if o.setDialTimeout {
		t.dialTimeout = o.newDialTimeout
	}
}

// LoadConfig loads tunnel configuration from a file and updates the tunnel settings.
// The tunnel must be stopped before calling LoadConfig to prevent inconsistent state.
// All I/O (file reads, address lookups) occurs before acquiring lifeMu to avoid
// priority inversions. The status check and struct update are applied under lifeMu.
func (t *TCPClient) LoadConfig(path string) error {
	if err := i2ptunnel.CheckTunnelStopped(t.Status()); err != nil {
		return err
	}
	newConfig, addr, err := loadTCPClientConfig(path)
	if err != nil {
		return err
	}
	t.lifeMu.Lock()
	defer t.lifeMu.Unlock()
	if err := i2ptunnel.CheckTunnelStopped(t.Status()); err != nil {
		return err
	}
	if len(newConfig.I2CP) == 0 && len(t.TunnelConfig.I2CP) > 0 {
		newConfig.I2CP = t.TunnelConfig.I2CP
	}
	t.TunnelConfig = *newConfig
	t.I2PAddr = addr
	return nil
}

// loadTCPClientConfig parses and validates the config file outside any lock.
func loadTCPClientConfig(path string) (*i2pconv.TunnelConfig, *i2pkeys.I2PAddr, error) {
	newConfig, err := i2ptunnel.ParseConfigFile(path)
	if err != nil {
		return nil, nil, err
	}
	if newConfig.Type != "tcpclient" {
		return nil, nil, fmt.Errorf("config file contains %s tunnel, expected tcpclient", newConfig.Type)
	}
	addr, err := i2pkeys.Lookup(newConfig.Target)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid target address in config: %w", err)
	}
	return newConfig, addr, nil
}
