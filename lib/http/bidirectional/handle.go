package httpbidirectional

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"

	httpclient "github.com/go-i2p/go-i2ptunnel/lib/http/client"
)

// DialContext connects to the given address through the I2P network.
//
// Routing logic (mirrors the standalone HTTPClient for feature parity):
//   - I2P addresses (*.i2p): human-readable hostnames are resolved via the
//     jump service before dialing; base32 addresses are dialed directly.
//   - Clearnet addresses (non-.i2p): routed through the configured outproxy.
//     If no outproxy is configured or enabled, the request is rejected.
//
// Used by goproxy as the outbound transport for the HTTP proxy side of the
// bidirectional tunnel.
func (h *HTTPBidirectional) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, _ := net.SplitHostPort(addr)
	if host == "" {
		host = addr
	}
	if !httpclient.IsI2PAddress(host) {
		return h.dialOutproxy(ctx, network, addr)
	}
	addr = h.resolveJump(addr)
	return h.Garlic.DialContext(ctx, network, addr)
}

// Dial connects to the given address through the I2P network without a context.
// See DialContext for routing details.
func (h *HTTPBidirectional) Dial(network, addr string) (net.Conn, error) {
	return h.DialContext(context.Background(), network, addr)
}

// resolveJump attempts to resolve a human-readable .i2p hostname using the
// configured jump service. Base32 addresses (*.b32.i2p) are returned unchanged
// since SAM resolves them directly. Returns the original addr on any error so
// the caller can still attempt a dial.
func (h *HTTPBidirectional) resolveJump(addr string) string {
	if h.Jump == nil {
		return addr
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
		port = ""
	}
	if !httpclient.NeedsJump(host) {
		return addr
	}
	dest, err := h.Jump.Lookup(host)
	if err != nil {
		log.Printf("httpbidirectional: jump service lookup failed for %s: %v", host, err)
		return addr
	}
	if port != "" {
		return net.JoinHostPort(dest, port)
	}
	return dest
}

// dialOutproxy routes a clearnet address through the configured I2P outproxy.
// Returns an error if no outproxy is configured, giving the caller a clear
// message rather than a confusing SAM dial failure.
func (h *HTTPBidirectional) dialOutproxy(ctx context.Context, network, addr string) (net.Conn, error) {
	if h.Outproxy == nil || !h.Outproxy.IsActive() {
		return nil, fmt.Errorf(
			"clearnet address %s cannot be reached: no outproxy configured",
			stripPort(addr),
		)
	}
	outproxyAddr := h.Outproxy.Address
	if !strings.Contains(outproxyAddr, ":") {
		outproxyAddr = net.JoinHostPort(outproxyAddr, "80")
	}
	conn, err := h.Garlic.DialContext(ctx, network, outproxyAddr)
	if err != nil {
		return nil, fmt.Errorf("outproxy dial failed (%s): %w", h.Outproxy.Address, err)
	}
	return conn, nil
}

// stripPort removes the port from a host:port string, returning just the host.
// If addr has no port, it is returned unchanged.
func stripPort(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}
