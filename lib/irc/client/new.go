package ircclient

import (
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/i2pkeys"
)

// NewIRCClient creates a new IRC Client tunnel with the given configuration
func NewIRCClient(config i2pconv.TunnelConfig, samAddr string) (*IRCClient, error) {
	garlic, err := i2ptunnel.NewGarlicFromConfig(config, samAddr)
	if err != nil {
		return nil, err
	}
	addr, err := i2pkeys.Lookup(config.Target)
	if err != nil {
		return nil, err
	}
	return &IRCClient{
		TunnelConfig:    config,
		Garlic:          garlic,
		I2PAddr:         addr,
		Config:          DefaultIRCClientConfig(),
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		LimitedConfig: limitedlistener.LimitedConfig{
			MaxConns:  i2ptunnel.TunnelOptionsMaxConns(config.Tunnel, 1000),
			RateLimit: i2ptunnel.TunnelOptionsRateLimit(config.Tunnel, 100.0),
		},
		done: make(chan struct{}),
	}, nil
}
