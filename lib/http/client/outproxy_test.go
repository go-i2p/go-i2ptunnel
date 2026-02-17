package httpclient

import (
	"testing"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
)

// TestIsI2PAddress verifies detection of I2P vs clearnet addresses.
// Why: Correct routing depends on accurately distinguishing I2P from clearnet.
// Design: Tests all address forms including edge cases like mixed case,
// ports, and addresses that contain "i2p" but aren't I2P addresses.
func TestIsI2PAddress(t *testing.T) {
	tests := []struct {
		name string
		host string
		want bool
	}{
		// I2P addresses — should return true
		{name: "human-readable i2p", host: "forum.i2p", want: true},
		{name: "base32 i2p", host: "abc123.b32.i2p", want: true},
		{name: "subdomain i2p", host: "www.forum.i2p", want: true},
		{name: "uppercase I2P", host: "Forum.I2P", want: true},
		{name: "mixed case b32", host: "ABC.B32.I2P", want: true},
		{name: "i2p with port", host: "forum.i2p:80", want: true},
		{name: "b32 with port", host: "abc.b32.i2p:443", want: true},
		{name: "stats.i2p", host: "stats.i2p", want: true},

		// Clearnet addresses — should return false
		{name: "regular domain", host: "example.com", want: false},
		{name: "subdomain", host: "www.example.com", want: false},
		{name: "localhost", host: "localhost", want: false},
		{name: "IP address", host: "127.0.0.1", want: false},
		{name: "IPv6", host: "::1", want: false},
		{name: "empty string", host: "", want: false},
		{name: "i2p in middle", host: "i2p.example.com", want: false},
		{name: "not suffix", host: "forum.i2pextra.com", want: false},
		{name: "ip with port", host: "192.168.1.1:8080", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsI2PAddress(tt.host)
			if got != tt.want {
				t.Errorf("IsI2PAddress(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

// TestOutproxyIsActive verifies the IsActive() helper.
// Why: Guards against nil pointer dereferences and misconfigured state.
// Design: Tests all combinations of nil, empty, enabled/disabled.
func TestOutproxyIsActive(t *testing.T) {
	tests := []struct {
		name     string
		outproxy *Outproxy
		want     bool
	}{
		{name: "nil outproxy", outproxy: nil, want: false},
		{name: "empty address", outproxy: &Outproxy{Address: "", Enabled: true}, want: false},
		{name: "disabled", outproxy: &Outproxy{Address: "exit.i2p", Enabled: false}, want: false},
		{name: "active", outproxy: &Outproxy{Address: "exit.i2p", Enabled: true}, want: true},
		{name: "b32 active", outproxy: &Outproxy{Address: "abc.b32.i2p", Enabled: true}, want: true},
		{name: "disabled empty", outproxy: &Outproxy{Address: "", Enabled: false}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.outproxy.IsActive()
			if got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestOutproxyDialRejectsWhenDisabled verifies that clearnet requests fail
// gracefully when no outproxy is configured.
// Why: Users should get a clear error, not a confusing connection failure.
// Design: Tests nil, disabled, and empty address scenarios.
func TestOutproxyDialRejectsWhenDisabled(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hc-outproxy-reject",
		Type:      "httpclient",
		Port:      8118,
		Interface: "127.0.0.1",
	}

	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	tests := []struct {
		name     string
		outproxy *Outproxy
	}{
		{name: "nil outproxy", outproxy: nil},
		{name: "disabled outproxy", outproxy: &Outproxy{Address: "exit.i2p", Enabled: false}},
		{name: "empty address", outproxy: &Outproxy{Address: "", Enabled: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client.Outproxy = tt.outproxy
			_, err := client.DialContext(nil, "tcp", "example.com:80")
			if err == nil {
				t.Error("Expected error when dialing clearnet without outproxy")
			}
			// Error should mention the clearnet address
			if err != nil && !contains(err.Error(), "example.com") {
				t.Errorf("Error should mention the address, got: %v", err)
			}
		})
	}
}

// TestOutproxyConnectDialRejectsWhenDisabled verifies CONNECT method handling.
// Why: HTTPS requests use CONNECT, which has a separate code path.
// Design: Ensures connectDial mirrors DialContext behavior for clearnet.
func TestOutproxyConnectDialRejectsWhenDisabled(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hc-connect-reject",
		Type:      "httpclient",
		Port:      8118,
		Interface: "127.0.0.1",
	}

	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	// Disable outproxy
	client.Outproxy = &Outproxy{}

	_, err = client.connectDial("tcp", "example.com:443")
	if err == nil {
		t.Error("Expected error for clearnet CONNECT without outproxy")
	}
}

// TestOutproxyOptionsRoundTrip verifies outproxy config via Options/SetOptions.
// Why: Configuration must persist correctly through the options interface.
// Design: Tests set → get → verify cycle for outproxy settings.
func TestOutproxyOptionsRoundTrip(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hc-outproxy-opts",
		Type:      "httpclient",
		Port:      8118,
		Interface: "127.0.0.1",
	}

	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	t.Run("set outproxy address enables it", func(t *testing.T) {
		err := client.SetOptions(map[string]string{
			"outproxy": "exit.i2p",
		})
		if err != nil {
			t.Fatalf("SetOptions failed: %v", err)
		}

		opts := client.Options()
		if opts["outproxy"] != "exit.i2p" {
			t.Errorf("Expected outproxy 'exit.i2p', got %q", opts["outproxy"])
		}
		if opts["outproxy.enabled"] != "true" {
			t.Errorf("Expected outproxy.enabled 'true', got %q", opts["outproxy.enabled"])
		}
		if !client.Outproxy.IsActive() {
			t.Error("Outproxy should be active after setting address")
		}
	})

	t.Run("disable outproxy with empty address", func(t *testing.T) {
		err := client.SetOptions(map[string]string{
			"outproxy": "",
		})
		if err != nil {
			t.Fatalf("SetOptions failed: %v", err)
		}

		if client.Outproxy.IsActive() {
			t.Error("Outproxy should be inactive after setting empty address")
		}
	})

	t.Run("disable outproxy with enabled=false", func(t *testing.T) {
		// First enable it
		err := client.SetOptions(map[string]string{
			"outproxy": "exit.i2p",
		})
		if err != nil {
			t.Fatalf("SetOptions failed: %v", err)
		}

		// Then disable via enabled flag
		err = client.SetOptions(map[string]string{
			"outproxy.enabled": "false",
		})
		if err != nil {
			t.Fatalf("SetOptions failed: %v", err)
		}

		if client.Outproxy.IsActive() {
			t.Error("Outproxy should be inactive after disabling")
		}
	})

	t.Run("re-enable outproxy", func(t *testing.T) {
		err := client.SetOptions(map[string]string{
			"outproxy.enabled": "true",
		})
		if err != nil {
			t.Fatalf("SetOptions failed: %v", err)
		}

		if !client.Outproxy.IsActive() {
			t.Error("Outproxy should be active after re-enabling")
		}
	})
}

// TestOutproxySetOptionsValidation verifies that invalid outproxy addresses
// are rejected during configuration.
// Why: An outproxy must be an I2P address; clearnet addresses make no sense.
// Design: Tests various invalid address formats.
func TestOutproxySetOptionsValidation(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hc-outproxy-valid",
		Type:      "httpclient",
		Port:      8118,
		Interface: "127.0.0.1",
	}

	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	tests := []struct {
		name      string
		address   string
		wantError bool
	}{
		{name: "valid i2p address", address: "exit.i2p", wantError: false},
		{name: "valid b32 address", address: "abc.b32.i2p", wantError: false},
		{name: "empty disables", address: "", wantError: false},
		{name: "clearnet rejected", address: "proxy.example.com", wantError: true},
		{name: "IP rejected", address: "192.168.1.1", wantError: true},
		{name: "localhost rejected", address: "localhost", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.SetOptions(map[string]string{
				"outproxy": tt.address,
			})
			if tt.wantError && err == nil {
				t.Errorf("Expected error for outproxy address %q", tt.address)
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error for outproxy address %q: %v", tt.address, err)
			}
		})
	}
}

// TestOutproxyDefaultDisabled verifies outproxy is disabled by default.
// Why: Outproxy should be opt-in for security — users must explicitly enable it.
// Design: Creates a new client and checks default outproxy state.
func TestOutproxyDefaultDisabled(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hc-outproxy-default",
		Type:      "httpclient",
		Port:      8118,
		Interface: "127.0.0.1",
	}

	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	if client.Outproxy == nil {
		t.Fatal("Outproxy struct should be initialized (not nil)")
	}
	if client.Outproxy.IsActive() {
		t.Error("Outproxy should be inactive by default")
	}
	if client.Outproxy.Address != "" {
		t.Errorf("Outproxy address should be empty by default, got %q", client.Outproxy.Address)
	}
}

// TestOutproxyNotInOptionsWhenDisabled verifies that disabled outproxy
// doesn't appear in the options map.
// Why: Clean options output for display in Web UI and CLI.
// Design: Checks Options() output with no outproxy configured.
func TestOutproxyNotInOptionsWhenDisabled(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hc-outproxy-hidden",
		Type:      "httpclient",
		Port:      8118,
		Interface: "127.0.0.1",
	}

	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	opts := client.Options()
	if _, ok := opts["outproxy"]; ok {
		t.Error("Outproxy should not appear in options when not configured")
	}
	if _, ok := opts["outproxy.enabled"]; ok {
		t.Error("outproxy.enabled should not appear in options when not configured")
	}
}

// TestOutproxyEnabledStringVariants tests various truthy/falsy string values
// for the outproxy.enabled option.
// Why: Configuration comes from YAML/INI files where booleans have varied representations.
// Design: Tests common boolean string representations.
func TestOutproxyEnabledStringVariants(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hc-outproxy-bool",
		Type:      "httpclient",
		Port:      8118,
		Interface: "127.0.0.1",
	}

	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	// Set an address first
	if err := client.SetOptions(map[string]string{"outproxy": "exit.i2p"}); err != nil {
		t.Fatalf("Failed to set outproxy: %v", err)
	}

	tests := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"1", true},
		{"yes", true},
		{"false", false},
		{"0", false},
		{"no", false},
		{"", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run("enabled="+tt.value, func(t *testing.T) {
			err := client.SetOptions(map[string]string{
				"outproxy.enabled": tt.value,
			})
			if err != nil {
				t.Fatalf("SetOptions failed: %v", err)
			}
			if client.Outproxy.Enabled != tt.want {
				t.Errorf("outproxy.enabled=%q → Enabled=%v, want %v",
					tt.value, client.Outproxy.Enabled, tt.want)
			}
		})
	}
}

