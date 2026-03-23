package i2ptunnel

import (
	"strconv"
)

// TunnelOptionsMaxConns extracts the maxconns value from a tunnel options map
// (TunnelConfig.Tunnel). Handles int, float64, and string types produced by
// different config parsers. Returns defaultVal when the key is absent or invalid.
func TunnelOptionsMaxConns(opts map[string]interface{}, defaultVal int) int {
	if opts == nil {
		return defaultVal
	}
	v, ok := opts["maxconns"]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case string:
		if parsed, err := strconv.Atoi(n); err == nil {
			return parsed
		}
	}
	return defaultVal
}

// TunnelOptionsRateLimit extracts the ratelimit value from a tunnel options map
// (TunnelConfig.Tunnel). Handles float64, int, and string types produced by
// different config parsers. Returns defaultVal when the key is absent or invalid.
func TunnelOptionsRateLimit(opts map[string]interface{}, defaultVal float64) float64 {
	if opts == nil {
		return defaultVal
	}
	v, ok := opts["ratelimit"]
	if !ok {
		return defaultVal
	}
	switch r := v.(type) {
	case float64:
		return r
	case int:
		return float64(r)
	case string:
		if parsed, err := strconv.ParseFloat(r, 64); err == nil {
			return parsed
		}
	}
	return defaultVal
}
