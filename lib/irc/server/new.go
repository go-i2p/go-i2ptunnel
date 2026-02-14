package ircserver

import (
	"fmt"
	"net"
	"strings"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/onramp"
)

// NewIRCServer creates a new IRC Server tunnel with the given configuration
func NewIRCServer(config i2pconv.TunnelConfig, samAddr string) (*IRCServer, error) {
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
		LimitedConfig: limitedlistener.LimitedConfig{
			MaxConns:  1000,
			RateLimit: 100,
		},
		done: make(chan struct{}),
	}, nil
}
