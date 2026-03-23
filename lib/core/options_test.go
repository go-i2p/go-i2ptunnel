package i2ptunnel

import (
	"testing"
)

// TestTunnelOptionsMaxConns verifies MaxConns extraction from tunnel options map.
func TestTunnelOptionsMaxConns(t *testing.T) {
	tests := []struct {
		name       string
		opts       map[string]interface{}
		defaultVal int
		want       int
	}{
		{"nil map uses default", nil, 1000, 1000},
		{"absent key uses default", map[string]interface{}{}, 1000, 1000},
		{"int value", map[string]interface{}{"maxconns": 50}, 1000, 50},
		{"float64 value", map[string]interface{}{"maxconns": float64(75)}, 1000, 75},
		{"string value", map[string]interface{}{"maxconns": "200"}, 1000, 200},
		{"invalid string uses default", map[string]interface{}{"maxconns": "bad"}, 1000, 1000},
		{"zero int", map[string]interface{}{"maxconns": 0}, 1000, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TunnelOptionsMaxConns(tt.opts, tt.defaultVal)
			if got != tt.want {
				t.Errorf("TunnelOptionsMaxConns(%v, %d) = %d, want %d", tt.opts, tt.defaultVal, got, tt.want)
			}
		})
	}
}

// TestTunnelOptionsRateLimit verifies RateLimit extraction from tunnel options map.
func TestTunnelOptionsRateLimit(t *testing.T) {
	tests := []struct {
		name       string
		opts       map[string]interface{}
		defaultVal float64
		want       float64
	}{
		{"nil map uses default", nil, 100, 100},
		{"absent key uses default", map[string]interface{}{}, 100, 100},
		{"float64 value", map[string]interface{}{"ratelimit": float64(20)}, 100, 20},
		{"int value", map[string]interface{}{"ratelimit": 50}, 100, 50},
		{"string value", map[string]interface{}{"ratelimit": "33.5"}, 100, 33.5},
		{"invalid string uses default", nil, 100, 100},
		{"zero float", map[string]interface{}{"ratelimit": float64(0)}, 100, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TunnelOptionsRateLimit(tt.opts, tt.defaultVal)
			if got != tt.want {
				t.Errorf("TunnelOptionsRateLimit(%v, %f) = %f, want %f", tt.opts, tt.defaultVal, got, tt.want)
			}
		})
	}
}
