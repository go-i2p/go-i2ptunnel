package i2ptunnel

import (
	"fmt"
	"net"
	"sync"
	"time"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
	limitedlistener "github.com/go-i2p/go-limit"
)

// TunnelBase holds the common fields and methods shared by all I2P tunnel
// implementations. Embed TunnelBase in a tunnel struct to eliminate the
// duplicated Options/SetOptions/Status/Name/Type/Error boilerplate that was
// previously copied across all 12 tunnel types.
//
// Fields that vary per tunnel type (net.Addr target, lifecycle channels,
// protocol-specific inspectors) are intentionally left in the outer struct.
type TunnelBase struct {
	// TunnelConfig carries the SAM connection, name, type, interface and port.
	i2pconv.TunnelConfig
	// LimitedConfig holds the MaxConns and RateLimit knobs shared by 10 of the
	// 12 tunnel types. The two that use custom rate-limiting (TCPClient,
	// HTTPClient) embed TunnelBase but override Options/SetOptions entirely.
	limitedlistener.LimitedConfig
	// I2PTunnelStatus is the current lifecycle state of the tunnel.
	I2PTunnelStatus
	// statusMu serialises concurrent reads/writes to I2PTunnelStatus.
	statusMu sync.RWMutex
	// ErrorTracker provides a bounded, thread-safe error history.
	ErrorTracker
	// Metrics is the live operational tracker injected by the webui controller
	// after construction. May be nil; all accesses must be nil-guarded.
	Metrics *metrics.TunnelMetrics
}

// Name returns the tunnel's human-readable name. Implements I2PTunnel.
func (b *TunnelBase) Name() string { return b.TunnelConfig.Name }

// ID returns a URL-safe identifier derived from the tunnel name.
// Implements I2PTunnel.
func (b *TunnelBase) ID() string { return Clean(b.Name()) }

// Type returns the tunnel type string (e.g. "tcpserver"). Implements I2PTunnel.
func (b *TunnelBase) Type() string { return b.TunnelConfig.Type }

// Error returns the most recently recorded error, or nil. Implements I2PTunnel.
func (b *TunnelBase) Error() error { return b.ErrorTracker.Last() }

// Status returns the current tunnel lifecycle state, safe for concurrent use.
// Implements I2PTunnel.
func (b *TunnelBase) Status() I2PTunnelStatus {
	b.statusMu.RLock()
	defer b.statusMu.RUnlock()
	return b.I2PTunnelStatus
}

// SetTunnelMetrics injects a live metrics tracker. Implements metrics.MetricsBearer.
func (b *TunnelBase) SetTunnelMetrics(m *metrics.TunnelMetrics) {
	b.Metrics = m
}

// SetStatus transitions the tunnel to status s, safe for concurrent use.
// Called from Start() and Stop() in each tunnel implementation.
func (b *TunnelBase) SetStatus(s I2PTunnelStatus) {
	b.statusMu.Lock()
	b.I2PTunnelStatus = s
	b.statusMu.Unlock()
}

// RecordError appends err to the bounded error history and, when metrics are
// configured, increments the error counter.
func (b *TunnelBase) RecordError(err error) {
	b.ErrorTracker.Record(b, err)
	if b.Metrics != nil {
		b.Metrics.RecordError()
	}
}

// Options returns the common configuration keys shared by all tunnel types:
// name, type, interface, port, all i2cp.* options, maxconns, and ratelimit.
// Tunnel types with additional keys (e.g. "target") should call this method
// first and then append their own entries to the returned map.
func (b *TunnelBase) Options() map[string]string {
	opts := BuildCommonOptions(b.TunnelConfig)
	AddRateLimitOptions(opts, b.LimitedConfig.MaxConns, b.LimitedConfig.RateLimit)
	return opts
}

// SetOptions applies the common option keys (name, interface, port, i2cp.*,
// maxconns, ratelimit) from opts. Tunnel types with additional keys should call
// this method first, then handle their own keys.
func (b *TunnelBase) SetOptions(opts map[string]string) error {
	if err := ApplyCommonOptions(opts, &b.TunnelConfig); err != nil {
		return err
	}
	return ApplyRateLimitOptions(opts, &b.LimitedConfig.MaxConns, &b.LimitedConfig.RateLimit)
}

// HandleAcceptError is the shared accept-loop error handler used by all tunnel
// types. It checks for done-channel cancellation, increments consecutiveErrors
// for non-rate-limit errors, records the error, and returns (false, fatal) once
// maxErrors is reached. Returns (true, nil) to continue the accept loop, or
// (false, nil) on shutdown. Rate-limit errors (ErrMaxConnsReached,
// ErrRateLimitExceeded) are recorded as metrics hits but do not count toward
// the consecutive-error limit.
func (b *TunnelBase) HandleAcceptError(err error, consecutiveErrors *int, maxErrors int, done <-chan struct{}) (cont bool, fatal error) {
	isRateLimit := err == limitedlistener.ErrMaxConnsReached || err == limitedlistener.ErrRateLimitExceeded
	if isRateLimit && b.Metrics != nil {
		b.Metrics.RecordRateLimitHit()
	}
	select {
	case <-done:
		return false, nil
	default:
	}
	if !isRateLimit {
		*consecutiveErrors++
		b.RecordError(fmt.Errorf("accept error (%d consecutive): %w", *consecutiveErrors, err))
		if *consecutiveErrors >= maxErrors {
			b.SetStatus(I2PTunnelStatusFailed)
			return false, fmt.Errorf("listener failed after %d consecutive accept errors", *consecutiveErrors)
		}
	}
	time.Sleep(50 * time.Millisecond)
	return true, nil
}

// RunAcceptDispatch runs a for-select accept loop on l, dispatching each
// accepted connection via handleConn in a goroutine. maxErrors is the
// consecutive-error limit. done is the shutdown channel.
func (b *TunnelBase) RunAcceptDispatch(l net.Listener, maxErrors int, done <-chan struct{}, handleConn func(net.Conn)) error {
	consecutiveErrors := 0
	for {
		select {
		case <-done:
			return nil
		default:
			con, err := l.Accept()
			if err != nil {
				if cont, fatal := b.HandleAcceptError(err, &consecutiveErrors, maxErrors, done); !cont {
					return fatal
				}
				continue
			}
			consecutiveErrors = 0
			go handleConn(con)
		}
	}
}
