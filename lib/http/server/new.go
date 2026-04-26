package httpserver

import (
	"fmt"
	"net"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	limitedlistener "github.com/go-i2p/go-limit"
)

// NewHTTPServer creates a new HTTP Server tunnel with the given configuration
func NewHTTPServer(config i2pconv.TunnelConfig, samAddr string) (*HTTPServer, error) {
	garlic, err := i2ptunnel.NewGarlicFromConfig(config, samAddr)
	if err != nil {
		return nil, err
	}
	// Resolve the forward target address (where incoming I2P connections are forwarded to)
	addr, err := net.ResolveTCPAddr("tcp", config.Target)
	if err != nil {
		return nil, fmt.Errorf("invalid target address %q: %w", config.Target, err)
	}
	return &HTTPServer{
		TunnelBase: i2ptunnel.TunnelBase{
			TunnelConfig:    config,
			I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
			LimitedConfig: limitedlistener.LimitedConfig{
				MaxConns:  i2ptunnel.TunnelOptionsMaxConns(config.Tunnel, 1000),
				RateLimit: i2ptunnel.TunnelOptionsRateLimit(config.Tunnel, 100.0),
			},
		},
		Garlic: garlic,
		Addr:   addr,
		Config: DefaultHTTPServerConfig(),
		done:   make(chan struct{}),
	}, nil
}