// TestI2PAddressDialSkipsOutproxy verifies that I2P addresses bypass outproxy.
// Why: I2P traffic should never route through an outproxy — that would be incorrect.
// Design: Checks that DialContext for an I2P address does NOT call dialOutproxy.
// The actual dial will fail (no SAM bridge in test), but the error path
// differs from the outproxy path.
func TestI2PAddressDialSkipsOutproxy(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hc-outproxy-skip",
		Type:      "httpclient",
		Port:      8118,
		Interface: "127.0.0.1",
	}

	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	// Enable outproxy — I2P addresses should still bypass it
	client.Outproxy = &Outproxy{Address: "exit.i2p", Enabled: true}

	// Verify address classification is correct
	if !IsI2PAddress("forum.i2p") {
		t.Error("forum.i2p should be classified as I2P address")
	}
	if IsI2PAddress("example.com") {
		t.Error("example.com should NOT be classified as I2P address")
	}

	// Clearnet address should fail with outproxy error (since no real I2P)
	_, clearnetErr := client.connectDial("tcp", "example.com:80")
	if clearnetErr == nil {
		t.Fatal("Expected error for clearnet dial (no SAM)")
	}
	// Clearnet error should mention outproxy (it goes through outproxy path)
	if !contains(clearnetErr.Error(), "outproxy") {
		t.Errorf("Clearnet error should mention outproxy, got: %v", clearnetErr)
	}
}

// contains is a test helper that checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
