package i2ptunnel

import (
	"fmt"
	"strconv"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
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

// BuildCommonOptions creates the standard options map from TunnelConfig fields.
// All 12 tunnel types share these four core fields plus I2CP options.
func BuildCommonOptions(cfg i2pconv.TunnelConfig) map[string]string {
	opts := make(map[string]string)
	opts["name"] = cfg.Name
	opts["type"] = cfg.Type
	opts["interface"] = cfg.Interface
	opts["port"] = strconv.Itoa(cfg.Port)
	MergeI2CPOptions(cfg.I2CP, opts)
	return opts
}

// AddRateLimitOptions adds maxconns and ratelimit entries to an existing options map.
func AddRateLimitOptions(opts map[string]string, maxConns int, rateLimit float64) {
	opts["maxconns"] = strconv.Itoa(maxConns)
	opts["ratelimit"] = strconv.FormatFloat(rateLimit, 'f', -1, 64)
}

// ApplyCommonOptions validates and applies the standard tunnel options
// (name, interface, port) from an options map to a TunnelConfig.
// I2CP-prefixed options are also extracted and merged.
func ApplyCommonOptions(opts map[string]string, cfg *i2pconv.TunnelConfig) error {
	if name, ok := opts["name"]; ok {
		if err := validate.RequiredString("name", name); err != nil {
			return err
		}
		cfg.Name = name
	}
	if iface, ok := opts["interface"]; ok {
		if err := validate.Interface(iface); err != nil {
			return err
		}
		cfg.Interface = iface
	}
	if portStr, ok := opts["port"]; ok {
		port, err := validate.PortString(portStr)
		if err != nil {
			return err
		}
		cfg.Port = port
	}
	if i2cpOpts := ExtractI2CPOptions(opts); i2cpOpts != nil {
		if cfg.I2CP == nil {
			cfg.I2CP = make(map[string]interface{})
		}
		for k, v := range i2cpOpts {
			cfg.I2CP[k] = v
		}
	}
	return nil
}

// ApplyRateLimitOptions validates and applies maxconns and ratelimit from an
// options map via pointers. Callers pass &LimitedConfig.MaxConns and
// &LimitedConfig.RateLimit (or equivalent fields).
func ApplyRateLimitOptions(opts map[string]string, maxConns *int, rateLimit *float64) error {
	if maxconnsStr, ok := opts["maxconns"]; ok {
		mc, err := strconv.Atoi(maxconnsStr)
		if err != nil {
			return fmt.Errorf("invalid maxconns value: %s", maxconnsStr)
		}
		if err := validate.MaxConnections(mc); err != nil {
			return err
		}
		*maxConns = mc
	}
	if ratelimitStr, ok := opts["ratelimit"]; ok {
		rl, err := validate.RateLimitString(ratelimitStr)
		if err != nil {
			return err
		}
		*rateLimit = rl
	}
	return nil
}
