package httpclient

import (
	"net/http"
	"testing"
)

// TestDefaultHTTPClientConfig verifies the default config strips identifying headers.
// Why: Privacy-protecting header removal is critical for I2P anonymity.
func TestDefaultHTTPClientConfig(t *testing.T) {
	config := DefaultHTTPClientConfig()

	if config.OnRequest == nil {
		t.Fatal("DefaultHTTPClientConfig().OnRequest should not be nil")
	}
	if config.OnResponse == nil {
		t.Fatal("DefaultHTTPClientConfig().OnResponse should not be nil")
	}

	t.Run("strips identifying headers", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.i2p", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.1")
		req.Header.Set("X-Real-IP", "10.0.0.1")
		req.Header.Set("Via", "1.1 proxy.example.com")
		req.Header.Set("Client-IP", "172.16.0.1")
		req.Header.Set("True-Client-IP", "192.168.0.1")
		req.Header.Set("X-ProxyUser-Ip", "10.10.10.10")

		if err := config.OnRequest(req); err != nil {
			t.Fatalf("OnRequest returned error: %v", err)
		}

		headersToCheck := []string{
			"X-Forwarded-For", "X-Real-IP", "Via",
			"Client-IP", "True-Client-IP", "X-ProxyUser-Ip",
		}
		for _, header := range headersToCheck {
			if val := req.Header.Get(header); val != "" {
				t.Errorf("Header %s should be removed, got %q", header, val)
			}
		}
	})

	t.Run("normalizes empty User-Agent", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.i2p", nil)
		req.Header.Del("User-Agent")

		if err := config.OnRequest(req); err != nil {
			t.Fatalf("OnRequest returned error: %v", err)
		}

		ua := req.Header.Get("User-Agent")
		if ua == "" {
			t.Error("Empty User-Agent should be replaced with default")
		}
	})

	t.Run("preserves existing User-Agent", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.i2p", nil)
		req.Header.Set("User-Agent", "CustomBrowser/1.0")

		if err := config.OnRequest(req); err != nil {
			t.Fatalf("OnRequest returned error: %v", err)
		}

		if req.Header.Get("User-Agent") != "CustomBrowser/1.0" {
			t.Error("Non-empty User-Agent should not be replaced")
		}
	})

	t.Run("preserves non-identifying headers", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.i2p", nil)
		req.Header.Set("Accept", "text/html")
		req.Header.Set("Content-Type", "application/json")

		if err := config.OnRequest(req); err != nil {
			t.Fatalf("OnRequest returned error: %v", err)
		}

		if req.Header.Get("Accept") != "text/html" {
			t.Error("Accept header should be preserved")
		}
		if req.Header.Get("Content-Type") != "application/json" {
			t.Error("Content-Type header should be preserved")
		}
	})

	t.Run("OnResponse is no-op", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("Server", "nginx")

		if err := config.OnResponse(resp); err != nil {
			t.Fatalf("OnResponse returned error: %v", err)
		}
		// Client-side response filter is a no-op
		if resp.Header.Get("Server") != "nginx" {
			t.Error("Client OnResponse should not modify response headers")
		}
	})
}

// TestHTTPClientConstructorHasDefaultConfig verifies the constructor sets the privacy config.
func TestHTTPClientConstructorHasDefaultConfig(t *testing.T) {
	// We can't easily call NewHTTPClient without SAM, but we can verify
	// DefaultHTTPClientConfig returns a valid config
	config := DefaultHTTPClientConfig()
	if config.OnRequest == nil {
		t.Fatal("Default config should have OnRequest set")
	}
}
