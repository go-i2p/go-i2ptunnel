package httpclient

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
)

// TestNeedsJump tests detection of hostnames requiring jump service resolution.
// Why: Correctly distinguishing resolvable vs unresolvable hostnames prevents
// unnecessary jump service queries and ensures base32 addresses are dialed directly.
func TestNeedsJump(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		// Hostnames that NEED jump resolution (human-readable .i2p)
		{"forum.i2p", true},
		{"stats.i2p", true},
		{"my-site.i2p", true},
		{"Forum.I2P", true},    // case insensitive
		{"STATS.I2P", true},    // all caps
		{"a.b.c.i2p", true},    // subdomain
		{"forum.i2p:80", true}, // with port

		// Hostnames that DON'T need jump resolution
		{"abc123def456.b32.i2p", false},  // base32 address
		{"ABC123DEF456.B32.I2P", false},  // base32 uppercase
		{"test.b32.i2p:8080", false},     // base32 with port
		{"example.com", false},           // clearnet
		{"localhost", false},             // local
		{"127.0.0.1", false},             // IP address
		{"", false},                      // empty
		{"not-i2p-at-all", false},        // no .i2p suffix
		{"forum.i2p.example.com", false}, // .i2p in middle
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			got := NeedsJump(tt.host)
			if got != tt.want {
				t.Errorf("NeedsJump(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

// TestJumpServiceCreation tests NewJumpService constructor behavior.
// Why: Validates default values and parameter handling.
func TestJumpServiceCreation(t *testing.T) {
	t.Run("default URL when empty", func(t *testing.T) {
		js := NewJumpService(nil, "")
		if js.URL() != DefaultJumpServiceURL {
			t.Errorf("expected default URL %q, got %q", DefaultJumpServiceURL, js.URL())
		}
	})

	t.Run("custom URL", func(t *testing.T) {
		js := NewJumpService(nil, "http://custom.i2p/jump")
		if js.URL() != "http://custom.i2p/jump" {
			t.Errorf("expected custom URL, got %q", js.URL())
		}
	})

	t.Run("nil client uses default", func(t *testing.T) {
		js := NewJumpService(nil, "")
		if js.client == nil {
			t.Error("expected non-nil client")
		}
	})

	t.Run("custom client preserved", func(t *testing.T) {
		client := &http.Client{Timeout: 5 * time.Second}
		js := NewJumpService(client, "")
		if js.client != client {
			t.Error("expected custom client to be preserved")
		}
	})
}

// TestJumpServiceCache tests the TTL-based address cache.
// Why: Caching prevents redundant network queries and improves performance.
func TestJumpServiceCache(t *testing.T) {
	t.Run("cache hit", func(t *testing.T) {
		js := NewJumpService(nil, "")
		// Manually insert a cache entry
		js.putCache("forum.i2p", "resolved-destination")

		dest, ok := js.getCached("forum.i2p")
		if !ok {
			t.Error("expected cache hit")
		}
		if dest != "resolved-destination" {
			t.Errorf("expected 'resolved-destination', got %q", dest)
		}
	})

	t.Run("cache miss", func(t *testing.T) {
		js := NewJumpService(nil, "")
		_, ok := js.getCached("unknown.i2p")
		if ok {
			t.Error("expected cache miss for unknown entry")
		}
	})

	t.Run("cache expiry", func(t *testing.T) {
		js := NewJumpService(nil, "")
		js.SetCacheTTL(1 * time.Millisecond)
		js.putCache("expiring.i2p", "dest")

		// Wait for TTL to expire
		time.Sleep(5 * time.Millisecond)

		_, ok := js.getCached("expiring.i2p")
		if ok {
			t.Error("expected cache entry to be expired")
		}
	})

	t.Run("cache disabled with zero TTL", func(t *testing.T) {
		js := NewJumpService(nil, "")
		js.SetCacheTTL(0)
		js.putCache("nocache.i2p", "dest")

		if js.CacheSize() != 0 {
			t.Error("expected empty cache when TTL is zero")
		}
	})

	t.Run("cache disabled with negative TTL", func(t *testing.T) {
		js := NewJumpService(nil, "")
		js.SetCacheTTL(-1 * time.Second)
		js.putCache("nocache.i2p", "dest")

		if js.CacheSize() != 0 {
			t.Error("expected empty cache when TTL is negative")
		}
	})

	t.Run("clear cache", func(t *testing.T) {
		js := NewJumpService(nil, "")
		js.putCache("a.i2p", "dest-a")
		js.putCache("b.i2p", "dest-b")

		if js.CacheSize() != 2 {
			t.Errorf("expected 2 cache entries, got %d", js.CacheSize())
		}

		js.ClearCache()
		if js.CacheSize() != 0 {
			t.Error("expected empty cache after clear")
		}
	})

	t.Run("cache size", func(t *testing.T) {
		js := NewJumpService(nil, "")
		for i := range 10 {
			js.putCache(fmt.Sprintf("site%d.i2p", i), "dest")
		}
		if js.CacheSize() != 10 {
			t.Errorf("expected 10 entries, got %d", js.CacheSize())
		}
	})
}

// TestJumpServiceLookupWithRedirect tests jump service resolution via HTTP 302.
// Why: Many jump services respond with redirects to the resolved base32 address.
func TestJumpServiceLookupWithRedirect(t *testing.T) {
	// Create a mock jump service that returns a 302 redirect
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hostname := r.URL.Query().Get("q")
		if hostname == "forum.i2p" {
			http.Redirect(w, r, "http://abc123def456.b32.i2p/", http.StatusFound)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer mockServer.Close()

	js := NewJumpService(mockServer.Client(), mockServer.URL)

	t.Run("successful redirect lookup", func(t *testing.T) {
		dest, err := js.Lookup("forum.i2p")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dest != "abc123def456.b32.i2p" {
			t.Errorf("expected 'abc123def456.b32.i2p', got %q", dest)
		}
	})

	t.Run("unknown hostname", func(t *testing.T) {
		_, err := js.Lookup("unknown.i2p")
		if err == nil {
			t.Error("expected error for unknown hostname")
		}
	})

	t.Run("cached result reused", func(t *testing.T) {
		js.ClearCache()
		// First lookup populates cache
		_, err := js.Lookup("forum.i2p")
		if err != nil {
			t.Fatalf("first lookup failed: %v", err)
		}
		if js.CacheSize() != 1 {
			t.Error("expected 1 cache entry after lookup")
		}
		// Second lookup should use cache (no HTTP request needed)
		dest, err := js.Lookup("forum.i2p")
		if err != nil {
			t.Fatalf("cached lookup failed: %v", err)
		}
		if dest != "abc123def456.b32.i2p" {
			t.Errorf("expected cached result, got %q", dest)
		}
	})
}

// TestJumpServiceLookupWithBody tests jump service that returns destination in body.
// Why: Some jump services respond with the destination as plain text or key=value.
func TestJumpServiceLookupWithBody(t *testing.T) {
	// Simulate a base64 I2P destination (516+ chars ending in AAAA)
	fakeDest := strings.Repeat("A", 512) + "AAAA"

	t.Run("key=value format", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hostname := r.URL.Query().Get("q")
			fmt.Fprintf(w, "%s=%s", hostname, fakeDest)
		}))
		defer mockServer.Close()

		js := NewJumpService(mockServer.Client(), mockServer.URL)
		dest, err := js.Lookup("forum.i2p")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dest != fakeDest {
			t.Errorf("expected destination, got %q", dest[:50]+"...")
		}
	})

	t.Run("plain base64 format", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, fakeDest)
		}))
		defer mockServer.Close()

		js := NewJumpService(mockServer.Client(), mockServer.URL)
		dest, err := js.Lookup("forum.i2p")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dest != fakeDest {
			t.Errorf("expected destination, got %q", dest[:50]+"...")
		}
	})

	t.Run("empty body returns error", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "")
		}))
		defer mockServer.Close()

		js := NewJumpService(mockServer.Client(), mockServer.URL)
		_, err := js.Lookup("forum.i2p")
		if err == nil {
			t.Error("expected error for empty body")
		}
	})

	t.Run("html body with destination", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "<html><body>\n%s\n</body></html>", fakeDest)
		}))
		defer mockServer.Close()

		js := NewJumpService(mockServer.Client(), mockServer.URL)
		dest, err := js.Lookup("forum.i2p")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dest != fakeDest {
			t.Errorf("expected destination extracted from HTML")
		}
	})
}

