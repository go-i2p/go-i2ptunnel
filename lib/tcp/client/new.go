package tcpclient

import (

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/i2pkeys"
)

// NewTCPClient creates a new TCP Client tunnel with the given configuration
func NewTCPClient(config i2pconv.TunnelConfig, samAddr string) (*TCPClient, error) {
	garlic, err := i2ptunnel.NewGarlicFromConfig(config, samAddr)
	if err != nil {
		return nil, err
	}
	addr, err := i2pkeys.Lookup(config.Target)
	if err != nil {
		return nil, err
	}
	return &TCPClient{
		TunnelConfig:    config,
		Garlic:          garlic,
		I2PAddr:         addr,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		done:            make(chan struct{}),
		dialTimeout:     defaultDialTimeout,
	}, nil
}
