package metrics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// TunnelMetrics tests
// ---------------------------------------------------------------------------

func TestTunnelMetrics_RecordConnection(t *testing.T) {
	m := &TunnelMetrics{Name: "test", ID: "t1", Type: "tcpclient"}

	m.RecordConnection()
	m.RecordConnection()
	m.RecordConnection()

	if got := m.ConnectionsAccepted.Load(); got != 3 {
		t.Errorf("ConnectionsAccepted = %d, want 3", got)
	}
	if got := m.ActiveConnections.Load(); got != 3 {
		t.Errorf("ActiveConnections = %d, want 3", got)
	}

	m.RecordDisconnection()
	if got := m.ActiveConnections.Load(); got != 2 {
		t.Errorf("ActiveConnections after disconnect = %d, want 2", got)
	}
}

func TestTunnelMetrics_RecordConnectionFailed(t *testing.T) {
	m := &TunnelMetrics{}
	m.RecordConnectionFailed()
	m.RecordConnectionFailed()

	if got := m.ConnectionsFailed.Load(); got != 2 {
		t.Errorf("ConnectionsFailed = %d, want 2", got)
	}
}

func TestTunnelMetrics_RecordBytes(t *testing.T) {
	m := &TunnelMetrics{}
	m.RecordBytesIn(100)
	m.RecordBytesIn(200)
	m.RecordBytesOut(50)

	if got := m.BytesIn.Load(); got != 300 {
		t.Errorf("BytesIn = %d, want 300", got)
	}
	if got := m.BytesOut.Load(); got != 50 {
		t.Errorf("BytesOut = %d, want 50", got)
	}
}

func TestTunnelMetrics_RecordError(t *testing.T) {
	m := &TunnelMetrics{}
	m.RecordError()
	m.RecordError()
	m.RecordError()

	if got := m.ErrorCount.Load(); got != 3 {
		t.Errorf("ErrorCount = %d, want 3", got)
	}
}

func TestTunnelMetrics_RecordRateLimitHit(t *testing.T) {
	m := &TunnelMetrics{}
	m.RecordRateLimitHit()

	if got := m.RateLimitHits.Load(); got != 1 {
		t.Errorf("RateLimitHits = %d, want 1", got)
	}
}

func TestTunnelMetrics_RecordFilterBlock(t *testing.T) {
	m := &TunnelMetrics{}
	m.RecordFilterBlock()
	m.RecordFilterBlock()

	if got := m.FilterBlocks.Load(); got != 2 {
		t.Errorf("FilterBlocks = %d, want 2", got)
	}
}

func TestTunnelMetrics_StartStop(t *testing.T) {
	m := &TunnelMetrics{}

	// Not started yet — uptime should be zero.
	if uptime := m.UptimeSeconds(); uptime != 0 {
		t.Errorf("UptimeSeconds before start = %f, want 0", uptime)
	}

	m.RecordStart()
	time.Sleep(50 * time.Millisecond)
	uptime := m.UptimeSeconds()

	if uptime < 0.04 {
		t.Errorf("UptimeSeconds after 50ms = %f, want >= 0.04", uptime)
	}

	m.RecordStop()
	if uptime := m.UptimeSeconds(); uptime != 0 {
		t.Errorf("UptimeSeconds after stop = %f, want 0", uptime)
	}
}

