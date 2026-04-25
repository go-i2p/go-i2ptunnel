package tcpserver

import (
	"testing"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// TestNewTCPServerRateLimitFromConfig verifies that NewTCPServer reads
// maxconns and ratelimit from TunnelConfig.Tunnel (the options map) rather
// than always using the hard-coded defaults.
//
// Because NewTCPServer requires a live SAM connection, this test validates the
// config extraction via the shared core helpers that the constructor calls.
// The constructor path is: config.Tunnel["maxconns"] -> TunnelOptionsMaxConns.
func TestNewTCPServerRateLimitFromConfig(t *testing.T) {
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
				Name:   "test-server",
				Type:   "tcpserver",
				Target: "127.0.0.1:8080",
				Tunnel: tt.tunnelOpts,
			}
			gotMax := i2ptunnel.TunnelOptionsMaxConns(config.Tunnel, defaultMaxConns)
			gotRate := i2ptunnel.TunnelOptionsRateLimit(config.Tunnel, defaultRateLimit)
			if gotMax != tt.wantMaxConns {
				t.Errorf("MaxConns = %d, want %d", gotMax, tt.wantMaxConns)
			}
			if gotRate != tt.wantRate {
				t.Errorf("RateLimit = %f, want %f", gotRate, tt.wantRate)
			}
		})
	}
}
