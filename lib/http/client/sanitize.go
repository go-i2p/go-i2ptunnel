package httpclient

import (
	"net"
	"net/http"

	httpinspector "github.com/go-i2p/go-connfilter/http"
)

// ApplyHTTPClientFilters wraps a listener with HTTP client-side filtering
// Removes potentially identifying headers from requests
func ApplyHTTPClientFilters(listener net.Listener) net.Listener {
	config := httpinspector.Config{
		OnRequest: func(req *http.Request) error {
			// Remove headers that could leak identity or system information
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
			// Only if explicitly set to empty or suspicious
			if ua := req.Header.Get("User-Agent"); ua == "" {
				req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; rv:102.0) Gecko/20100101 Firefox/102.0")
			}

			return nil
		},
		OnResponse: func(resp *http.Response) error {
			return nil
		},
	}

	return httpinspector.New(listener, config)
}
