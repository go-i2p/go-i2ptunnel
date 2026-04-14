package httpclient

/**
HTTP Outproxy
-------------

Routes clearnet (non-I2P) HTTP requests through an I2P outproxy service.
An outproxy is an I2P destination that accepts HTTP proxy requests and
forwards them to the clearnet internet, acting as an exit node.

```
[Browser] -> http://example.com/page
                    |
             [I2P HTTP Client Proxy]
                    |
         Is it *.i2p? ─── Yes ──> Dial via I2P (direct)
                    |
                   No (clearnet)
                    |
         Outproxy configured? ─── No ──> Reject with error page
                    |
                  Yes
                    |
         Dial outproxy's I2P address
                    |
         Forward HTTP request through I2P tunnel to outproxy
                    |
         Outproxy fetches clearnet content and returns it
```

This mirrors the Java I2P HTTP client tunnel's outproxy behavior.
The outproxy address is configurable via Options()/SetOptions().
When disabled, clearnet requests receive a friendly error.

Security note: Using an outproxy means the outproxy operator can see
your clearnet traffic. Only use trusted outproxies for sensitive traffic.
**/

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// Outproxy holds the configuration for routing clearnet requests through
// an I2P outproxy service. When enabled, non-I2P HTTP requests are tunneled
// through the specified I2P destination instead of being rejected.
//
// Thread safety: Outproxy is only modified via SetOptions() which is
// documented as not safe for concurrent use with Start().
type Outproxy struct {
	// Address is the I2P destination of the outproxy service.
	// Can be a human-readable name (e.g., "outproxy.i2p") or
	// a base32 address (e.g., "abc...xyz.b32.i2p").
	// Empty string disables outproxy routing.
	Address string

	// Enabled controls whether clearnet requests are forwarded.
	// When false, clearnet requests are rejected with an error.
	// Both Enabled and Address must be set for outproxy to work.
	Enabled bool
}

// IsActive reports whether the outproxy is configured and enabled.
func (o *Outproxy) IsActive() bool {
	return o != nil && o.Enabled && o.Address != ""
}

// IsI2PAddress reports whether the given host is an I2P network address.
// Returns true for any address ending in ".i2p" (including ".b32.i2p").
// Returns false for clearnet addresses, empty strings, and addresses
// that only happen to contain "i2p" elsewhere in the string.
func IsI2PAddress(host string) bool {
	host = stripPort(host)
	if host == "" {
		return false
	}
	return strings.HasSuffix(strings.ToLower(host), ".i2p")
}

// dialOutproxy connects to the configured outproxy's I2P address.
// The calling HTTP proxy (goproxy) will send the actual HTTP request
// through this connection, and the outproxy will forward it to the
// clearnet destination.
//
// Returns a descriptive error if the outproxy is not configured,
// allowing callers to present a user-friendly rejection message.
func (h *HTTPClient) dialOutproxy(ctx context.Context, network, addr string) (net.Conn, error) {
	if !h.Outproxy.IsActive() {
		return nil, fmt.Errorf(
			"clearnet address %s cannot be reached: no outproxy configured",
			stripPort(addr),
		)
	}

	// Build the outproxy dial address.
	// The outproxy is an I2P HTTP proxy, so we connect on port 80 by default.
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

// dialOutproxyNoCtx is the non-context version of dialOutproxy,
// used for goproxy's ConnectDial callback (HTTPS CONNECT method).
func (h *HTTPClient) dialOutproxyNoCtx(network, addr string) (net.Conn, error) {
	return h.dialOutproxy(context.Background(), network, addr)
}

// dialOutproxySnapshot connects to an outproxy using a pre-snapshotted Outproxy reference,
// avoiding a data race with SetOptions() which may replace h.Outproxy concurrently.
func dialOutproxySnapshot(outproxy *Outproxy, garlic interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}, network, addr string) (net.Conn, error) {
	if !outproxy.IsActive() {
		return nil, fmt.Errorf(
			"clearnet address %s cannot be reached: no outproxy configured",
			stripPort(addr),
		)
	}
	outproxyAddr := outproxy.Address
	if !strings.Contains(outproxyAddr, ":") {
		outproxyAddr = net.JoinHostPort(outproxyAddr, "80")
	}
	conn, err := garlic.DialContext(context.Background(), network, outproxyAddr)
	if err != nil {
		return nil, fmt.Errorf("outproxy dial failed (%s): %w", outproxy.Address, err)
	}
	return conn, nil
}
