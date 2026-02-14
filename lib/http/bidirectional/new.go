package httpbidirectional

import (
	"fmt"
	"net"
	"strings"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/onramp"
)

// NewHTTPBidirectional creates a new HTTP bidirectional tunnel.
// It shares a single Garlic (I2P identity) for both the server-side HTTP
// forwarding and the client-side SOCKS5 proxy.
//
// config.Target is the local HTTP service address for inbound I2P connections.
// config.Port is the SOCKS5 proxy listen port for outbound connections.
func NewHTTPBidirectional(config i2pconv.TunnelConfig, samAddr string) (*HTTPBidirectional, error) {
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

	return &HTTPBidirectional{
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
