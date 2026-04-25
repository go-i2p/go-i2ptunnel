// Package ircserver implements an IRC server tunnel that forwards I2P
// connections to a local IRC server with admin command filtering and
// hostname masking.
package ircserver

import (
	"fmt"
	"net"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	limitedlistener "github.com/go-i2p/go-limit"
)

// NewIRCServer creates a new IRC Server tunnel with the given configuration
func NewIRCServer(config i2pconv.TunnelConfig, samAddr string) (*IRCServer, error) {
	garlic, err := i2ptunnel.NewGarlicFromConfig(config, samAddr)
	if err != nil {
		return nil, err
	}
	// Resolve the forward target address (where incoming I2P connections are forwarded to)
	addr, err := net.ResolveTCPAddr("tcp", config.Target)
	if err != nil {
		return nil, fmt.Errorf("invalid target address %q: %w", config.Target, err)
	}
	return &IRCServer{
		TunnelConfig:    config,
		Garlic:          garlic,
		Addr:            addr,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		Config:          DefaultIRCServerConfig(),
		LimitedConfig: limitedlistener.LimitedConfig{
			MaxConns:  i2ptunnel.TunnelOptionsMaxConns(config.Tunnel, 1000),
			RateLimit: i2ptunnel.TunnelOptionsRateLimit(config.Tunnel, 100.0),
		},
		done: make(chan struct{}),
	}, nil
}
