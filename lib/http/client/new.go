package httpclient

import (
	"context"
	"net"
	"strings"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/onramp"
)

// NewHTTPClient creates a new HTTP Client tunnel with the given configuration
func NewHTTPClient(config i2pconv.TunnelConfig, samAddr string) (*HTTPClient, error) {
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
	h := &HTTPClient{
		TunnelConfig:    config,
		Garlic:          garlic,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		done:            make(chan struct{}),
	}
	return h, nil
}

func (h *HTTPClient) DialContext(ctx context.Context, network, addr string) (c net.Conn, err error) {
	return h.Garlic.DialContext(ctx, network, addr)
}

func (h *HTTPClient) Dial(network, addr string) (c net.Conn, err error) {
	return h.Garlic.Dial(network, addr)
}