// TestJumpServiceLookupErrors tests error handling in the jump service client.
// Why: Network failures and invalid responses must be handled gracefully.
func TestJumpServiceLookupErrors(t *testing.T) {
	t.Run("server error", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}))
		defer mockServer.Close()

		js := NewJumpService(mockServer.Client(), mockServer.URL)
		_, err := js.Lookup("forum.i2p")
		if err == nil {
			t.Error("expected error for server error response")
		}
		if !strings.Contains(err.Error(), "500") {
			t.Errorf("error should mention status code: %v", err)
		}
	})

	t.Run("redirect without location header", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusFound) // 302 without Location
		}))
		defer mockServer.Close()

		js := NewJumpService(mockServer.Client(), mockServer.URL)
		_, err := js.Lookup("forum.i2p")
		if err == nil {
			t.Error("expected error for redirect without Location")
		}
	})

	t.Run("connection refused", func(t *testing.T) {
		// Use a URL that definitely won't connect
		js := NewJumpService(&http.Client{Timeout: 100 * time.Millisecond}, "http://127.0.0.1:1")
		_, err := js.Lookup("forum.i2p")
		if err == nil {
			t.Error("expected error for connection failure")
		}
	})
}

// TestParseDestinationFromBody tests the response body parser.
// Why: Jump services return destinations in various formats.
func TestParseDestinationFromBody(t *testing.T) {
	fakeDest := strings.Repeat("A", 512) + "AAAA"

	tests := []struct {
		name     string
		body     string
		hostname string
		want     string
	}{
		{
			name:     "key=value format",
			body:     "forum.i2p=" + fakeDest,
			hostname: "forum.i2p",
			want:     fakeDest,
		},
		{
			name:     "plain base64",
			body:     fakeDest,
			hostname: "forum.i2p",
			want:     fakeDest,
		},
		{
			name:     "with whitespace",
			body:     "  " + fakeDest + "  \n",
			hostname: "forum.i2p",
			want:     fakeDest,
		},
		{
			name:     "multi-line with destination",
			body:     "some header\n" + fakeDest + "\nsome footer",
			hostname: "forum.i2p",
			want:     fakeDest,
		},
		{
			name:     "empty body",
			body:     "",
			hostname: "forum.i2p",
			want:     "",
		},
		{
			name:     "no destination",
			body:     "error: hostname not found",
			hostname: "forum.i2p",
			want:     "",
		},
		{
			name:     "short base64 not a destination",
			body:     "shortAAAA",
			hostname: "forum.i2p",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDestinationFromBody(tt.body, tt.hostname)
			if got != tt.want {
				if len(got) > 50 {
					got = got[:50] + "..."
				}
				if len(tt.want) > 50 {
					t.Errorf("expected dest (truncated), got %q", got)
				} else {
					t.Errorf("expected %q, got %q", tt.want, got)
				}
			}
		})
	}
}

