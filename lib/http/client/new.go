package httpclient

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

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
	// Create an HTTP client that routes through I2P so the jump service
	// (which lives at stats.i2p) can be reached. Without this, lookups
	// for human-readable .i2p hostnames would fail with DNS errors.
	jumpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: garlic.DialContext,
		},
		Timeout: 30 * time.Second,
	}
	h := &HTTPClient{
		TunnelConfig:    config,
		Garlic:          garlic,
		Config:          DefaultHTTPClientConfig(),
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		done:            make(chan struct{}),
		Jump:            NewJumpService(jumpClient, DefaultJumpServiceURL),
		Outproxy:        &Outproxy{},
	}
	return h, nil
}

func (h *HTTPClient) DialContext(ctx context.Context, network, addr string) (c net.Conn, err error) {
	host, _, _ := net.SplitHostPort(addr)
	if host == "" {
		host = addr
	}
	// Clearnet addresses (non-.i2p) are routed through the outproxy.
	// I2P addresses are dialed directly after jump service resolution.
	if !IsI2PAddress(host) {
		return h.dialOutproxy(ctx, network, addr)
	}
	// Resolve human-readable .i2p hostnames via jump service before dialing.
	// Base32 addresses (*.b32.i2p) bypass this — SAM handles them directly.
	addr = h.resolveJump(addr)
	return h.Garlic.DialContext(ctx, network, addr)
}

func (h *HTTPClient) Dial(network, addr string) (c net.Conn, err error) {
	host, _, _ := net.SplitHostPort(addr)
	if host == "" {
		host = addr
	}
	// Clearnet addresses route through outproxy; I2P addresses dial directly.
	if !IsI2PAddress(host) {
		return h.dialOutproxy(context.Background(), network, addr)
	}
	// Resolve human-readable .i2p hostnames via jump service before dialing.
	addr = h.resolveJump(addr)
	return h.Garlic.Dial(network, addr)
}

// resolveJump attempts to resolve a human-readable .i2p hostname using
// the configured jump service. If the jump service is disabled or the
// hostname doesn't need resolution, the original address is returned.
func (h *HTTPClient) resolveJump(addr string) string {
	if h.Jump == nil {
		return addr
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
		port = ""
	}
	if !NeedsJump(host) {
		return addr
	}
	dest, err := h.Jump.Lookup(host)
	if err != nil {
		log.Printf("jump service lookup failed for %s: %v", host, err)
		return addr
	}
	if port != "" {
		return net.JoinHostPort(dest, port)
	}
	return dest
}
