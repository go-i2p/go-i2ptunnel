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

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/webui/templates"
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

// TestConfigServeHTTPPostWhileRunning tests that config cannot be changed while tunnel is running.
// Uses a mock I2PTunnel whose Status() always returns running so the test does not
// require a live SAM connection.
func TestConfigServeHTTPPostWhileRunning(t *testing.T) {
	configFile := createTestConfig(t, "wcc-running", "tcpclient", "example.i2p", 8080)

	mock := &mockTunnel{
		name:    "wcc-running",
		id:      "wcc-running",
		kind:    "tcpclient",
		status:  i2ptunnel.I2PTunnelStatusRunning,
		options: map[string]string{"port": "8080"},
	}
	cfg := &Config{I2PTunnel: mock, configPath: configFile}

	// Try to modify config while tunnel reports running status
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

// TestConfigTemplateTargetDestinationVisibility verifies that the Target Destination
// field is rendered for all tunnel types that have a meaningful target, and hidden
// for proxy types (httpclient, socks) which are one-to-many.
//
// This tests the fix for AUDIT finding #4: the condition was previously
// `eq .Type "client"` which never matched any real tunnel type string.
func TestConfigTemplateTargetDestinationVisibility(t *testing.T) {
	typesWithTarget := []string{
		"tcpclient", "udpclient", "ircclient",
		"tcpserver", "httpserver", "ircserver", "udpserver",
		"tcpbidirectional", "httpbidirectional", "udpbidirectional",
	}
	typesWithoutTarget := []string{
		"httpclient", "socks",
	}

	for _, tunnelType := range typesWithTarget {
		t.Run(tunnelType+"_shows_target", func(t *testing.T) {
			data := ConfigData{
				Name:    "test-tunnel",
				ID:      "test-tunnel",
				Type:    tunnelType,
				Target:  "target.i2p",
				Options: map[string]string{"port": "8080"},
			}
			var buf strings.Builder
			if err := templates.I2PTunnelConfigTemplate.Execute(&buf, data); err != nil {
				t.Fatalf("Template execution failed: %v", err)
			}
			body := buf.String()
			if !strings.Contains(body, "Target Destination") {
				t.Errorf("type %q should show Target Destination field", tunnelType)
			}
			if !strings.Contains(body, "target.i2p") {
				t.Errorf("type %q should render the target value", tunnelType)
			}
		})
	}

	for _, tunnelType := range typesWithoutTarget {
		t.Run(tunnelType+"_hides_target", func(t *testing.T) {
			data := ConfigData{
				Name:    "test-proxy",
				ID:      "test-proxy",
				Type:    tunnelType,
				Target:  "",
				Options: map[string]string{"port": "4444"},
			}
			var buf strings.Builder
			if err := templates.I2PTunnelConfigTemplate.Execute(&buf, data); err != nil {
				t.Fatalf("Template execution failed: %v", err)
			}
			body := buf.String()
			if strings.Contains(body, "Target Destination") {
				t.Errorf("type %q should NOT show Target Destination field", tunnelType)
			}
		})
	}
}

// TestConfigTemplateBidirectionalTypesInDropdown verifies that bidirectional tunnel
// types appear in the type dropdown. This tests the fix for AUDIT finding #5:
// the dropdown previously omitted tcpbidirectional, httpbidirectional, udpbidirectional.
func TestConfigTemplateBidirectionalTypesInDropdown(t *testing.T) {
	bidirectionalTypes := []string{
		"tcpbidirectional",
		"httpbidirectional",
		"udpbidirectional",
	}

	// Render template with a bidirectional type
	data := ConfigData{
		Name:    "test-bidir",
		ID:      "test-bidir",
		Type:    "tcpbidirectional",
		Target:  "127.0.0.1:8080",
		Options: map[string]string{"port": "7070"},
	}
	var buf strings.Builder
	if err := templates.I2PTunnelConfigTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("Template execution failed: %v", err)
	}
	body := buf.String()

	for _, bt := range bidirectionalTypes {
		if !strings.Contains(body, fmt.Sprintf("value=\"%s\"", bt)) {
			t.Errorf("dropdown should contain bidirectional type %q", bt)
		}
	}

	// Verify the optgroup label exists
	if !strings.Contains(body, "Bidirectional Tunnels") {
		t.Error("dropdown should contain 'Bidirectional Tunnels' optgroup")
	}

	// Verify the selected bidirectional type has 'selected' attribute
	if !strings.Contains(body, "value=\"tcpbidirectional\" selected") {
		t.Error("tcpbidirectional should be selected")
	}
}

