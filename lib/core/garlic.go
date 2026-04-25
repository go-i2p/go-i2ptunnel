package i2ptunnel

import (
	"strings"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
	"github.com/go-i2p/onramp"
)

// NewGarlicFromConfig creates a configured onramp.Garlic from a TunnelConfig
// and SAM bridge address. It loads keys from the config, sanitizes the name,
// creates the Garlic instance, and assigns the service keys. This helper
// eliminates the identical 4-line boilerplate repeated across all 12 tunnel
// constructors.
func NewGarlicFromConfig(config i2pconv.TunnelConfig, samAddr string) (*onramp.Garlic, error) {
	keys, options, err := config.SAMTunnel()
	if err != nil {
		return nil, err
	}
	name := strings.ReplaceAll(config.Name, " ", "_")
	garlic, err := onramp.NewGarlic(name, samAddr, options)
	if err != nil {
		return nil, err
	}
	garlic.ServiceKeys = keys
	return garlic, nil
}
