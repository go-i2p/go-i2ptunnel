package metrics

import (
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// TunnelMetrics tracks operational metrics for a single I2P tunnel.
// All fields use atomic operations and are safe for concurrent access.
type TunnelMetrics struct {
	// Labels identify the tunnel.
	Name string
	ID   string
	Type string

	// Counters track cumulative totals.
	ConnectionsAccepted atomic.Int64
	ConnectionsFailed   atomic.Int64
	BytesIn             atomic.Int64
	BytesOut            atomic.Int64
	ErrorCount          atomic.Int64
	RateLimitHits       atomic.Int64
	FilterBlocks        atomic.Int64

	// ActiveConnections is a gauge tracking currently open connections.
	ActiveConnections atomic.Int64

	// startedAt stores a Unix nanosecond timestamp set when the tunnel
	// enters "running" state. Zero means the tunnel is not running.
	startedAt atomic.Int64
}

// RecordStart marks the tunnel as started at the current time.
func (m *TunnelMetrics) RecordStart() {
	m.startedAt.Store(time.Now().UnixNano())
}

// RecordStop clears the start timestamp.
func (m *TunnelMetrics) RecordStop() {
	m.startedAt.Store(0)
}

// UptimeSeconds returns seconds since the tunnel started.
// Returns 0 if the tunnel is not running.
func (m *TunnelMetrics) UptimeSeconds() float64 {
	started := m.startedAt.Load()
	if started == 0 {
		return 0
	}
	return time.Since(time.Unix(0, started)).Seconds()
}

// RecordConnection increments accepted connections and active count.
func (m *TunnelMetrics) RecordConnection() {
	m.ConnectionsAccepted.Add(1)
	m.ActiveConnections.Add(1)
}

// RecordDisconnection decrements the active connection count.
func (m *TunnelMetrics) RecordDisconnection() {
	m.ActiveConnections.Add(-1)
}

// RecordError increments the error counter.
func (m *TunnelMetrics) RecordError() {
	m.ErrorCount.Add(1)
}

// RecordConnectionFailed increments the failed connections counter.
func (m *TunnelMetrics) RecordConnectionFailed() {
	m.ConnectionsFailed.Add(1)
}

// RecordBytesIn adds n to the inbound byte counter.
func (m *TunnelMetrics) RecordBytesIn(n int64) {
	m.BytesIn.Add(n)
}

// RecordBytesOut adds n to the outbound byte counter.
func (m *TunnelMetrics) RecordBytesOut(n int64) {
	m.BytesOut.Add(n)
}

// RecordRateLimitHit increments the rate limit rejection counter.
func (m *TunnelMetrics) RecordRateLimitHit() {
	m.RateLimitHits.Add(1)
}

// RecordFilterBlock increments the filter block counter.
func (m *TunnelMetrics) RecordFilterBlock() {
	m.FilterBlocks.Add(1)
}

// Snapshot returns a point-in-time copy of all metric values.
func (m *TunnelMetrics) Snapshot() MetricSnapshot {
	return MetricSnapshot{
		Name:                m.Name,
		ID:                  m.ID,
		Type:                m.Type,
		ConnectionsAccepted: m.ConnectionsAccepted.Load(),
		ConnectionsFailed:   m.ConnectionsFailed.Load(),
		ActiveConnections:   m.ActiveConnections.Load(),
		BytesIn:             m.BytesIn.Load(),
		BytesOut:            m.BytesOut.Load(),
		ErrorCount:          m.ErrorCount.Load(),
		RateLimitHits:       m.RateLimitHits.Load(),
		FilterBlocks:        m.FilterBlocks.Load(),
		UptimeSeconds:       m.UptimeSeconds(),
	}
}

// MetricSnapshot is an immutable, JSON-serializable copy of tunnel metrics.
type MetricSnapshot struct {
	Name                string  `json:"name"`
	ID                  string  `json:"id"`
	Type                string  `json:"type"`
	ConnectionsAccepted int64   `json:"connections_accepted"`
	ConnectionsFailed   int64   `json:"connections_failed"`
	ActiveConnections   int64   `json:"active_connections"`
	BytesIn             int64   `json:"bytes_in"`
	BytesOut            int64   `json:"bytes_out"`
	ErrorCount          int64   `json:"error_count"`
	RateLimitHits       int64   `json:"rate_limit_hits"`
	FilterBlocks        int64   `json:"filter_blocks"`
	UptimeSeconds       float64 `json:"uptime_seconds"`
}

// Registry is a thread-safe collection of TunnelMetrics keyed by tunnel ID.
type Registry struct {
	mu      sync.RWMutex
	tunnels map[string]*TunnelMetrics
}

// NewRegistry creates an empty metrics Registry.
func NewRegistry() *Registry {
	return &Registry{
		tunnels: make(map[string]*TunnelMetrics),
	}
}

// Register creates or retrieves metrics for the given tunnel.
// If metrics already exist for the ID, the existing instance is returned.
func (r *Registry) Register(name, id, tunnelType string) *TunnelMetrics {
	r.mu.Lock()
	defer r.mu.Unlock()

	if m, ok := r.tunnels[id]; ok {
		return m
	}

	m := &TunnelMetrics{
		Name: name,
		ID:   id,
		Type: tunnelType,
	}
	r.tunnels[id] = m
	return m
}

// Unregister removes metrics for the given tunnel ID.
func (r *Registry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tunnels, id)
}

// Get returns metrics for a specific tunnel, or nil if not registered.
func (r *Registry) Get(id string) *TunnelMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tunnels[id]
}

// Len returns the number of registered tunnels.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tunnels)
}

// Snapshots returns point-in-time copies of all tunnel metrics.
func (r *Registry) Snapshots() []MetricSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshots := make([]MetricSnapshot, 0, len(r.tunnels))
	for _, m := range r.tunnels {
		snapshots = append(snapshots, m.Snapshot())
	}
	return snapshots
}

// DefaultRegistry is the process-wide metrics registry.
var DefaultRegistry = NewRegistry()

// MetricsBearer is implemented by tunnel types that support live metrics injection.
// The webui controller uses this interface to inject a registered TunnelMetrics
// into each tunnel after construction.
type MetricsBearer interface {
	SetTunnelMetrics(m *TunnelMetrics)
}

// CountingConn wraps a net.Conn and records byte transfers to a TunnelMetrics
// instance. Read calls record BytesIn; Write calls record BytesOut.
// Close records a disconnection event exactly once via closeOnce.
type CountingConn struct {
	net.Conn
	metrics   *TunnelMetrics
	closeOnce sync.Once
}

func (c *CountingConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 && c.metrics != nil {
		c.metrics.RecordBytesIn(int64(n))
	}
	return n, err
}

func (c *CountingConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	if n > 0 && c.metrics != nil {
		c.metrics.RecordBytesOut(int64(n))
	}
	return n, err
}

// Close records a disconnection event (once) then closes the underlying connection.
func (c *CountingConn) Close() error {
	c.closeOnce.Do(func() {
		if c.metrics != nil {
			c.metrics.RecordDisconnection()
		}
	})
	return c.Conn.Close()
}

// WrapConn returns conn wrapped in a CountingConn that records bytes and
// disconnection events to m. Returns conn unchanged when m is nil.
func WrapConn(conn net.Conn, m *TunnelMetrics) net.Conn {
	if m == nil {
		return conn
	}
	return &CountingConn{Conn: conn, metrics: m}
}
