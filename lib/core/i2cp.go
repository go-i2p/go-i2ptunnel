package i2ptunnel

import (
	"fmt"
	"strings"
)

// MergeI2CPOptions extracts I2CP options from a map[string]interface{} (as stored
// in TunnelConfig.I2CP) and adds them to the options map with an "i2cp." prefix.
//
// Why: Options() returns map[string]string for all tunnel configuration. I2CP options
// like leaseSetEncType, leaseSetType, leaseSetAuthType need to be surfaced so the
// Web UI and API consumers can read and display encrypted LeaseSet settings.
//
// Design: Uses a generic map[string]interface{} input to avoid coupling the core
// package to the go-i2ptunnel-config library. All values are converted to strings.
func MergeI2CPOptions(i2cp map[string]interface{}, opts map[string]string) {
	for k, v := range i2cp {
		opts["i2cp."+k] = fmt.Sprintf("%v", v)
	}
}

// ExtractI2CPOptions extracts keys with the "i2cp." prefix from an options map
// and returns them as a map[string]interface{} suitable for TunnelConfig.I2CP.
// Empty values are skipped to avoid storing blank entries.
// Returns nil if no I2CP options were found.
//
// Why: SetOptions() receives a flat map[string]string. This function separates
// I2CP-prefixed keys so they can be stored in the TunnelConfig.I2CP map, which
// is later consumed by SAMTunnel() to produce SAM session options.
//
// Design: Returns nil instead of empty map so callers can cheaply skip the
// assignment when no I2CP options are present.
func ExtractI2CPOptions(opts map[string]string) map[string]interface{} {
	i2cp := make(map[string]interface{})
	for k, v := range opts {
		if strings.HasPrefix(k, "i2cp.") && v != "" {
			i2cp[strings.TrimPrefix(k, "i2cp.")] = v
		}
	}
	if len(i2cp) == 0 {
		return nil
	}
	return i2cp
}