func TestTunnelMetrics_Snapshot(t *testing.T) {
	m := &TunnelMetrics{Name: "web", ID: "web-1", Type: "httpserver"}
	m.RecordConnection()
	m.RecordConnection()
	m.RecordConnectionFailed()
	m.RecordBytesIn(1024)
	m.RecordBytesOut(512)
	m.RecordError()
	m.RecordRateLimitHit()
	m.RecordFilterBlock()
	m.RecordStart()
	time.Sleep(10 * time.Millisecond)

	snap := m.Snapshot()

	if snap.Name != "web" || snap.ID != "web-1" || snap.Type != "httpserver" {
		t.Errorf("labels mismatch: %+v", snap)
	}
	if snap.ConnectionsAccepted != 2 {
		t.Errorf("snap.ConnectionsAccepted = %d, want 2", snap.ConnectionsAccepted)
	}
	if snap.ConnectionsFailed != 1 {
		t.Errorf("snap.ConnectionsFailed = %d, want 1", snap.ConnectionsFailed)
	}
	if snap.ActiveConnections != 2 {
		t.Errorf("snap.ActiveConnections = %d, want 2", snap.ActiveConnections)
	}
	if snap.BytesIn != 1024 {
		t.Errorf("snap.BytesIn = %d, want 1024", snap.BytesIn)
	}
	if snap.BytesOut != 512 {
		t.Errorf("snap.BytesOut = %d, want 512", snap.BytesOut)
	}
	if snap.ErrorCount != 1 {
		t.Errorf("snap.ErrorCount = %d, want 1", snap.ErrorCount)
	}
	if snap.RateLimitHits != 1 {
		t.Errorf("snap.RateLimitHits = %d, want 1", snap.RateLimitHits)
	}
	if snap.FilterBlocks != 1 {
		t.Errorf("snap.FilterBlocks = %d, want 1", snap.FilterBlocks)
	}
	if snap.UptimeSeconds < 0.005 {
		t.Errorf("snap.UptimeSeconds = %f, want >= 0.005", snap.UptimeSeconds)
	}
}

func TestTunnelMetrics_ConcurrentAccess(t *testing.T) {
	m := &TunnelMetrics{Name: "concurrent", ID: "c1", Type: "tcpserver"}
	m.RecordStart()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.RecordConnection()
			m.RecordBytesIn(10)
			m.RecordBytesOut(5)
			m.RecordDisconnection()
		}()
	}
	wg.Wait()

	if got := m.ConnectionsAccepted.Load(); got != 100 {
		t.Errorf("ConnectionsAccepted = %d, want 100", got)
	}
	if got := m.ActiveConnections.Load(); got != 0 {
		t.Errorf("ActiveConnections = %d, want 0", got)
	}
	if got := m.BytesIn.Load(); got != 1000 {
		t.Errorf("BytesIn = %d, want 1000", got)
	}
	if got := m.BytesOut.Load(); got != 500 {
		t.Errorf("BytesOut = %d, want 500", got)
	}
}

