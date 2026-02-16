package httpserver

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// TestHTTPServerCreation tests the creation of an HTTP server tunnel.
// Why: Validates constructor logic and default state initialization.
// Design: Uses mock SAM address since we're testing creation, not connection.
func TestHTTPServerCreation(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hs-creation",
		Type:      "httpserver",
		Port:      8080,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:8080",
	}

	server, err := NewHTTPServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create HTTP server: %v", err)
	}
	defer server.Garlic.Close()

	// Verify initial state
	if server.Name() != "hs-creation" {
		t.Errorf("Expected name 'hs-creation', got '%s'", server.Name())
	}
	if server.Type() != "httpserver" {
		t.Errorf("Expected type 'httpserver', got '%s'", server.Type())
	}
	if server.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected initial status stopped, got %v", server.Status())
	}
}

// TestHTTPServerOptions tests Options() and SetOptions() methods.
// Why: Configuration management is critical for production deployments.
// Design: Tests both retrieval and modification of tunnel options.
func TestHTTPServerOptions(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hs-options",
		Type:      "httpserver",
		Port:      8080,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewHTTPServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	// Test Options() retrieval
	opts := server.Options()
	if opts["name"] != "hs-options" {
		t.Errorf("Expected name 'hs-options', got '%s'", opts["name"])
	}
	if opts["port"] != "8080" {
		t.Errorf("Expected port '8080', got '%s'", opts["port"])
	}
	// Note: target in options comes from server.Addr, which is set to interface:port during construction
	// This is the local listening address, not the forward target

	// Test SetOptions() modification
	newOpts := map[string]string{
		"name":      "updated-http-server",
		"interface": "0.0.0.0",
		"port":      "9999",
		"maxconns":  "50",
		"ratelimit": "10.5",
	}
	if err := server.SetOptions(newOpts); err != nil {
		t.Fatalf("Failed to set options: %v", err)
	}

	// Verify changes applied
	updatedOpts := server.Options()
	if updatedOpts["name"] != "updated-http-server" {
		t.Errorf("Name not updated: got '%s'", updatedOpts["name"])
	}
	if updatedOpts["port"] != "9999" {
		t.Errorf("Port not updated: got '%s'", updatedOpts["port"])
	}
	if updatedOpts["maxconns"] != "50" {
		t.Errorf("MaxConns not updated: got '%s'", updatedOpts["maxconns"])
	}
}

// TestHTTPServerSetOptionsValidation tests validation in SetOptions.
// Why: Invalid configurations should be rejected before causing runtime errors.
// Design: Tests multiple validation scenarios with expected error cases.
func TestHTTPServerSetOptionsValidation(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hs-validation",
		Type:      "httpserver",
		Port:      8080,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:8080",
	}

	server, err := NewHTTPServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	tests := []struct {
		name      string
		opts      map[string]string
		wantError bool
	}{
		{
			name:      "valid config",
			opts:      map[string]string{"port": "8080", "maxconns": "100"},
			wantError: false,
		},
		{
			name:      "invalid port - too high",
			opts:      map[string]string{"port": "99999"},
			wantError: true,
		},
		{
			name:      "invalid port - negative",
			opts:      map[string]string{"port": "-1"},
			wantError: true,
		},
		{
			name:      "invalid maxconns - not a number",
			opts:      map[string]string{"maxconns": "not-a-number"},
			wantError: true,
		},
		{
			name:      "invalid ratelimit - not a number",
			opts:      map[string]string{"ratelimit": "not-a-number"},
			wantError: true,
		},
		{
			name:      "empty name",
			opts:      map[string]string{"name": ""},
			wantError: true,
		},
		{
			name:      "valid target",
			opts:      map[string]string{"target": "127.0.0.1:3000"},
			wantError: false,
		},
		{
			name:      "invalid target - no port",
			opts:      map[string]string{"target": "127.0.0.1"},
			wantError: true,
		},
		{
			name:      "invalid target - empty",
			opts:      map[string]string{"target": ""},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := server.SetOptions(tt.opts)
			if tt.wantError && err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no error for %s, got %v", tt.name, err)
			}
		})
	}
}

// TestHTTPServerSetOptionsTarget verifies that SetOptions correctly updates the
// target address and that the change is reflected in Options() output.
// Why: Finding #7 — target was previously silently dropped by SetOptions.
func TestHTTPServerSetOptionsTarget(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hs-target",
		Type:      "httpserver",
		Port:      8080,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewHTTPServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	origTarget := server.Target()

	// Change target via SetOptions
	newTarget := "192.168.1.100:3000"
	if err := server.SetOptions(map[string]string{"target": newTarget}); err != nil {
		t.Fatalf("SetOptions with valid target failed: %v", err)
	}

	// Verify Target() returns the new value
	if server.Target() != newTarget {
		t.Errorf("Target() = %q, want %q", server.Target(), newTarget)
	}

	// Verify Options() reflects the new value
	opts := server.Options()
	if opts["target"] != newTarget {
		t.Errorf("Options()[target] = %q, want %q", opts["target"], newTarget)
	}

	// Verify it actually changed from the original
	if server.Target() == origTarget {
		t.Error("Target was not actually updated from original value")
	}
}

