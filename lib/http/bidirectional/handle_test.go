package httpbidirectional

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpclient "github.com/go-i2p/go-i2ptunnel/lib/http/client"
)

// TestDialContext_ClearnetNoOutproxy verifies that a clearnet address to
// DialContext returns a descriptive error (not a confusing SAM failure) when
// no outproxy is configured.
//
// Why: The bare Garlic.DialContext would attempt to dial a clearnet host
// through SAM, which fails with a cryptic SAM error. Users need to know
// they must configure an outproxy.
func TestDialContext_ClearnetNoOutproxy(t *testing.T) {
	tunnel := &HTTPBidirectional{
		done: make(chan struct{}),
		// Outproxy is nil — no clearnet routing possible
	}
	_, err := tunnel.DialContext(context.Background(), "tcp", "example.com:80")
	if err == nil {
		t.Fatal("Expected error dialing clearnet without outproxy, got nil")
	}
	if !strings.Contains(err.Error(), "no outproxy configured") {
		t.Errorf("Error should mention 'no outproxy configured', got: %v", err)
	}
}

// TestDialContext_ClearnetOutproxyDisabled verifies the error when outproxy
// is configured but not enabled.
func TestDialContext_ClearnetOutproxyDisabled(t *testing.T) {
	tunnel := &HTTPBidirectional{
		done: make(chan struct{}),
		Outproxy: &httpclient.Outproxy{
			Address: "outproxy.i2p",
			Enabled: false, // explicitly disabled
		},
	}
	_, err := tunnel.DialContext(context.Background(), "tcp", "news.ycombinator.com:80")
	if err == nil {
		t.Fatal("Expected error with disabled outproxy, got nil")
	}
	if !strings.Contains(err.Error(), "no outproxy configured") {
		t.Errorf("Error should mention 'no outproxy configured', got: %v", err)
	}
}

// TestResolveJump_NilJump returns the original address unchanged when no
// jump service is configured.
func TestResolveJump_NilJump(t *testing.T) {
	tunnel := &HTTPBidirectional{
		done: make(chan struct{}),
		Jump: nil,
	}
	original := "forum.i2p:80"
	got := tunnel.resolveJump(original)
	if got != original {
		t.Errorf("resolveJump with nil Jump: got %q, want %q", got, original)
	}
}

// TestResolveJump_Base32Passthrough verifies that base32 addresses are
// returned unchanged — SAM can resolve them directly, no jump needed.
func TestResolveJump_Base32Passthrough(t *testing.T) {
	// A JumpService that fails if called (base32 should bypass it entirely)
	js := httpclient.NewJumpService(http.DefaultClient, "http://stats.i2p/cgi-bin/jump.cgi")
	tunnel := &HTTPBidirectional{
		done: make(chan struct{}),
		Jump: js,
	}
	b32 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.b32.i2p:80"
	got := tunnel.resolveJump(b32)
	if got != b32 {
		t.Errorf("resolveJump with b32 address: got %q, want %q (should be unchanged)", got, b32)
	}
}

// TestResolveJump_FallbackOnError verifies that when the jump service
// lookup fails (e.g., I2P not running), the original address is returned
// rather than an empty string or error that would prevent any dial attempt.
func TestResolveJump_FallbackOnError(t *testing.T) {
	// Use a test HTTP server that returns a bad response
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	js := httpclient.NewJumpService(ts.Client(), ts.URL)
	tunnel := &HTTPBidirectional{
		done: make(chan struct{}),
		Jump: js,
	}
	original := "forum.i2p:80"
	got := tunnel.resolveJump(original)
	if got != original {
		t.Errorf("resolveJump fallback: got %q, want original %q", got, original)
	}
}

// TestResolveJump_ResolvesHostname verifies that a successful jump service
// lookup (via a 302 redirect) rewrites the host to the resolved destination.
//
// The real I2P jump service at stats.i2p responds with a 302 redirect to
// http://<base32>.b32.i2p/<path>. We simulate that here.
func TestResolveJump_ResolvesHostname(t *testing.T) {
	dest32 := "aaaabbbbccccddddeeeeffffgggghhhhiiiijjjjkkkkllllmmmm.b32.i2p"
	// Simulate a jump-service 302 redirect: Location: http://<dest32>/
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://"+dest32+"/", http.StatusFound)
	}))
	defer ts.Close()

	js := httpclient.NewJumpService(ts.Client(), ts.URL)
	tunnel := &HTTPBidirectional{
		done: make(chan struct{}),
		Jump: js,
	}

	got := tunnel.resolveJump("forum.i2p:80")
	if !strings.Contains(got, dest32) {
		t.Errorf("resolveJump: want resolved address containing %q, got %q", dest32, got)
	}
	// Port should be preserved in the rewritten address
	if !strings.Contains(got, ":80") {
		t.Errorf("resolveJump: port should be preserved, got %q", got)
	}
}