// TestConfigTemplateAllTypesInDropdown verifies that every valid tunnel type
// appears in the configuration form's type dropdown.
func TestConfigTemplateAllTypesInDropdown(t *testing.T) {
	allTypes := []string{
		"tcpserver", "httpserver", "ircserver", "udpserver",
		"tcpclient", "udpclient", "ircclient",
		"httpclient", "socks",
		"tcpbidirectional", "httpbidirectional", "udpbidirectional",
	}

	data := ConfigData{
		Name:    "dropdown-test",
		ID:      "dropdown-test",
		Type:    "tcpclient",
		Options: map[string]string{"port": "8080"},
	}
	var buf strings.Builder
	if err := templates.I2PTunnelConfigTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("Template execution failed: %v", err)
	}
	body := buf.String()

	for _, typ := range allTypes {
		if !strings.Contains(body, fmt.Sprintf("value=\"%s\"", typ)) {
			t.Errorf("dropdown should contain tunnel type %q", typ)
		}
	}
}

// TestConfigTemplateEncryptedLeaseSetFields verifies that the configuration form
// renders the Encrypted LeaseSet fieldset with all required fields:
// LeaseSet Type, Encryption Type, Authentication Type, and Private Key.
func TestConfigTemplateEncryptedLeaseSetFields(t *testing.T) {
	data := ConfigData{
		Name:   "test-els",
		ID:     "test-els",
		Type:   "tcpserver",
		Target: "127.0.0.1:8080",
		Options: map[string]string{
			"port":                  "8080",
			"i2cp.leaseSetType":     "3",
			"i2cp.leaseSetEncType":  "4,0",
			"i2cp.leaseSetAuthType": "2",
			"i2cp.leaseSetPrivKey":  "testkey123",
		},
	}
	var buf strings.Builder
	if err := templates.I2PTunnelConfigTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("Template execution failed: %v", err)
	}
	body := buf.String()

	// Verify fieldset exists
	if !strings.Contains(body, "Encrypted LeaseSet") {
		t.Error("template should contain 'Encrypted LeaseSet' fieldset")
	}

	// Verify LeaseSet Type dropdown renders with Encrypted selected
	if !strings.Contains(body, "name=\"i2cp.leaseSetType\"") {
		t.Error("template should contain leaseSetType field")
	}
	// "3" is Encrypted — should be selected
	if !strings.Contains(body, "value=\"3\" selected") {
		t.Error("Encrypted (3) should be selected when leaseSetType=3")
	}

	// Verify Encryption Type input renders with value
	if !strings.Contains(body, "name=\"i2cp.leaseSetEncType\"") {
		t.Error("template should contain leaseSetEncType field")
	}
	if !strings.Contains(body, "value=\"4,0\"") {
		t.Error("leaseSetEncType should show value 4,0")
	}

	// Verify Authentication Type dropdown renders with PSK selected
	if !strings.Contains(body, "name=\"i2cp.leaseSetAuthType\"") {
		t.Error("template should contain leaseSetAuthType field")
	}
	if !strings.Contains(body, "value=\"2\" selected") {
		t.Error("PSK (2) should be selected when leaseSetAuthType=2")
	}

	// Verify Private Key field renders with value
	if !strings.Contains(body, "name=\"i2cp.leaseSetPrivKey\"") {
		t.Error("template should contain leaseSetPrivKey field")
	}
	if !strings.Contains(body, "testkey123") {
		t.Error("leaseSetPrivKey should show the configured value")
	}
}

// TestConfigTemplateEncryptedLeaseSetDefaults verifies that the Encrypted LeaseSet
// fields show sensible defaults when no I2CP options are configured.
func TestConfigTemplateEncryptedLeaseSetDefaults(t *testing.T) {
	data := ConfigData{
		Name:    "test-defaults",
		ID:      "test-defaults",
		Type:    "tcpserver",
		Options: map[string]string{"port": "8080"},
	}
	var buf strings.Builder
	if err := templates.I2PTunnelConfigTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("Template execution failed: %v", err)
	}
	body := buf.String()

	// Standard (1) should be selected by default when leaseSetType is empty
	if !strings.Contains(body, "value=\"1\" selected") {
		t.Error("Standard (1) should be selected by default")
	}

	// None (0) should be selected by default for auth type
	if !strings.Contains(body, "value=\"0\" selected") {
		t.Error("None (0) auth type should be selected by default")
	}
}