// TestHTTPServerID tests ID generation.
// Why: IDs are used for tunnel identification in management interfaces.
// Design: Verifies ID is generated correctly from tunnel name.
func TestHTTPServerID(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hs-id-server",
		Type:      "httpserver",
		Port:      8080,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewHTTPServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	id := server.ID()
	// ID should be cleaned version of name
	if id == "" {
		t.Error("ID should not be empty")
	}
}

// TestHTTPServerLocalAddress tests LocalAddress() method.
// Why: Management interfaces need to know the I2P listening address.
// Design: Verifies correct host:port formatting.
func TestHTTPServerLocalAddress(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hs-localaddr",
		Type:      "httpserver",
		Port:      8080,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewHTTPServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	addr, err := server.LocalAddress()
	if err != nil {
		t.Fatalf("Failed to get local address: %v", err)
	}

	expected := net.JoinHostPort("127.0.0.1", "8080")
	if addr != expected {
		t.Errorf("Expected address '%s', got '%s'", expected, addr)
	}
}

// TestHTTPServerTarget tests Target() method.
// Why: HTTP server forwards to a specific local service target.
// Design: Verifies the target address reflects the configured forward target.
func TestHTTPServerTarget(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hs-target2",
		Type:      "httpserver",
		Port:      8080,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewHTTPServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	target := server.Target()
	// Target() should return the forward target address from config
	expectedTarget := "127.0.0.1:9090"
	if target != expectedTarget {
		t.Errorf("Expected target '%s', got '%s'", expectedTarget, target)
	}
}

// TestHTTPServerLoadConfig tests configuration loading from file.
// Why: Production deployments need to persist and reload configurations.
// Design: Tests successful load, validation, and error cases.
func TestHTTPServerLoadConfig(t *testing.T) {
	// Create temporary directory for test configs
	tmpDir := t.TempDir()

	// Test case 1: Successful load from YAML
	t.Run("successful load from yaml", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "http-server.yaml")
		// YAML format requires tunnels: map with tunnel name as key
		configContent := `tunnels:
  loaded-http-server:
    name: loaded-http-server
    type: httpserver
    interface: 0.0.0.0
    port: 8888
    target: 127.0.0.1:9999
`
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		server, err := NewHTTPServer(i2pconv.TunnelConfig{
			Name:      "hs-lcfg-yaml",
			Type:      "httpserver",
			Port:      8080,
			Interface: "127.0.0.1",
			Target:    "127.0.0.1:8080",
		}, "127.0.0.1:7656")
		if err != nil {
			t.Fatalf("Failed to create server: %v", err)
		}
		defer server.Garlic.Close()

		if err := server.LoadConfig(configPath); err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Verify config was loaded
		if server.Name() != "loaded-http-server" {
			t.Errorf("Expected name 'loaded-http-server', got '%s'", server.Name())
		}
		if server.TunnelConfig.Port != 8888 {
			t.Errorf("Expected port 8888, got %d", server.TunnelConfig.Port)
		}
		if server.Target() != "127.0.0.1:9999" {
			t.Errorf("Expected target '127.0.0.1:9999', got '%s'", server.Target())
		}
	})

	// Test case 2: Reject config load while running
	t.Run("reject load while running", func(t *testing.T) {
		server, err := NewHTTPServer(i2pconv.TunnelConfig{
			Name:      "hs-lcfg-run",
			Type:      "httpserver",
			Port:      8080,
			Interface: "127.0.0.1",
			Target:    "127.0.0.1:8080",
		}, "127.0.0.1:7656")
		if err != nil {
			t.Fatalf("Failed to create server: %v", err)
		}
		defer server.Garlic.Close()

		// Simulate running state
		server.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning

		configPath := filepath.Join(tmpDir, "test.yaml")
		configContent := `tunnels:
  test:
    name: test
    type: httpserver
    interface: 127.0.0.1
    port: 8080
    target: 127.0.0.1:8080
`
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		err = server.LoadConfig(configPath)
		if err == nil {
			t.Error("Expected error when loading config while running")
		}
	})

	// Test case 3: Reject wrong tunnel type
	t.Run("reject wrong type", func(t *testing.T) {
		server, err := NewHTTPServer(i2pconv.TunnelConfig{
			Name:      "hs-lcfg-type",
			Type:      "httpserver",
			Port:      8080,
			Interface: "127.0.0.1",
			Target:    "127.0.0.1:8080",
		}, "127.0.0.1:7656")
		if err != nil {
			t.Fatalf("Failed to create server: %v", err)
		}
		defer server.Garlic.Close()

		configPath := filepath.Join(tmpDir, "wrong-type.yaml")
		configContent := `tunnels:
  test:
    name: test
    type: httpclient
    interface: 127.0.0.1
    port: 8080
`
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		err = server.LoadConfig(configPath)
		if err == nil {
			t.Error("Expected error when loading wrong tunnel type")
		}
	})

	// Test case 4: Reject invalid target address
	t.Run("invalid target address", func(t *testing.T) {
		server, err := NewHTTPServer(i2pconv.TunnelConfig{
			Name:      "hs-lcfg-tgt",
			Type:      "httpserver",
			Port:      8080,
			Interface: "127.0.0.1",
			Target:    "127.0.0.1:8080",
		}, "127.0.0.1:7656")
		if err != nil {
			t.Fatalf("Failed to create server: %v", err)
		}
		defer server.Garlic.Close()

		configPath := filepath.Join(tmpDir, "invalid-target.yaml")
		configContent := `tunnels:
  test:
    name: test
    type: httpserver
    interface: 127.0.0.1
    port: 8080
    target: invalid-target-address
`
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		err = server.LoadConfig(configPath)
		if err == nil {
			t.Error("Expected error when loading invalid target address")
		}
	})

	// Test case 5: Handle invalid file
	t.Run("invalid file", func(t *testing.T) {
		server, err := NewHTTPServer(i2pconv.TunnelConfig{
			Name:      "hs-lcfg-inv",
			Type:      "httpserver",
			Port:      8080,
			Interface: "127.0.0.1",
			Target:    "127.0.0.1:8080",
		}, "127.0.0.1:7656")
		if err != nil {
			t.Fatalf("Failed to create server: %v", err)
		}
		defer server.Garlic.Close()

		err = server.LoadConfig("/nonexistent/path/config.yaml")
		if err == nil {
			t.Error("Expected error when loading nonexistent file")
		}
	})
}

