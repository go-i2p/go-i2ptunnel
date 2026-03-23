package udpclient

import (

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/i2pkeys"
)

// NewUDPClient creates a new UDP Client tunnel with the given configuration
func NewUDPClient(config i2pconv.TunnelConfig, samAddr string) (*UDPClient, error) {
	garlic, err := i2ptunnel.NewGarlicFromConfig(config, samAddr)
	if err != nil {
		return nil, err
	}
	addr, err := i2pkeys.Lookup(config.Target)
	if err != nil {
		return nil, err
	}
	return &UDPClient{
		TunnelConfig:    config,
		Garlic:          garlic,
		I2PAddr:         addr,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		done:            make(chan struct{}),
	}, nil
}
