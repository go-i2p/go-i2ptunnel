package httpserver

import (
	"net"
	"net/http"
	"net/url"

	httpinspector "github.com/go-i2p/go-connfilter/http"
)

// defaultHTTPServerRequestFilter removes headers that could expose client information.
func defaultHTTPServerRequestFilter(req *http.Request) error {
	headersToRemove := []string{
		"X-Forwarded-For",
		"X-Real-IP",
		"Via",
		"X-Forwarded-Host",
		"X-Forwarded-Proto",
		"Forwarded",
		"Client-IP",
		"True-Client-IP",
		"X-Client-IP",
		"CF-Connecting-IP",
		"CF-IPCountry",
	}

	for _, header := range headersToRemove {
		req.Header.Del(header)
	}

	// Sanitize referer to prevent information leakage
	if referer := req.Header.Get("Referer"); referer != "" {
		if req.URL != nil && !isSameOrigin(referer, req.URL.String()) {
			req.Header.Del("Referer")
		}
	}

	return nil
}

// defaultHTTPServerResponseFilter removes server fingerprinting headers and
// adds privacy-preserving headers.
func defaultHTTPServerResponseFilter(resp *http.Response) error {
	resp.Header.Del("Server")
	resp.Header.Del("X-Powered-By")
	resp.Header.Del("X-AspNet-Version")
	resp.Header.Del("X-AspNetMvc-Version")
	resp.Header.Del("X-Runtime")

	if resp.Header.Get("X-Frame-Options") == "" {
		resp.Header.Set("X-Frame-Options", "SAMEORIGIN")
	}
	if resp.Header.Get("X-Content-Type-Options") == "" {
		resp.Header.Set("X-Content-Type-Options", "nosniff")
	}

	return nil
}

// DefaultHTTPServerConfig returns an httpinspector.Config pre-configured with
// privacy-protecting defaults for HTTP server tunnels. It strips identifying
// headers from requests and removes server fingerprinting headers from responses.
func DefaultHTTPServerConfig() httpinspector.Config {
	return httpinspector.Config{
		OnRequest:  defaultHTTPServerRequestFilter,
		OnResponse: defaultHTTPServerResponseFilter,
	}
}

// ApplyHTTPServerFilters wraps a listener with HTTP server-side filtering.
// Sanitizes requests and responses for privacy and security.
func ApplyHTTPServerFilters(listener net.Listener) net.Listener {
	return httpinspector.New(listener, DefaultHTTPServerConfig())
}

// isSameOrigin checks if two URLs share the same origin (scheme + host + port)
func isSameOrigin(raw1, raw2 string) bool {
	u1, err1 := url.Parse(raw1)
	u2, err2 := url.Parse(raw2)
	if err1 != nil || err2 != nil {
		return false
	}
	return u1.Scheme == u2.Scheme && u1.Host == u2.Host
}
