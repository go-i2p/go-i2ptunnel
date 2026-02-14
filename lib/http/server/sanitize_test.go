package httpserver

import (
	"net/http"
	"testing"
)

// TestDefaultHTTPServerConfig verifies the default config strips fingerprinting headers.
// Why: Server-side filtering prevents exposing backend technology to I2P clients.
func TestDefaultHTTPServerConfig(t *testing.T) {
	config := DefaultHTTPServerConfig()

	if config.OnRequest == nil {
		t.Fatal("DefaultHTTPServerConfig().OnRequest should not be nil")
	}
	if config.OnResponse == nil {
		t.Fatal("DefaultHTTPServerConfig().OnResponse should not be nil")
	}

	t.Run("strips identifying request headers", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.i2p", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.1")
		req.Header.Set("X-Real-IP", "10.0.0.1")
		req.Header.Set("CF-Connecting-IP", "1.2.3.4")
		req.Header.Set("CF-IPCountry", "US")

		if err := config.OnRequest(req); err != nil {
			t.Fatalf("OnRequest returned error: %v", err)
		}

		headersToCheck := []string{
			"X-Forwarded-For", "X-Real-IP", "CF-Connecting-IP", "CF-IPCountry",
		}
		for _, header := range headersToCheck {
			if val := req.Header.Get(header); val != "" {
				t.Errorf("Header %s should be removed, got %q", header, val)
			}
		}
	})

	t.Run("strips cross-origin referer", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.i2p/page", nil)
		req.Header.Set("Referer", "http://other-site.i2p/secret")

		if err := config.OnRequest(req); err != nil {
			t.Fatalf("OnRequest returned error: %v", err)
		}

		if req.Header.Get("Referer") != "" {
			t.Error("Cross-origin Referer should be stripped")
		}
	})

	t.Run("preserves same-origin referer", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.i2p/page2", nil)
		req.Header.Set("Referer", "http://example.i2p/page1")

		if err := config.OnRequest(req); err != nil {
			t.Fatalf("OnRequest returned error: %v", err)
		}

		if req.Header.Get("Referer") != "http://example.i2p/page1" {
			t.Error("Same-origin Referer should be preserved")
		}
	})

	t.Run("removes server fingerprinting headers", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("Server", "nginx/1.24")
		resp.Header.Set("X-Powered-By", "PHP/8.2")
		resp.Header.Set("X-AspNet-Version", "4.0.30319")
		resp.Header.Set("X-Runtime", "0.123456")

		if err := config.OnResponse(resp); err != nil {
			t.Fatalf("OnResponse returned error: %v", err)
		}

		headersToCheck := []string{"Server", "X-Powered-By", "X-AspNet-Version", "X-Runtime"}
		for _, header := range headersToCheck {
			if val := resp.Header.Get(header); val != "" {
				t.Errorf("Header %s should be removed, got %q", header, val)
			}
		}
	})

	t.Run("adds security headers when missing", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}

		if err := config.OnResponse(resp); err != nil {
			t.Fatalf("OnResponse returned error: %v", err)
		}

		if resp.Header.Get("X-Frame-Options") != "SAMEORIGIN" {
			t.Error("X-Frame-Options should default to SAMEORIGIN")
		}
		if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Error("X-Content-Type-Options should default to nosniff")
		}
	})

	t.Run("preserves existing security headers", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("X-Frame-Options", "DENY")
		resp.Header.Set("X-Content-Type-Options", "nosniff")

		if err := config.OnResponse(resp); err != nil {
			t.Fatalf("OnResponse returned error: %v", err)
		}

		if resp.Header.Get("X-Frame-Options") != "DENY" {
			t.Error("Existing X-Frame-Options should be preserved")
		}
	})
}

// TestIsSameOrigin tests the origin comparison helper.
func TestIsSameOrigin(t *testing.T) {
	tests := []struct {
		name   string
		url1   string
		url2   string
		expect bool
	}{
		{"same origin", "http://example.i2p/page1", "http://example.i2p/page2", true},
		{"different host", "http://a.i2p/x", "http://b.i2p/x", false},
		{"different scheme", "http://a.i2p/x", "https://a.i2p/x", false},
		{"invalid URL", "not-a-url", "http://a.i2p", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSameOrigin(tt.url1, tt.url2)
			if got != tt.expect {
				t.Errorf("isSameOrigin(%q, %q) = %v, want %v", tt.url1, tt.url2, got, tt.expect)
			}
		})
	}
}
