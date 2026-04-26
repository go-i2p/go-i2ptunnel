package httpclient

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// NewHTTPClient creates a new HTTP Client tunnel with the given configuration
func NewHTTPClient(config i2pconv.TunnelConfig, samAddr string) (*HTTPClient, error) {
	garlic, err := i2ptunnel.NewGarlicFromConfig(config, samAddr)
	if err != nil {
		return nil, err
	}
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
		TunnelBase: i2ptunnel.TunnelBase{
			TunnelConfig:    config,
			I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		},
		Garlic:   garlic,
		Config:   DefaultHTTPClientConfig(),
		done:     make(chan struct{}),
		Jump:     NewJumpService(jumpClient, DefaultJumpServiceURL),
		Outproxy: &Outproxy{},
	}
	return h, nil
}

// DialContext connects to addr over the I2P network, routing clearnet
// addresses through the outproxy when configured.
func (h *HTTPClient) DialContext(ctx context.Context, network, addr string) (c net.Conn, err error) {
	host, _, _ := net.SplitHostPort(addr)
	if host == "" {
		host = addr
	}
	// Snapshot fields under fieldsMu to prevent races with SetOptions.
	h.fieldsMu.RLock()
	jump := h.Jump
	outproxy := h.Outproxy
	h.fieldsMu.RUnlock()
	// Clearnet addresses (non-.i2p) are routed through the outproxy.
	// I2P addresses are dialed directly after jump service resolution.
	if !IsI2PAddress(host) {
		return dialOutproxySnapshot(outproxy, h.Garlic, network, addr)
	}
	// Resolve human-readable .i2p hostnames via jump service before dialing.
	// Base32 addresses (*.b32.i2p) bypass this — SAM handles them directly.
	addr = resolveJumpSnapshot(jump, addr)
	return h.Garlic.DialContext(ctx, network, addr)
}

// Dial connects to addr over the I2P network without an explicit context.
func (h *HTTPClient) Dial(network, addr string) (c net.Conn, err error) {
	host, _, _ := net.SplitHostPort(addr)
	if host == "" {
		host = addr
	}
	// Snapshot fields under fieldsMu to prevent races with SetOptions.
	h.fieldsMu.RLock()
	jump := h.Jump
	outproxy := h.Outproxy
	h.fieldsMu.RUnlock()
	// Clearnet addresses route through outproxy; I2P addresses dial directly.
	if !IsI2PAddress(host) {
		return dialOutproxySnapshot(outproxy, h.Garlic, network, addr)
	}
	// Resolve human-readable .i2p hostnames via jump service before dialing.
	addr = resolveJumpSnapshot(jump, addr)
	return h.Garlic.Dial(network, addr)
}

// resolveJump attempts to resolve a human-readable .i2p hostname using
// the configured jump service. If the jump service is disabled or the
// hostname doesn't need resolution, the original address is returned.
func (h *HTTPClient) resolveJump(addr string) string {
	h.fieldsMu.RLock()
	jump := h.Jump
	h.fieldsMu.RUnlock()
	return resolveJumpSnapshot(jump, addr)
}

// resolveJumpSnapshot resolves a .i2p hostname using a pre-snapshotted JumpService
// reference, avoiding a data race with SetOptions().
func resolveJumpSnapshot(jump *JumpService, addr string) string {
	if jump == nil {
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
	dest, err := jump.Lookup(host)
	if err != nil {
		log.Printf("jump service lookup failed for %s: %v", host, err)
		return addr
	}
	if port != "" {
		return net.JoinHostPort(dest, port)
	}
	return dest
}