// ---------------------------------------------------------------------------
// Registry tests
// ---------------------------------------------------------------------------

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	m := r.Register("tun", "tun-1", "tcpclient")

	if m == nil {
		t.Fatal("Register returned nil")
	}
	if m.Name != "tun" || m.ID != "tun-1" || m.Type != "tcpclient" {
		t.Errorf("unexpected labels: %+v", m)
	}
	if r.Len() != 1 {
		t.Errorf("Len() = %d, want 1", r.Len())
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	m1 := r.Register("tun", "tun-1", "tcpclient")
	m2 := r.Register("tun-other-name", "tun-1", "tcpserver")

	// Should return the same pointer — first registration wins.
	if m1 != m2 {
		t.Error("duplicate Register should return existing instance")
	}
	if r.Len() != 1 {
		t.Errorf("Len() = %d, want 1", r.Len())
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := NewRegistry()
	r.Register("tun", "tun-1", "tcpclient")
	r.Unregister("tun-1")

	if r.Len() != 0 {
		t.Errorf("Len() after unregister = %d, want 0", r.Len())
	}
	if m := r.Get("tun-1"); m != nil {
		t.Error("Get should return nil after Unregister")
	}
}

func TestRegistry_UnregisterNonexistent(t *testing.T) {
	r := NewRegistry()
	// Should not panic.
	r.Unregister("nope")
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	r.Register("tun", "tun-1", "tcpclient")

	if m := r.Get("tun-1"); m == nil {
		t.Error("Get returned nil for registered tunnel")
	}
	if m := r.Get("nope"); m != nil {
		t.Error("Get returned non-nil for unregistered tunnel")
	}
}

func TestRegistry_Snapshots(t *testing.T) {
	r := NewRegistry()
	m1 := r.Register("a", "a-1", "tcpclient")
	m2 := r.Register("b", "b-1", "httpserver")
	m1.RecordConnection()
	m2.RecordBytesIn(42)

	snaps := r.Snapshots()
	if len(snaps) != 2 {
		t.Fatalf("Snapshots() returned %d, want 2", len(snaps))
	}

	// Find each by ID.
	found := map[string]MetricSnapshot{}
	for _, s := range snaps {
		found[s.ID] = s
	}
	if s, ok := found["a-1"]; !ok || s.ConnectionsAccepted != 1 {
		t.Errorf("a-1 snapshot wrong: %+v", found["a-1"])
	}
	if s, ok := found["b-1"]; !ok || s.BytesIn != 42 {
		t.Errorf("b-1 snapshot wrong: %+v", found["b-1"])
	}
}

func TestRegistry_SnapshotsEmpty(t *testing.T) {
	r := NewRegistry()
	snaps := r.Snapshots()
	if len(snaps) != 0 {
		t.Errorf("Snapshots() on empty registry returned %d items", len(snaps))
	}
}

// ---------------------------------------------------------------------------
// Prometheus format tests
// ---------------------------------------------------------------------------

func TestWritePrometheus_Empty(t *testing.T) {
	r := NewRegistry()
	var buf strings.Builder
	if err := r.WritePrometheus(&buf); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	// Should still have HELP/TYPE lines but no data lines.
	if !strings.Contains(output, "# HELP i2ptunnel_connections_accepted_total") {
		t.Error("missing HELP header for connections_accepted")
	}
	if !strings.Contains(output, "# TYPE i2ptunnel_uptime_seconds gauge") {
		t.Error("missing TYPE header for uptime")
	}
}

func TestWritePrometheus_WithData(t *testing.T) {
	r := NewRegistry()
	m := r.Register("irc", "irc-1", "ircclient")
	m.RecordConnection()
	m.RecordBytesIn(256)
	m.RecordFilterBlock()

	var buf strings.Builder
	if err := r.WritePrometheus(&buf); err != nil {
		t.Fatal(err)
	}

	output := buf.String()

	// Verify counter values appear.
	if !strings.Contains(output, `i2ptunnel_connections_accepted_total{name="irc",id="irc-1",type="ircclient"} 1`) {
		t.Errorf("missing connections_accepted line in:\n%s", output)
	}
	if !strings.Contains(output, `i2ptunnel_bytes_received_total{name="irc",id="irc-1",type="ircclient"} 256`) {
		t.Errorf("missing bytes_received line in:\n%s", output)
	}
	if !strings.Contains(output, `i2ptunnel_filter_blocks_total{name="irc",id="irc-1",type="ircclient"} 1`) {
		t.Errorf("missing filter_blocks line in:\n%s", output)
	}
}

func TestEscapeLabelValue(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{`simple`, `simple`},
		{`back\slash`, `back\\slash`},
		{`has"quote`, `has\"quote`},
		{"has\nnewline", `has\nnewline`},
		{`all\"\n`, `all\\\"\\n`},
	}
	for _, tc := range tests {
		got := escapeLabelValue(tc.input)
		if got != tc.want {
			t.Errorf("escapeLabelValue(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFormatLabels(t *testing.T) {
	got := formatLabels("my tunnel", "id-1", "tcpclient")
	want := `name="my tunnel",id="id-1",type="tcpclient"`
	if got != want {
		t.Errorf("formatLabels() = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// HTTP handler tests
// ---------------------------------------------------------------------------

func newTestHandler() *Handler {
	r := NewRegistry()
	m := r.Register("test-tunnel", "tt-1", "tcpclient")
	m.RecordConnection()
	m.RecordStart()

	statusFunc := func() []TunnelStatus {
		return []TunnelStatus{
			{
				Name:         "test-tunnel",
				ID:           "tt-1",
				Type:         "tcpclient",
				Status:       "running",
				Address:      "abcd.b32.i2p",
				Target:       "target.b32.i2p",
				LocalAddress: "127.0.0.1:4444",
			},
			{
				Name:   "stopped-tunnel",
				ID:     "st-1",
				Type:   "httpserver",
				Status: "stopped",
			},
		}
	}

	return NewHandler(r, statusFunc)
}

func TestHandleHealth(t *testing.T) {
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	h.HandleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Status != "ok" {
		t.Errorf("health status = %q, want ok", resp.Status)
	}
	if resp.TunnelsTotal != 2 {
		t.Errorf("TunnelsTotal = %d, want 2", resp.TunnelsTotal)
	}
	if resp.TunnelsRunning != 1 {
		t.Errorf("TunnelsRunning = %d, want 1", resp.TunnelsRunning)
	}
}

func TestHandleHealth_NoTunnels(t *testing.T) {
	h := NewHandler(NewRegistry(), func() []TunnelStatus { return nil })
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	h.HandleHealth(rec, req)

	var resp HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q, want ok", resp.Status)
	}
	if resp.TunnelsTotal != 0 {
		t.Errorf("TunnelsTotal = %d, want 0", resp.TunnelsTotal)
	}
}

func TestHandleStatus(t *testing.T) {
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()

	h.HandleStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp StatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if len(resp.Tunnels) != 2 {
		t.Fatalf("len(Tunnels) = %d, want 2", len(resp.Tunnels))
	}

	// First tunnel should have metrics (it's registered in the registry).
	found := false
	for _, td := range resp.Tunnels {
		if td.ID == "tt-1" {
			found = true
			if td.Status != "running" {
				t.Errorf("tunnel status = %q, want running", td.Status)
			}
			if td.Metrics == nil {
				t.Error("expected metrics for registered tunnel")
			} else if td.Metrics.ConnectionsAccepted != 1 {
				t.Errorf("metrics.ConnectionsAccepted = %d, want 1", td.Metrics.ConnectionsAccepted)
			}
		}
	}
	if !found {
		t.Error("tunnel tt-1 not found in status response")
	}

	// Second tunnel has no metrics registered.
	for _, td := range resp.Tunnels {
		if td.ID == "st-1" {
			if td.Metrics != nil {
				t.Error("stopped-tunnel should have nil metrics (not registered)")
			}
		}
	}
}

func TestHandleMetrics(t *testing.T) {
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	h.HandleMetrics(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain prefix", ct)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "i2ptunnel_connections_accepted_total") {
		t.Error("Prometheus output missing connections_accepted metric")
	}
	if !strings.Contains(body, `name="test-tunnel"`) {
		t.Error("Prometheus output missing tunnel label")
	}
}

func TestHandleMetrics_EmptyRegistry(t *testing.T) {
	h := NewHandler(NewRegistry(), func() []TunnelStatus { return nil })
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	h.HandleMetrics(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	// Should still have HELP/TYPE lines.
	if !strings.Contains(rec.Body.String(), "# HELP") {
		t.Error("empty metrics output missing HELP lines")
	}
}

// ---------------------------------------------------------------------------
// MetricSnapshot JSON serialization test
// ---------------------------------------------------------------------------

func TestMetricSnapshot_JSON(t *testing.T) {
	snap := MetricSnapshot{
		Name:                "test",
		ID:                  "t-1",
		Type:                "tcpclient",
		ConnectionsAccepted: 10,
		BytesIn:             1024,
		UptimeSeconds:       60.5,
	}

	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}

	var decoded MetricSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Name != "test" || decoded.ConnectionsAccepted != 10 || decoded.BytesIn != 1024 {
		t.Errorf("JSON round-trip mismatch: %+v", decoded)
	}
}

// ---------------------------------------------------------------------------
// DefaultRegistry test
// ---------------------------------------------------------------------------

func TestDefaultRegistry(t *testing.T) {
	// DefaultRegistry should be initialized and usable.
	if DefaultRegistry == nil {
		t.Fatal("DefaultRegistry is nil")
	}

	id := "default-reg-test-" + time.Now().Format("150405.000")
	m := DefaultRegistry.Register("test", id, "tcpclient")
	if m == nil {
		t.Fatal("DefaultRegistry.Register returned nil")
	}
	DefaultRegistry.Unregister(id)
}
