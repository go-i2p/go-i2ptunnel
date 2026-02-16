package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// createTestConfig creates a temporary config file with the given parameters
func createTestConfig(t *testing.T, name, tunnelType, target string, port int) string {
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, fmt.Sprintf("%s.yaml", name))

	// YAML format requires tunnels: map with tunnel name as key
	configContent := fmt.Sprintf(`tunnels:
  %s:
    name: %s
    type: %s
    interface: 127.0.0.1
    port: %d`, name, name, tunnelType, port)

	if target != "" {
		configContent += fmt.Sprintf(`
    target: %s`, target)
	}

	configContent += "\n"

	if err := os.WriteFile(configFile, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	return configFile
}

// TestConfigServeHTTPGet tests the GET request for configuration display
func TestConfigServeHTTPGet(t *testing.T) {
	configFile := createTestConfig(t, "wcc-get", "tcpclient", "example.i2p", 8080)

	cfg, err := NewConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/wcc-get/config", nil)
	w := httptest.NewRecorder()

	cfg.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "wcc-get") {
		t.Errorf("Response should contain tunnel name")
	}
}

// TestConfigServeHTTPPost tests saving configuration changes
func TestConfigServeHTTPPost(t *testing.T) {
	configFile := createTestConfig(t, "wcc-post", "tcpclient", "example.i2p", 8080)

	cfg, err := NewConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Test POST request with new config
	formData := url.Values{}
	formData.Set("host", "localhost")
	formData.Set("port", "9090")

	req := httptest.NewRequest(http.MethodPost, "/wcc-post/config", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	cfg.ServeHTTP(w, req)

	// Should redirect on success
	if w.Code != http.StatusSeeOther {
		t.Logf("Response body: %s", w.Body.String())
		t.Errorf("Expected status 303 (redirect), got %d", w.Code)
	}
}

// TestConfigServeHTTPPostInvalidPort tests validation of invalid port numbers
func TestConfigServeHTTPPostInvalidPort(t *testing.T) {
	configFile := createTestConfig(t, "wcc-invport", "tcpclient", "example.i2p", 8080)

	cfg, err := NewConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	testCases := []struct {
		name     string
		port     string
		wantCode int
	}{
		{"invalid port text", "abc", http.StatusBadRequest},
		{"port too low", "0", http.StatusBadRequest},
		{"port too high", "99999", http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			formData := url.Values{}
			formData.Set("port", tc.port)

			req := httptest.NewRequest(http.MethodPost, "/wcc-invport/config", strings.NewReader(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			cfg.ServeHTTP(w, req)

			if w.Code != tc.wantCode {
				t.Errorf("Expected status %d, got %d", tc.wantCode, w.Code)
			}
		})
	}
}

// TestConfigServeHTTPPostWhileRunning tests that config cannot be changed while tunnel is running
func TestConfigServeHTTPPostWhileRunning(t *testing.T) {
	configFile := createTestConfig(t, "wcc-running", "tcpclient", "example.i2p", 8080)

	cfg, err := NewConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Start the tunnel in a goroutine since Start() blocks in accept loop
	go cfg.Start()
	defer cfg.Stop()

	// Give the tunnel time to enter running state
	time.Sleep(200 * time.Millisecond)

	// Try to modify config while running
	formData := url.Values{}
	formData.Set("port", "9090")

	req := httptest.NewRequest(http.MethodPost, "/wcc-running/config", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	cfg.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "running") {
		t.Errorf("Error message should mention tunnel is running")
	}
} // TestNewConfig tests config creation from file
func TestNewConfig(t *testing.T) {
	configFile := createTestConfig(t, "wcc-newcfg", "tcpclient", "example.i2p", 8080)

	cfg, err := NewConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected non-nil config")
	}

	if cfg.configPath != configFile {
		t.Errorf("Expected configPath %s, got %s", configFile, cfg.configPath)
	}
}

// TestNewConfigInvalidFile tests handling of invalid config files
func TestNewConfigInvalidFile(t *testing.T) {
	cfg, err := NewConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if cfg != nil {
		t.Error("Expected nil config for non-existent file")
	}
}
