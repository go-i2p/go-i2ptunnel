package socks

import (
	"testing"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// TestNewSocksClientRateLimitFromConfig verifies that NewSocksClient reads
// maxconns and ratelimit from TunnelConfig.Tunnel (the options map) rather
// than always using the hard-coded defaults.
//
// Because NewSocksClient requires a live SAM connection, this test validates
// the config extraction via the shared core helpers that the constructor calls.
func TestNewSocksClientRateLimitFromConfig(t *testing.T) {
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
				Name:   "test-socks",
				Type:   "socks",
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

// TestSOCKSOptionsRoundTrip verifies that maxconns and ratelimit survive
// a SetOptions → Options round-trip.
func TestSOCKSOptionsRoundTrip(t *testing.T) {
	s := &SOCKS{}
	opts := map[string]string{
		"name":      "test-rt",
		"type":      "socks",
		"interface": "127.0.0.1",
		"port":      "1080",
		"maxconns":  "25",
		"ratelimit": "5.5",
	}
	if err := s.SetOptions(opts); err != nil {
		t.Fatalf("SetOptions failed: %v", err)
	}
	got := s.Options()
	if got["maxconns"] != "25" {
		t.Errorf("maxconns = %q, want %q", got["maxconns"], "25")
	}
	if got["ratelimit"] != "5.5" {
		t.Errorf("ratelimit = %q, want %q", got["ratelimit"], "5.5")
	}
}
