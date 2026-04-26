package i2ptunnel

import (
	"fmt"
	"os"
	"strconv"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
)

// ParseConfigFile reads, detects the format of, and parses a tunnel config file.
// It is the common boilerplate shared by every tunnel's LoadConfig implementation.
// Returns the parsed TunnelConfig, or a descriptive error.
func ParseConfigFile(path string) (*i2pconv.TunnelConfig, error) {
	conv := i2pconv.Converter{}
	format, err := conv.DetectFormat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to detect config format: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	cfg, err := conv.ParseInput(data, format)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return cfg, nil
}

// CheckTunnelStopped returns an error when status is Running or Starting.
// Call this at the top of every LoadConfig implementation to guard against
// configuration changes while the tunnel is active.
func CheckTunnelStopped(status I2PTunnelStatus) error {
	if status == I2PTunnelStatusRunning || status == I2PTunnelStatusStarting {
		return fmt.Errorf("cannot load config while tunnel is %s - stop tunnel first", status)
	}
	return nil
}

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
	if err := applyNameOption(opts, cfg); err != nil {
		return err
	}
	if err := applyInterfaceOption(opts, cfg); err != nil {
		return err
	}
	if err := applyPortOption(opts, cfg); err != nil {
		return err
	}
	mergeI2CPOptionsInto(opts, cfg)
	return nil
}

// applyNameOption validates and applies the "name" key from opts to cfg.
func applyNameOption(opts map[string]string, cfg *i2pconv.TunnelConfig) error {
	if name, ok := opts["name"]; ok {
		if err := validate.RequiredString("name", name); err != nil {
			return err
		}
		cfg.Name = name
	}
	return nil
}

// applyInterfaceOption validates and applies the "interface" key from opts to cfg.
func applyInterfaceOption(opts map[string]string, cfg *i2pconv.TunnelConfig) error {
	if iface, ok := opts["interface"]; ok {
		if err := validate.Interface(iface); err != nil {
			return err
		}
		cfg.Interface = iface
	}
	return nil
}

// applyPortOption validates and applies the "port" key from opts to cfg.
func applyPortOption(opts map[string]string, cfg *i2pconv.TunnelConfig) error {
	if portStr, ok := opts["port"]; ok {
		port, err := validate.PortString(portStr)
		if err != nil {
			return err
		}
		cfg.Port = port
	}
	return nil
}

// mergeI2CPOptionsInto extracts i2cp.* keys from opts and merges them into cfg.I2CP.
func mergeI2CPOptionsInto(opts map[string]string, cfg *i2pconv.TunnelConfig) {
	if i2cpOpts := ExtractI2CPOptions(opts); i2cpOpts != nil {
		if cfg.I2CP == nil {
			cfg.I2CP = make(map[string]interface{})
		}
		for k, v := range i2cpOpts {
			cfg.I2CP[k] = v
		}
	}
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
