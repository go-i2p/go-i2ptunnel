// Package socks implements a SOCKS5 proxy client tunnel that routes local
// SOCKS5 connections through the I2P network.
package socks

import (
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	limitedlistener "github.com/go-i2p/go-limit"
)

// NewSocksClient creates a SOCKS5 tunnel from a TunnelConfig and SAM address.
func NewSocksClient(config i2pconv.TunnelConfig, samAddr string) (*SOCKS, error) {
	garlic, err := i2ptunnel.NewGarlicFromConfig(config, samAddr)
	if err != nil {
		return nil, err
	}
	maxConns := i2ptunnel.TunnelOptionsMaxConns(config.Tunnel, 1000)
	var connSem chan struct{}
	if maxConns > 0 {
		connSem = make(chan struct{}, maxConns)
	}
	return &SOCKS{
		TunnelConfig:    config,
		Garlic:          garlic,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		LimitedConfig: limitedlistener.LimitedConfig{
			MaxConns:  maxConns,
			RateLimit: i2ptunnel.TunnelOptionsRateLimit(config.Tunnel, 100.0),
		},
		connSem: connSem,
		done:    make(chan struct{}),
	}, nil
}
