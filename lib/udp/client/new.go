package udpclient

import (
	"strings"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	"github.com/go-i2p/i2pkeys"
	"github.com/go-i2p/onramp"
)

// NewUDPClient creates a new UDP Client tunnel with the given configuration
func NewUDPClient(config i2pconv.TunnelConfig, samAddr string) (*UDPClient, error) {
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
	addr, err := i2pkeys.Lookup(config.Target)
	if err != nil {
		return nil, err
	}
	return &UDPClient{
		TunnelConfig: config,
		Garlic:       garlic,
		I2PAddr:      addr,
	}, nil
}