// TestIsI2PDestination tests I2P base64 destination validation.
// Why: Prevents treating random strings as valid destinations.
func TestIsI2PDestination(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"valid 516 char base64", strings.Repeat("A", 512) + "AAAA", true},
		{"valid with mixed base64 chars", strings.Repeat("abcABC012+/", 47) + strings.Repeat("B", 5) + "AAAA", true},
		{"too short", "shortAAAA", false},
		{"ends in AAAA but all A's", strings.Repeat("A", 516), true},
		{"empty", "", false},
		{"with spaces", strings.Repeat("A", 512) + " AAAA", false},
		{"with invalid chars", strings.Repeat("A", 510) + "!!AAAA", false},
		{"no AAAA suffix", strings.Repeat("B", 516), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isI2PDestination(tt.s)
			if got != tt.want {
				t.Errorf("isI2PDestination(%q...) = %v, want %v", truncate(tt.s, 30), got, tt.want)
			}
		})
	}
}

// TestExtractHostFromURL tests URL host extraction.
// Why: Jump service redirects include full URLs; we need just the host.
func TestExtractHostFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"http://abc.b32.i2p/", "abc.b32.i2p"},
		{"http://abc.b32.i2p:80/path", "abc.b32.i2p"},
		{"https://abc.b32.i2p/path?q=1", "abc.b32.i2p"},
		{"abc.b32.i2p/path", "abc.b32.i2p"},
		{"abc.b32.i2p", "abc.b32.i2p"},
		{"http://example.i2p", "example.i2p"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := extractHostFromURL(tt.url)
			if got != tt.want {
				t.Errorf("extractHostFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// TestStripPort tests port stripping from host strings.
// Why: Hostnames arrive with or without ports; normalization is needed for cache keys.
func TestStripPort(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{"forum.i2p:80", "forum.i2p"},
		{"forum.i2p", "forum.i2p"},
		{"127.0.0.1:8080", "127.0.0.1"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			got := stripPort(tt.host)
			if got != tt.want {
				t.Errorf("stripPort(%q) = %q, want %q", tt.host, got, tt.want)
			}
		})
	}
}

// TestResolveJump tests the resolveJump integration method on HTTPClient.
// Why: This is the actual code path used in DialContext — must handle all cases.
func TestResolveJump(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hostname := r.URL.Query().Get("q")
		if hostname == "forum.i2p" {
			http.Redirect(w, r, "http://resolved.b32.i2p/", http.StatusFound)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer mockServer.Close()

	t.Run("resolves human-readable hostname", func(t *testing.T) {
		h := &HTTPClient{
			Jump: NewJumpService(mockServer.Client(), mockServer.URL),
		}
		result := h.resolveJump("forum.i2p:80")
		if result != "resolved.b32.i2p:80" {
			t.Errorf("expected 'resolved.b32.i2p:80', got %q", result)
		}
	})

	t.Run("passes through base32 address", func(t *testing.T) {
		h := &HTTPClient{
			Jump: NewJumpService(mockServer.Client(), mockServer.URL),
		}
		result := h.resolveJump("abc.b32.i2p:80")
		if result != "abc.b32.i2p:80" {
			t.Errorf("expected passthrough, got %q", result)
		}
	})

	t.Run("passes through clearnet address", func(t *testing.T) {
		h := &HTTPClient{
			Jump: NewJumpService(mockServer.Client(), mockServer.URL),
		}
		result := h.resolveJump("example.com:443")
		if result != "example.com:443" {
			t.Errorf("expected passthrough, got %q", result)
		}
	})

	t.Run("nil jump service passes through", func(t *testing.T) {
		h := &HTTPClient{Jump: nil}
		result := h.resolveJump("forum.i2p:80")
		if result != "forum.i2p:80" {
			t.Errorf("expected passthrough with nil jump, got %q", result)
		}
	})

	t.Run("failed lookup returns original", func(t *testing.T) {
		h := &HTTPClient{
			Jump: NewJumpService(mockServer.Client(), mockServer.URL),
		}
		result := h.resolveJump("unknown.i2p:80")
		// Should return original on failure (graceful degradation)
		if result != "unknown.i2p:80" {
			t.Errorf("expected original on failed lookup, got %q", result)
		}
	})

	t.Run("address without port", func(t *testing.T) {
		h := &HTTPClient{
			Jump: NewJumpService(mockServer.Client(), mockServer.URL),
		}
		result := h.resolveJump("forum.i2p")
		if result != "resolved.b32.i2p" {
			t.Errorf("expected 'resolved.b32.i2p', got %q", result)
		}
	})
}

// TestJumpServiceOptionsIntegration tests that jump service config flows
// through Options() and SetOptions() properly.
// Why: Users need to configure jump service URL via the standard options API.
func TestJumpServiceOptionsIntegration(t *testing.T) {
	config := newTestConfig("js-opts")
	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	t.Run("default jump service URL in options", func(t *testing.T) {
		opts := client.Options()
		if opts["jumpservice"] != DefaultJumpServiceURL {
			t.Errorf("expected default URL in options, got %q", opts["jumpservice"])
		}
	})

	t.Run("set custom jump service URL", func(t *testing.T) {
		err := client.SetOptions(map[string]string{
			"jumpservice": "http://custom.i2p/jump",
		})
		if err != nil {
			t.Fatalf("SetOptions failed: %v", err)
		}
		opts := client.Options()
		if opts["jumpservice"] != "http://custom.i2p/jump" {
			t.Errorf("expected custom URL, got %q", opts["jumpservice"])
		}
	})

	t.Run("disable jump service with empty string", func(t *testing.T) {
		err := client.SetOptions(map[string]string{
			"jumpservice": "",
		})
		if err != nil {
			t.Fatalf("SetOptions failed: %v", err)
		}
		if client.Jump != nil {
			t.Error("expected jump service to be disabled")
		}
		opts := client.Options()
		if _, ok := opts["jumpservice"]; ok {
			t.Error("options should not include jumpservice when disabled")
		}
	})

	t.Run("re-enable jump service", func(t *testing.T) {
		err := client.SetOptions(map[string]string{
			"jumpservice": DefaultJumpServiceURL,
		})
		if err != nil {
			t.Fatalf("SetOptions failed: %v", err)
		}
		if client.Jump == nil {
			t.Error("expected jump service to be re-enabled")
		}
	})
}

// newTestConfig creates a minimal TunnelConfig for testing.
func newTestConfig(name string) i2pconv.TunnelConfig {
	return i2pconv.TunnelConfig{
		Name:      name,
		Type:      "httpclient",
		Port:      8118,
		Interface: "127.0.0.1",
	}
}

// truncate shortens a string for display in error messages.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
