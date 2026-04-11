// Package tcpserver implements a TCP server tunnel that accepts inbound I2P
// connections and forwards them to a local TCP service.
package tcpserver

import (
	"fmt"
	"net"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	limitedlistener "github.com/go-i2p/go-limit"
)

const (
	defaultMaxConns  = 1000
	defaultRateLimit = 100.0
)

// NewTCPServer creates a new TCP Server tunnel with the given configuration
func NewTCPServer(config i2pconv.TunnelConfig, samAddr string) (*TCPServer, error) {
	garlic, err := i2ptunnel.NewGarlicFromConfig(config, samAddr)
	if err != nil {
		return nil, err
	}
	// Resolve the forward target address (where incoming I2P connections are forwarded to)
	addr, err := net.ResolveTCPAddr("tcp", config.Target)
	if err != nil {
		return nil, fmt.Errorf("invalid target address %q: %w", config.Target, err)
	}
	return &TCPServer{
		TunnelConfig:    config,
		Garlic:          garlic,
		Addr:            addr,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		LimitedConfig: limitedlistener.LimitedConfig{
			MaxConns:  i2ptunnel.TunnelOptionsMaxConns(config.Tunnel, defaultMaxConns),
			RateLimit: i2ptunnel.TunnelOptionsRateLimit(config.Tunnel, defaultRateLimit),
		},
		done: make(chan struct{}),
	}, nil
}