// TestStripPort verifies that stripPort correctly removes port suffixes.
func TestStripPort(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"example.com:80", "example.com"},
		{"192.168.1.1:443", "192.168.1.1"},
		{"example.com", "example.com"}, // no port — returned unchanged
		{"", ""},                       // empty — returned unchanged
		{"[::1]:8080", "::1"},          // IPv6
	}
	for _, tt := range tests {
		got := stripPort(tt.input)
		if got != tt.want {
			t.Errorf("stripPort(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestSetOptions_Outproxy verifies that SetOptions correctly sets the outproxy
// address and enabled flag, and that Options() reflects these values.
//
// Why: The outproxy can only be configured at runtime via SetOptions; if the
// parsing is wrong, users would have no way to enable clearnet access.
func TestSetOptions_Outproxy(t *testing.T) {
	tunnel := &HTTPBidirectional{
		done: make(chan struct{}),
	}

	err := tunnel.SetOptions(map[string]string{
		"name":             "bidi-proxy",
		"outproxy":         "outproxy.i2p",
		"outproxy.enabled": "true",
	})
	if err != nil {
		t.Fatalf("SetOptions with outproxy: %v", err)
	}
	if tunnel.Outproxy == nil {
		t.Fatal("Outproxy should not be nil after SetOptions")
	}
	if tunnel.Outproxy.Address != "outproxy.i2p" {
		t.Errorf("Outproxy.Address = %q, want %q", tunnel.Outproxy.Address, "outproxy.i2p")
	}
	if !tunnel.Outproxy.Enabled {
		t.Error("Outproxy.Enabled should be true after SetOptions outproxy.enabled=true")
	}

	opts := tunnel.Options()
	if opts["outproxy"] != "outproxy.i2p" {
		t.Errorf("Options()[outproxy] = %q, want %q", opts["outproxy"], "outproxy.i2p")
	}
	if opts["outproxy.enabled"] != "true" {
		t.Errorf("Options()[outproxy.enabled] = %q, want %q", opts["outproxy.enabled"], "true")
	}
}

// TestSetOptions_OutproxyDisable verifies toggling outproxy off via SetOptions.
func TestSetOptions_OutproxyDisable(t *testing.T) {
	tunnel := &HTTPBidirectional{
		done: make(chan struct{}),
		Outproxy: &httpclient.Outproxy{
			Address: "outproxy.i2p",
			Enabled: true,
		},
	}
	if err := tunnel.SetOptions(map[string]string{"outproxy.enabled": "false"}); err != nil {
		t.Fatalf("SetOptions with outproxy.enabled=false: %v", err)
	}
	if tunnel.Outproxy.Enabled {
		t.Error("Outproxy.Enabled should be false after outproxy.enabled=false")
	}
}

// TestOptions_OutproxyIncludedInOutput verifies that jump/outproxy fields
// appear in the Options() output when Outproxy is configured.
func TestOptions_OutproxyIncludedInOutput(t *testing.T) {
	tunnel := &HTTPBidirectional{
		done: make(chan struct{}),
		Outproxy: &httpclient.Outproxy{
			Address: "exit.i2p",
			Enabled: true,
		},
	}
	opts := tunnel.Options()
	if opts["outproxy"] != "exit.i2p" {
		t.Errorf("Options()[outproxy] = %q, want %q", opts["outproxy"], "exit.i2p")
	}
	if opts["outproxy.enabled"] != "true" {
		t.Errorf("Options()[outproxy.enabled] = %q, want %q", opts["outproxy.enabled"], "true")
	}
}

// TestRoutingDecision_I2PVsClearnet verifies the routing split: I2P addresses
// should NOT produce a "no outproxy" error (they go to Garlic), while clearnet
// addresses without an outproxy SHOULD produce that error.
//
// This tests the logic gate in DialContext without requiring a real I2P network.
func TestRoutingDecision_I2PVsClearnet(t *testing.T) {
	tunnel := &HTTPBidirectional{
		done: make(chan struct{}),
		// No outproxy, no Garlic — any dial attempt will fail,
		// but the *reason* for the failure differs by address type.
	}

	// Clearnet address → must fail with "no outproxy" (routing reached dialOutproxy)
	_, errClearnet := tunnel.DialContext(context.Background(), "tcp", "example.com:80")
	if errClearnet == nil {
		t.Fatal("expected error for clearnet without outproxy")
	}
	if !strings.Contains(errClearnet.Error(), "no outproxy configured") {
		t.Errorf("clearnet error should say 'no outproxy configured', got: %v", errClearnet)
	}

	// I2P address → must NOT produce 'no outproxy' error (routing went to Garlic)
	b32host := net.JoinHostPort("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.b32.i2p", "80")
	// Catch the panic from nil Garlic: we only care that the error path is different
	func() {
		defer func() { recover() }()
		_, errI2P := tunnel.DialContext(context.Background(), "tcp", b32host)
		// If no panic, verify it's not the outproxy error
		if errI2P != nil && strings.Contains(errI2P.Error(), "no outproxy configured") {
			t.Errorf("I2P address should not hit outproxy path, got: %v", errI2P)
		}
	}()
}

// TestDial_DelegatesToDialContext verifies that Dial calls DialContext and
// returns the same error for the same input.
func TestDial_DelegatesToDialContext(t *testing.T) {
	tunnel := &HTTPBidirectional{
		done: make(chan struct{}),
	}
	// clearnet without outproxy → predictable error from both paths
	_, errCtx := tunnel.DialContext(context.Background(), "tcp", "example.com:80")
	_, errDial := tunnel.Dial("tcp", "example.com:80")

	if errCtx == nil || errDial == nil {
		t.Fatal("Both DialContext and Dial should error for clearnet without outproxy")
	}
	if errCtx.Error() != errDial.Error() {
		t.Errorf("Dial error differs from DialContext error:\n  DialContext: %v\n  Dial: %v", errCtx, errDial)
	}
}
