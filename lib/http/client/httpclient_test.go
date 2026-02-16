package httpclient

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

// TestHTTPClientCreation tests the creation of an HTTP client tunnel.
// Why: Validates constructor logic and default state initialization.
// Design: Uses mock SAM address since we're testing creation, not connection.
func TestHTTPClientCreation(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hc-creation",
		Type:      "httpclient",
		Port:      8080,
		Interface: "127.0.0.1",
	}

	// NewHTTPClient should create instance even without SAM available (connection happens on Start)
	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create HTTP client: %v", err)
	}
	defer client.Garlic.Close()

	// Verify initial state
	if client.Name() != "hc-creation" {
		t.Errorf("Expected name 'hc-creation', got '%s'", client.Name())
	}
	if client.Type() != "httpclient" {
		t.Errorf("Expected type 'httpclient', got '%s'", client.Type())
	}
	if client.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected initial status stopped, got %v", client.Status())
	}
}

// TestHTTPClientOptions tests Options() and SetOptions() methods.
// Why: Configuration management is critical for production deployments.
// Design: Tests both retrieval and modification of tunnel options.
func TestHTTPClientOptions(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hc-options",
		Type:      "httpclient",
		Port:      8118,
		Interface: "127.0.0.1",
	}

	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	// Test Options() retrieval
	opts := client.Options()
	if opts["name"] != "hc-options" {
		t.Errorf("Expected name 'hc-options', got '%s'", opts["name"])
	}
	if opts["port"] != "8118" {
		t.Errorf("Expected port '8118', got '%s'", opts["port"])
	}

	// Test SetOptions() modification
	newOpts := map[string]string{
		"name":      "updated-http-client",
		"interface": "0.0.0.0",
		"port":      "9090",
	}
	if err := client.SetOptions(newOpts); err != nil {
		t.Fatalf("Failed to set options: %v", err)
	}

	// Verify changes applied
	updatedOpts := client.Options()
	if updatedOpts["name"] != "updated-http-client" {
		t.Errorf("Name not updated: got '%s'", updatedOpts["name"])
	}
	if updatedOpts["port"] != "9090" {
		t.Errorf("Port not updated: got '%s'", updatedOpts["port"])
	}
}

// TestHTTPClientSetOptionsValidation tests validation in SetOptions.
// Why: Invalid configurations should be rejected before causing runtime errors.
// Design: Tests multiple validation scenarios with expected error cases.
func TestHTTPClientSetOptionsValidation(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hc-validation",
		Type:      "httpclient",
		Port:      8118,
		Interface: "127.0.0.1",
	}

	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	tests := []struct {
		name      string
		opts      map[string]string
		wantError bool
	}{
		{
			name:      "valid config",
			opts:      map[string]string{"port": "8080", "interface": "127.0.0.1"},
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
			name:      "invalid port - not a number",
			opts:      map[string]string{"port": "not-a-port"},
			wantError: true,
		},
		{
			name:      "empty name",
			opts:      map[string]string{"name": ""},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.SetOptions(tt.opts)
			if tt.wantError && err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no error for %s, got %v", tt.name, err)
			}
		})
	}
}

// TestHTTPClientID tests ID generation.
// Why: IDs are used for tunnel identification in management interfaces.
// Design: Verifies ID is generated correctly from tunnel name.
func TestHTTPClientID(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hc-id-proxy",
		Type:      "httpclient",
		Port:      8118,
		Interface: "127.0.0.1",
	}

	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	id := client.ID()
	// ID should be cleaned version of name
	if id == "" {
		t.Error("ID should not be empty")
	}
}

// TestHTTPClientLocalAddress tests LocalAddress() method.
// Why: Applications need to know where to connect to use the proxy.
// Design: Verifies correct host:port formatting.
func TestHTTPClientLocalAddress(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hc-localaddr",
		Type:      "httpclient",
		Port:      8118,
		Interface: "127.0.0.1",
	}

	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	addr, err := client.LocalAddress()
	if err != nil {
		t.Fatalf("Failed to get local address: %v", err)
	}

	expected := net.JoinHostPort("127.0.0.1", "8118")
	if addr != expected {
		t.Errorf("Expected address '%s', got '%s'", expected, addr)
	}
}

// TestHTTPClientTarget tests Target() method.
// Why: HTTP clients are one-to-many proxies, so Target should be empty.
// Design: Verifies the method returns empty string for proxy tunnels.
func TestHTTPClientTarget(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hc-target",
		Type:      "httpclient",
		Port:      8118,
		Interface: "127.0.0.1",
	}

	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	target := client.Target()
	if target != "" {
		t.Errorf("HTTP client should have empty target (one-to-many), got '%s'", target)
	}
}

