package udpserver

import (
	"net"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// NewUDPServer creates a new UDP Server tunnel with the given configuration
func NewUDPServer(config i2pconv.TunnelConfig, samAddr string) (*UDPServer, error) {
	garlic, err := i2ptunnel.NewGarlicFromConfig(config, samAddr)
	if err != nil {
		return nil, err
	}
	// Resolve the target address (the local service to forward I2P datagrams to)
	addr, err := net.ResolveUDPAddr("udp", config.Target)
	if err != nil {
		return nil, err
	}
	return &UDPServer{
		TunnelConfig:    config,
		Garlic:          garlic,
		Addr:            addr,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		done:            make(chan struct{}),
	}, nil
}
