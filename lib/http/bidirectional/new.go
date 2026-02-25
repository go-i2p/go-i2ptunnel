package httpbidirectional

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	tunnelconfig "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	httpClientSanitize "github.com/go-i2p/go-i2ptunnel/lib/http/client"
	httpServerSanitize "github.com/go-i2p/go-i2ptunnel/lib/http/server"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/onramp"
)

// NewHTTPBidirectional creates a new HTTP bidirectional tunnel.
// It shares a single Garlic (I2P identity) for both the server-side HTTP
// forwarding and the client-side HTTP proxy.
//
// config.Target is the local HTTP service address for inbound I2P connections.
// config.Port is the HTTP proxy listen port for outbound connections.
func NewHTTPBidirectional(config tunnelconfig.TunnelConfig, samAddr string) (*HTTPBidirectional, error) {
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

	// Create an I2P-routed HTTP client so the jump service (at stats.i2p) is
	// reachable. Without this, human-readable .i2p hostnames would fail to
	// resolve because http.DefaultClient cannot reach I2P network addresses.
	jumpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: garlic.DialContext,
		},
		Timeout: 30 * time.Second,
	}

	return &HTTPBidirectional{
		TunnelConfig:    config,
		Garlic:          garlic,
		Addr:            addr,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		ServerConfig:    httpServerSanitize.DefaultHTTPServerConfig(),
		ClientConfig:    httpClientSanitize.DefaultHTTPClientConfig(),
		LimitedConfig: limitedlistener.LimitedConfig{
			MaxConns:  1000,
			RateLimit: 100,
		},
		Jump:     httpClientSanitize.NewJumpService(jumpClient, httpClientSanitize.DefaultJumpServiceURL),
		Outproxy: &httpClientSanitize.Outproxy{},
		done:     make(chan struct{}),
	}, nil
}