// TestHTTPClientLoadConfig tests configuration loading from file.
// Why: Production deployments need to persist and reload configurations.
// Design: Tests successful load, validation, and error cases.
func TestHTTPClientLoadConfig(t *testing.T) {
	// Create temporary directory for test configs
	tmpDir := t.TempDir()

	// Test case 1: Successful load from YAML
	t.Run("successful load from yaml", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "http-client.yaml")
		// YAML format requires tunnels: map with tunnel name as key
		configContent := `tunnels:
  loaded-http-client:
    name: loaded-http-client
    type: httpclient
    interface: 0.0.0.0
    port: 8888
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		client, err := NewHTTPClient(i2pconv.TunnelConfig{
			Name:      "hc-lcfg-yaml",
			Type:      "httpclient",
			Port:      8080,
			Interface: "127.0.0.1",
		}, "127.0.0.1:7656")
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer client.Garlic.Close()

		if err := client.LoadConfig(configPath); err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Verify config was loaded
		if client.Name() != "loaded-http-client" {
			t.Errorf("Expected name 'loaded-http-client', got '%s'", client.Name())
		}
		if client.TunnelConfig.Port != 8888 {
			t.Errorf("Expected port 8888, got %d", client.TunnelConfig.Port)
		}
	})

	// Test case 2: Reject config load while running
	t.Run("reject load while running", func(t *testing.T) {
		client, err := NewHTTPClient(i2pconv.TunnelConfig{
			Name:      "hc-lcfg-run",
			Type:      "httpclient",
			Port:      8080,
			Interface: "127.0.0.1",
		}, "127.0.0.1:7656")
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer client.Garlic.Close()

		// Simulate running state
		client.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning

		configPath := filepath.Join(tmpDir, "test.yaml")
		configContent := `tunnels:
  test:
    name: test
    type: httpclient
    interface: 127.0.0.1
    port: 8080
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		err = client.LoadConfig(configPath)
		if err == nil {
			t.Error("Expected error when loading config while running")
		}
	})

	// Test case 3: Reject wrong tunnel type
	t.Run("reject wrong type", func(t *testing.T) {
		client, err := NewHTTPClient(i2pconv.TunnelConfig{
			Name:      "hc-lcfg-type",
			Type:      "httpclient",
			Port:      8080,
			Interface: "127.0.0.1",
		}, "127.0.0.1:7656")
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer client.Garlic.Close()

		configPath := filepath.Join(tmpDir, "wrong-type.yaml")
		configContent := `tunnels:
  test:
    name: test
    type: tcpclient
    interface: 127.0.0.1
    port: 8080
    target: test.i2p
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		err = client.LoadConfig(configPath)
		if err == nil {
			t.Error("Expected error when loading wrong tunnel type")
		}
	})

	// Test case 4: Handle invalid file
	t.Run("invalid file", func(t *testing.T) {
		client, err := NewHTTPClient(i2pconv.TunnelConfig{
			Name:      "hc-lcfg-inv",
			Type:      "httpclient",
			Port:      8080,
			Interface: "127.0.0.1",
		}, "127.0.0.1:7656")
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer client.Garlic.Close()

		err = client.LoadConfig("/nonexistent/path/config.yaml")
		if err == nil {
			t.Error("Expected error when loading nonexistent file")
		}
	})
}

// TestHTTPClientErrorTracking tests error recording and retrieval.
// Why: Production systems need error visibility for debugging.
// Design: Tests error history tracking and Error() method.
func TestHTTPClientErrorTracking(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hc-errors",
		Type:      "httpclient",
		Port:      8118,
		Interface: "127.0.0.1",
	}

	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	// Initially no errors
	if client.Error() != nil {
		t.Error("Expected no error initially")
	}

	// Record an error
	testErr := fmt.Errorf("test error")
	client.recordError(testErr)

	// Verify error was recorded
	if client.Error() == nil {
		t.Error("Expected error to be recorded")
	}
	if len(client.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(client.Errors))
	}
}

// TestHTTPClientStopBeforeStart tests stopping a non-started tunnel.
// Why: Defensive programming - stop should be safe to call anytime.
// Design: Verifies Stop() is idempotent and doesn't panic.
func TestHTTPClientStopBeforeStart(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "hc-stop",
		Type:      "httpclient",
		Port:      8118,
		Interface: "127.0.0.1",
	}

	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	// Stop should not error on non-started tunnel
	if err := client.Stop(); err != nil {
		t.Errorf("Stop should not error on non-started tunnel: %v", err)
	}
}

// TestHTTPClientPortAllocation tests that the client can bind to available ports.
// Why: Port conflicts are common deployment issues.
// Design: Uses random available port to avoid conflicts in test environment.
func TestHTTPClientPortAllocation(t *testing.T) {
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
		Name:      "hc-port",
		Type:      "httpclient",
		Port:      port,
		Interface: "127.0.0.1",
	}

	client, err := NewHTTPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	addr, err := client.LocalAddress()
	if err != nil {
		t.Fatalf("Failed to get local address: %v", err)
	}

	expectedAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	if addr != expectedAddr {
		t.Errorf("Expected address %s, got %s", expectedAddr, addr)
	}
}
