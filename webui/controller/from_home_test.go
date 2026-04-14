package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandlerFunction tests the handler() URL routing function.
func TestHandlerFunction(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"home page", "/home", "group"},
		{"new tunnel page", "/new", "new"},
		{"root", "/", "group"},
		{"config page", "/my-tunnel/config", "config"},
		{"control page", "/my-tunnel/control", "control"},
		{"unknown path", "/unknown", "group"},
		{"nested config", "/group/my-tunnel/config", "config"},
		{"nested control", "/group/my-tunnel/control", "control"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			result := handler(req)
			if result != tt.expected {
				t.Errorf("handler(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}

	t.Run("nil request", func(t *testing.T) {
		result := handler(nil)
		if result != "group" {
			t.Errorf("handler(nil) = %q, want %q", result, "group")
		}
	})
}

// TestTunnelFunction tests the tunnel() URL routing function.
// Why: Ensures tunnel names are correctly extracted from URL paths,
// including nested paths that previously caused incorrect tunnel IDs.
func TestTunnelFunction(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"simple config", "/my-tunnel/config", "my-tunnel"},
		{"simple control", "/my-tunnel/control", "my-tunnel"},
		{"nested path config", "/group/my-tunnel/config", "my-tunnel"},
		{"nested path control", "/group/my-tunnel/control", "my-tunnel"},
		{"deeply nested", "/a/b/c/my-tunnel/control", "my-tunnel"},
		{"tunnel with hyphens", "/my-cool-tunnel/config", "my-cool-tunnel"},
		{"no action suffix", "/my-tunnel", ""},
		{"root path", "/", ""},
		{"home page", "/home", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			result := tunnel(req)
			if result != tt.expected {
				t.Errorf("tunnel(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}

	t.Run("nil request", func(t *testing.T) {
		result := tunnel(nil)
		if result != "" {
			t.Errorf("tunnel(nil) = %q, want empty string", result)
		}
	})
}
