package udpbidirectional

import (
	"fmt"
	"net"
	"strings"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/onramp"
)

// NewUDPBidirectional creates a new UDP bidirectional tunnel.
// It shares a single Garlic (I2P identity) for both the server-side datagram
// forwarding and the client-side SOCKS5 proxy.
//
// config.Target is the local service address for inbound I2P datagrams.
// config.Port is the SOCKS5 proxy listen port for outbound connections.
func NewUDPBidirectional(config i2pconv.TunnelConfig, samAddr string) (*UDPBidirectional, error) {
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

	// Resolve the forward target address (where incoming I2P datagrams go)
	addr, err := net.ResolveUDPAddr("udp", config.Target)
	if err != nil {
		return nil, fmt.Errorf("invalid target address %q: %w", config.Target, err)
	}

	return &UDPBidirectional{
		TunnelConfig:    config,
		Garlic:          garlic,
		Addr:            addr,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		done:            make(chan struct{}),
	}, nil
}
