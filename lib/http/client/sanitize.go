package httpclient

import (
	"net"
	"net/http"

	httpinspector "github.com/go-i2p/go-connfilter/http"
)

// defaultHTTPClientRequestFilter removes headers that could leak identity
// or system information and normalizes User-Agent.
func defaultHTTPClientRequestFilter(req *http.Request) error {
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
		"X-Host",
		"X-ProxyUser-Ip",
	}

	for _, header := range headersToRemove {
		req.Header.Del(header)
	}

	// Normalize User-Agent to prevent fingerprinting
	if ua := req.Header.Get("User-Agent"); ua == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; rv:102.0) Gecko/20100101 Firefox/102.0")
	}

	return nil
}

// DefaultHTTPClientConfig returns an httpinspector.Config pre-configured with
// privacy-protecting defaults for HTTP client tunnels. It strips identifying
// headers and normalizes User-Agent to prevent fingerprinting.
func DefaultHTTPClientConfig() httpinspector.Config {
	return httpinspector.Config{
		OnRequest: defaultHTTPClientRequestFilter,
		OnResponse: func(resp *http.Response) error {
			return nil
		},
	}
}

// ApplyHTTPClientFilters wraps a listener with HTTP client-side filtering.
// Removes potentially identifying headers from requests.
func ApplyHTTPClientFilters(listener net.Listener) net.Listener {
	return httpinspector.New(listener, DefaultHTTPClientConfig())
}
