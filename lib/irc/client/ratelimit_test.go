package ircclient

import (
	"testing"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// TestNewIRCClientRateLimitFromConfig verifies that NewIRCClient reads
// maxconns and ratelimit from TunnelConfig.Tunnel (the options map) rather
// than always using the hard-coded defaults.
//
// Because NewIRCClient requires a live SAM connection, this test validates the
// config extraction via the shared core helpers that the constructor calls.
// The constructor path is: config.Tunnel["maxconns"] -> TunnelOptionsMaxConns.
func TestNewIRCClientRateLimitFromConfig(t *testing.T) {
	tests := []struct {
		name         string
		tunnelOpts   map[string]interface{}
		wantMaxConns int
		wantRate     float64
	}{
		{
			name:         "defaults when options absent",
			tunnelOpts:   nil,
			wantMaxConns: 1000,
			wantRate:     100.0,
		},
		{
			name:         "custom maxconns from int",
			tunnelOpts:   map[string]interface{}{"maxconns": 50},
			wantMaxConns: 50,
			wantRate:     100.0,
		},
		{
			name:         "custom ratelimit from float64",
			tunnelOpts:   map[string]interface{}{"ratelimit": float64(20)},
			wantMaxConns: 1000,
			wantRate:     20.0,
		},
		{
			name:         "both custom values",
			tunnelOpts:   map[string]interface{}{"maxconns": 50, "ratelimit": float64(25)},
			wantMaxConns: 50,
			wantRate:     25.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := i2pconv.TunnelConfig{
				Name:   "test-ircclient",
				Type:   "ircclient",
				Target: "test.i2p",
				Tunnel: tt.tunnelOpts,
			}
			gotMax := i2ptunnel.TunnelOptionsMaxConns(config.Tunnel, 1000)
			gotRate := i2ptunnel.TunnelOptionsRateLimit(config.Tunnel, 100.0)
			if gotMax != tt.wantMaxConns {
				t.Errorf("MaxConns = %d, want %d", gotMax, tt.wantMaxConns)
			}
			if gotRate != tt.wantRate {
				t.Errorf("RateLimit = %f, want %f", gotRate, tt.wantRate)
			}
		})
	}
}

// TestIRCClientOptionsRoundTrip verifies that maxconns and ratelimit survive
// a SetOptions → Options round-trip.
func TestIRCClientOptionsRoundTrip(t *testing.T) {
	client := &IRCClient{}
	opts := map[string]string{
		"name":      "test-rt",
		"type":      "ircclient",
		"interface": "127.0.0.1",
		"port":      "6668",
		"maxconns":  "42",
		"ratelimit": "7.5",
	}
	if err := client.SetOptions(opts); err != nil {
		t.Fatalf("SetOptions failed: %v", err)
	}
	got := client.Options()
	if got["maxconns"] != "42" {
		t.Errorf("maxconns = %q, want %q", got["maxconns"], "42")
	}
	if got["ratelimit"] != "7.5" {
		t.Errorf("ratelimit = %q, want %q", got["ratelimit"], "7.5")
	}
}