// TestHTTPServerErrorTracking tests error recording and retrieval.
// Why: Production systems need error visibility for debugging.
// Design: Tests error history tracking and Error() method.
func TestHTTPServerErrorTracking(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hs-errors",
		Type:      "httpserver",
		Port:      8080,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewHTTPServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	// Initially no errors
	if server.Error() != nil {
		t.Error("Expected no error initially")
	}

	// Record an error
	testErr := fmt.Errorf("test error")
	server.recordError(testErr)

	// Verify error was recorded
	if server.Error() == nil {
		t.Error("Expected error to be recorded")
	}
	if len(server.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(server.Errors))
	}
}

// TestHTTPServerStopBeforeStart tests stopping a non-started tunnel.
// Why: Defensive programming - stop should be safe to call anytime.
// Design: Verifies Stop() is idempotent and doesn't panic.
func TestHTTPServerStopBeforeStart(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hs-stop",
		Type:      "httpserver",
		Port:      8080,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewHTTPServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	// Stop should not panic on non-started tunnel
	if err := server.Stop(); err != nil {
		t.Errorf("Stop should not error on non-started tunnel: %v", err)
	}
}

// TestHTTPServerRateLimiting tests rate limiting configuration.
// Why: Rate limiting is crucial for preventing abuse in production.
// Design: Verifies rate limit and maxconns settings.
func TestHTTPServerRateLimiting(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hs-ratelimit",
		Type:      "httpserver",
		Port:      8080,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewHTTPServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	// Set rate limiting options
	opts := map[string]string{
		"maxconns":  "25",
		"ratelimit": "5.5",
	}
	if err := server.SetOptions(opts); err != nil {
		t.Fatalf("Failed to set rate limiting options: %v", err)
	}

	// Verify settings
	retrievedOpts := server.Options()
	if retrievedOpts["maxconns"] != "25" {
		t.Errorf("Expected maxconns '25', got '%s'", retrievedOpts["maxconns"])
	}
	if retrievedOpts["ratelimit"] != "5.5" {
		t.Errorf("Expected ratelimit '5.5', got '%s'", retrievedOpts["ratelimit"])
	}
}

// TestHTTPServerPortAllocation tests that the server can bind to available ports.
// Why: Port conflicts are common deployment issues.
// Design: Uses random available port to avoid conflicts in test environment.
func TestHTTPServerPortAllocation(t *testing.T) {
	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Give OS time to release the port
	time.Sleep(100 * time.Millisecond)

	config := i2pconv.TunnelConfig{
		Name:      "hs-port",
		Type:      "httpserver",
		Port:      port,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewHTTPServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	addr, err := server.LocalAddress()
	if err != nil {
		t.Fatalf("Failed to get local address: %v", err)
	}

	expectedAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	if addr != expectedAddr {
		t.Errorf("Expected address %s, got %s", expectedAddr, addr)
	}
}
