package httpbidirectional

import (
	"context"
	"net"
)

// DialContext dials an I2P destination through the shared Garlic instance.
// Used by goproxy as the outbound transport for the HTTP proxy.
func (h *HTTPBidirectional) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return h.Garlic.DialContext(ctx, network, addr)
}

// Dial dials an I2P destination through the shared Garlic instance.
func (h *HTTPBidirectional) Dial(network, addr string) (net.Conn, error) {
	return h.Garlic.Dial(network, addr)
}
