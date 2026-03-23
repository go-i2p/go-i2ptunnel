package tcpbidirectional

import (
	"fmt"
	"net"
	"strings"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/onramp"
)

// NewTCPBidirectional creates a new TCP bidirectional tunnel.
// It shares a single Garlic (I2P identity) for both the server-side forwarding
// and the client-side SOCKS5 proxy.
//
// config.Target is the local service address for inbound I2P connections.
// config.Port is the SOCKS5 proxy listen port for outbound connections.
func NewTCPBidirectional(config i2pconv.TunnelConfig, samAddr string) (*TCPBidirectional, error) {
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

	// Resolve the forward target address (where incoming I2P connections go)
	addr, err := net.ResolveTCPAddr("tcp", config.Target)
	if err != nil {
		return nil, fmt.Errorf("invalid target address %q: %w", config.Target, err)
	}

	return &TCPBidirectional{
		TunnelConfig:    config,
		Garlic:          garlic,
		Addr:            addr,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		LimitedConfig: limitedlistener.LimitedConfig{
			MaxConns:  i2ptunnel.TunnelOptionsMaxConns(config.Tunnel, 1000),
			RateLimit: i2ptunnel.TunnelOptionsRateLimit(config.Tunnel, 100.0),
		},
		done: make(chan struct{}),
	}, nil
}
