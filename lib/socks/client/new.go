package socks

import (

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

func NewSocksClient(config i2pconv.TunnelConfig, samAddr string) (*SOCKS, error) {
	garlic, err := i2ptunnel.NewGarlicFromConfig(config, samAddr)
	if err != nil {
		return nil, err
	}
	return &SOCKS{
		TunnelConfig:    config,
		Garlic:          garlic,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		done:            make(chan struct{}),
	}, nil
}
