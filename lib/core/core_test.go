package i2ptunnel

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
)

// --- Clean() tests ---

func TestClean(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", ""},
		{"no special chars", "tunnel1", "tunnel1"},
		{"spaces to dashes", "my tunnel", "my-tunnel"},
		{"tabs to underscores", "my\ttunnel", "my_tunnel"},
		{"newlines to plus", "my\ntunnel", "my+tunnel"},
		{"slashes removed", "my/tunnel", "mytunnel"},
		{"all replacements", "a b\tc\nd/e", "a-b_c+de"},
		{"multiple spaces", "a  b", "a--b"},
		{"leading slash", "/tunnel", "tunnel"},
		{"trailing slash", "tunnel/", "tunnel"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Clean(tt.input)
			if got != tt.want {
				t.Errorf("Clean(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- I2PTunnelError / NewError tests ---

// mockTunnel implements I2PTunnel for testing NewError.
type mockTunnel struct {
	name string
}

func (m *mockTunnel) Start() error                       { return nil }
func (m *mockTunnel) Stop() error                        { return nil }
func (m *mockTunnel) Name() string                       { return m.name }
func (m *mockTunnel) ID() string                         { return Clean(m.name) }
func (m *mockTunnel) Type() string                       { return "test" }
func (m *mockTunnel) Address() string                    { return "" }
func (m *mockTunnel) Target() string                     { return "" }
func (m *mockTunnel) Options() map[string]string         { return nil }
func (m *mockTunnel) SetOptions(map[string]string) error { return nil }
func (m *mockTunnel) LoadConfig(string) error            { return nil }
func (m *mockTunnel) Status() I2PTunnelStatus            { return I2PTunnelStatusStopped }
func (m *mockTunnel) Error() error                       { return nil }
func (m *mockTunnel) LocalAddress() (string, error)      { return "", nil }

func TestNewError(t *testing.T) {
	tun := &mockTunnel{name: "test-tunnel"}
	origErr := errors.New("connection refused")
	tunnelErr := NewError(tun, origErr)

	if !strings.Contains(tunnelErr.Error(), "test-tunnel") {
		t.Errorf("error should contain tunnel name, got: %s", tunnelErr.Error())
	}
	if !strings.Contains(tunnelErr.Error(), "connection refused") {
		t.Errorf("error should contain original error, got: %s", tunnelErr.Error())
	}
}

func TestI2PTunnelErrorImplementsError(t *testing.T) {
	var _ error = I2PTunnelError{}
	e := I2PTunnelError{}
	if e.Error() != "" {
		t.Errorf("zero-value error should be empty, got %q", e.Error())
	}
}

// --- ErrorTracker tests ---

func TestErrorTrackerEmpty(t *testing.T) {
	var et ErrorTracker
	if et.Last() != nil {
		t.Error("Last() on empty tracker should return nil")
	}
	if et.All() != nil {
		t.Error("All() on empty tracker should return nil")
	}
}

func TestErrorTrackerRecordAndLast(t *testing.T) {
	var et ErrorTracker
	tun := &mockTunnel{name: "tracker-test"}

	et.Record(tun, errors.New("first"))
	et.Record(tun, errors.New("second"))

	last := et.Last()
	if last == nil {
		t.Fatal("Last() should not be nil after recording")
	}
	if !strings.Contains(last.Error(), "second") {
		t.Errorf("Last() should contain most recent error, got: %s", last.Error())
	}
}

func TestErrorTrackerAll(t *testing.T) {
	var et ErrorTracker
	tun := &mockTunnel{name: "all-test"}

	for i := 0; i < 5; i++ {
		et.Record(tun, fmt.Errorf("err-%d", i))
	}

	all := et.All()
	if len(all) != 5 {
		t.Fatalf("expected 5 errors, got %d", len(all))
	}
	// Verify ordering: first recorded should be first in slice
	if !strings.Contains(all[0].Error(), "err-0") {
		t.Errorf("first error should contain 'err-0', got: %s", all[0].Error())
	}
	if !strings.Contains(all[4].Error(), "err-4") {
		t.Errorf("last error should contain 'err-4', got: %s", all[4].Error())
	}
}

func TestErrorTrackerBoundsMaxErrors(t *testing.T) {
	var et ErrorTracker
	tun := &mockTunnel{name: "bounds-test"}

	for i := 0; i < MaxErrors+50; i++ {
		et.Record(tun, fmt.Errorf("error-%d", i))
	}

	all := et.All()
	if len(all) != MaxErrors {
		t.Fatalf("expected %d errors, got %d", MaxErrors, len(all))
	}
	// Oldest should be discarded: first error should be error-50
	if !strings.Contains(all[0].Error(), "error-50") {
		t.Errorf("oldest retained should be error-50, got: %s", all[0].Error())
	}
	// Latest should be preserved
	if !strings.Contains(all[MaxErrors-1].Error(), fmt.Sprintf("error-%d", MaxErrors+49)) {
		t.Errorf("newest should be error-%d, got: %s", MaxErrors+49, all[MaxErrors-1].Error())
	}
}

func TestErrorTrackerAllReturnsSnapshot(t *testing.T) {
	var et ErrorTracker
	tun := &mockTunnel{name: "snapshot-test"}

	et.Record(tun, errors.New("original"))
	snapshot := et.All()

	et.Record(tun, errors.New("added-later"))
	if len(snapshot) != 1 {
		t.Error("snapshot should not grow after further Record() calls")
	}
}

func TestErrorTrackerConcurrent(t *testing.T) {
	var et ErrorTracker
	tun := &mockTunnel{name: "concurrent-test"}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			et.Record(tun, fmt.Errorf("err-%d", n))
		}(i)
	}
	wg.Wait()

	all := et.All()
	if len(all) != 50 {
		t.Errorf("expected 50 errors after concurrent writes, got %d", len(all))
	}
}

// --- BuildCommonOptions tests ---

func TestBuildCommonOptions(t *testing.T) {
	cfg := i2pconv.TunnelConfig{
		Name:      "test-tunnel",
		Type:      "tcpclient",
		Interface: "127.0.0.1",
		Port:      8080,
	}
	opts := BuildCommonOptions(cfg)

	expected := map[string]string{
		"name":      "test-tunnel",
		"type":      "tcpclient",
		"interface": "127.0.0.1",
		"port":      "8080",
	}
	for k, want := range expected {
		if got := opts[k]; got != want {
			t.Errorf("opts[%q] = %q, want %q", k, got, want)
		}
	}
}

func TestBuildCommonOptionsWithI2CP(t *testing.T) {
	cfg := i2pconv.TunnelConfig{
		Name:      "test",
		Type:      "httpserver",
		Interface: "127.0.0.1",
		Port:      80,
		I2CP: map[string]interface{}{
			"leaseSetEncType": "4,0",
		},
	}
	opts := BuildCommonOptions(cfg)

	if got := opts["i2cp.leaseSetEncType"]; got != "4,0" {
		t.Errorf("expected I2CP option merged, got %q", got)
	}
}

// --- AddRateLimitOptions tests ---

func TestAddRateLimitOptions(t *testing.T) {
	opts := map[string]string{}
	AddRateLimitOptions(opts, 500, 10.5)

	if got := opts["maxconns"]; got != "500" {
		t.Errorf("maxconns = %q, want %q", got, "500")
	}
	if got := opts["ratelimit"]; got != "10.5" {
		t.Errorf("ratelimit = %q, want %q", got, "10.5")
	}
}

func TestAddRateLimitOptionsZero(t *testing.T) {
	opts := map[string]string{}
	AddRateLimitOptions(opts, 0, 0)

	if got := opts["maxconns"]; got != "0" {
		t.Errorf("maxconns = %q, want %q", got, "0")
	}
	if got := opts["ratelimit"]; got != "0" {
		t.Errorf("ratelimit = %q, want %q", got, "0")
	}
}

// --- ApplyCommonOptions tests ---

func TestApplyCommonOptionsValid(t *testing.T) {
	opts := map[string]string{
		"name":      "my-tunnel",
		"interface": "127.0.0.1",
		"port":      "9090",
	}
	cfg := &i2pconv.TunnelConfig{}
	err := ApplyCommonOptions(opts, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "my-tunnel" {
		t.Errorf("Name = %q, want %q", cfg.Name, "my-tunnel")
	}
	if cfg.Interface != "127.0.0.1" {
		t.Errorf("Interface = %q, want %q", cfg.Interface, "127.0.0.1")
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want %d", cfg.Port, 9090)
	}
}

func TestApplyCommonOptionsEmptyName(t *testing.T) {
	opts := map[string]string{"name": ""}
	cfg := &i2pconv.TunnelConfig{}
	err := ApplyCommonOptions(opts, cfg)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestApplyCommonOptionsInvalidInterface(t *testing.T) {
	opts := map[string]string{"interface": "not-an-ip"}
	cfg := &i2pconv.TunnelConfig{}
	err := ApplyCommonOptions(opts, cfg)
	if err == nil {
		t.Fatal("expected error for invalid interface")
	}
}

func TestApplyCommonOptionsInvalidPort(t *testing.T) {
	opts := map[string]string{"port": "notanumber"}
	cfg := &i2pconv.TunnelConfig{}
	err := ApplyCommonOptions(opts, cfg)
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestApplyCommonOptionsPortOutOfRange(t *testing.T) {
	opts := map[string]string{"port": "99999"}
	cfg := &i2pconv.TunnelConfig{}
	err := ApplyCommonOptions(opts, cfg)
	if err == nil {
		t.Fatal("expected error for port out of range")
	}
}

func TestApplyCommonOptionsI2CPExtraction(t *testing.T) {
	opts := map[string]string{
		"name":                 "test",
		"i2cp.leaseSetEncType": "4,0",
		"i2cp.leaseSetType":    "3",
	}
	cfg := &i2pconv.TunnelConfig{}
	err := ApplyCommonOptions(opts, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.I2CP == nil {
		t.Fatal("I2CP should not be nil")
	}
	if cfg.I2CP["leaseSetEncType"] != "4,0" {
		t.Errorf("I2CP leaseSetEncType = %v, want %q", cfg.I2CP["leaseSetEncType"], "4,0")
	}
}

func TestApplyCommonOptionsEmptyMap(t *testing.T) {
	cfg := &i2pconv.TunnelConfig{Name: "original"}
	err := ApplyCommonOptions(map[string]string{}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "original" {
		t.Error("Name should not change when not present in options")
	}
}

// --- ApplyRateLimitOptions tests ---

func TestApplyRateLimitOptionsValid(t *testing.T) {
	opts := map[string]string{
		"maxconns":  "100",
		"ratelimit": "50.5",
	}
	var mc int
	var rl float64
	err := ApplyRateLimitOptions(opts, &mc, &rl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mc != 100 {
		t.Errorf("maxconns = %d, want %d", mc, 100)
	}
	if rl != 50.5 {
		t.Errorf("ratelimit = %f, want %f", rl, 50.5)
	}
}

func TestApplyRateLimitOptionsInvalidMaxConns(t *testing.T) {
	opts := map[string]string{"maxconns": "abc"}
	var mc int
	var rl float64
	err := ApplyRateLimitOptions(opts, &mc, &rl)
	if err == nil {
		t.Fatal("expected error for invalid maxconns")
	}
}

func TestApplyRateLimitOptionsNegativeMaxConns(t *testing.T) {
	opts := map[string]string{"maxconns": "-1"}
	var mc int
	var rl float64
	err := ApplyRateLimitOptions(opts, &mc, &rl)
	if err == nil {
		t.Fatal("expected error for negative maxconns")
	}
}

func TestApplyRateLimitOptionsInvalidRateLimit(t *testing.T) {
	opts := map[string]string{"ratelimit": "abc"}
	var mc int
	var rl float64
	err := ApplyRateLimitOptions(opts, &mc, &rl)
	if err == nil {
		t.Fatal("expected error for invalid ratelimit")
	}
}

func TestApplyRateLimitOptionsNegativeRateLimit(t *testing.T) {
	opts := map[string]string{"ratelimit": "-5.0"}
	var mc int
	var rl float64
	err := ApplyRateLimitOptions(opts, &mc, &rl)
	if err == nil {
		t.Fatal("expected error for negative ratelimit")
	}
}

func TestApplyRateLimitOptionsEmptyMap(t *testing.T) {
	mc := 999
	rl := 88.8
	err := ApplyRateLimitOptions(map[string]string{}, &mc, &rl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mc != 999 || rl != 88.8 {
		t.Error("values should not change when keys are absent")
	}
}

// --- I2PTunnelStatus tests ---

func TestI2PTunnelStatusValues(t *testing.T) {
	statuses := map[I2PTunnelStatus]string{
		I2PTunnelStatusRunning:  "running",
		I2PTunnelStatusStopped:  "stopped",
		I2PTunnelStatusStarting: "starting",
		I2PTunnelStatusStopping: "stopping",
		I2PTunnelStatusFailed:   "failed",
		I2PTunnelStatusUnknown:  "unknown",
	}
	for status, want := range statuses {
		if string(status) != want {
			t.Errorf("status %v = %q, want %q", status, string(status), want)
		}
	}
}
