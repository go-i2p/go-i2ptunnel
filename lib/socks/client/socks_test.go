package socks

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

// TestSOCKSClientCreation tests the creation of a SOCKS client tunnel.
// Why: Validates constructor logic and default state initialization.
// Design: Uses real SAM address to create a Garlic session; tests struct fields.
func TestSOCKSClientCreation(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "sc-creation",
		Type:      "socksclient",
		Port:      1080,
		Interface: "127.0.0.1",
	}

	client, err := NewSocksClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create SOCKS client: %v", err)
	}
	defer client.Garlic.Close()

	// Verify initial state
	if client.Name() != "sc-creation" {
		t.Errorf("Expected name 'sc-creation', got '%s'", client.Name())
	}
	if client.Type() != "socksclient" {
		t.Errorf("Expected type 'socksclient', got '%s'", client.Type())
	}
	if client.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected initial status stopped, got %v", client.Status())
	}
}

// TestSOCKSClientOptions tests Options() and SetOptions() methods.
// Why: Configuration management is critical for production deployments.
// Design: Tests both retrieval and modification of tunnel options.
func TestSOCKSClientOptions(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "sc-options",
		Type:      "socksclient",
		Port:      1081,
		Interface: "127.0.0.1",
	}

	client, err := NewSocksClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	// Test Options() retrieval
	opts := client.Options()
	if opts["name"] != "sc-options" {
		t.Errorf("Expected name 'sc-options', got '%s'", opts["name"])
	}
	if opts["port"] != "1081" {
		t.Errorf("Expected port '1081', got '%s'", opts["port"])
	}

	// Test SetOptions() modification
	newOpts := map[string]string{
		"name":      "updated-socks-client",
		"interface": "0.0.0.0",
		"port":      "1082",
	}
	if err := client.SetOptions(newOpts); err != nil {
		t.Fatalf("Failed to set options: %v", err)
	}

	// Verify changes applied
	updatedOpts := client.Options()
	if updatedOpts["name"] != "updated-socks-client" {
		t.Errorf("Name not updated: got '%s'", updatedOpts["name"])
	}
	if updatedOpts["port"] != "1082" {
		t.Errorf("Port not updated: got '%s'", updatedOpts["port"])
	}
}

// TestSOCKSClientSetOptionsValidation tests validation in SetOptions.
// Why: Invalid configurations should be rejected before causing runtime errors.
// Design: Tests multiple validation scenarios with expected error cases.
func TestSOCKSClientSetOptionsValidation(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "sc-validation",
		Type:      "socksclient",
		Port:      1083,
		Interface: "127.0.0.1",
	}

	client, err := NewSocksClient(config, "127.0.0.1:7656")
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
			opts:      map[string]string{"port": "1080", "interface": "127.0.0.1"},
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

// TestSOCKSClientID tests ID generation.
// Why: IDs are used for tunnel identification in management interfaces.
// Design: Verifies ID is generated correctly from tunnel name.
func TestSOCKSClientID(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "sc-id-proxy",
		Type:      "socksclient",
		Port:      1084,
		Interface: "127.0.0.1",
	}

	client, err := NewSocksClient(config, "127.0.0.1:7656")
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

// TestSOCKSClientLocalAddress tests LocalAddress() method.
// Why: Applications need to know where to connect to use the SOCKS proxy.
// Design: Verifies correct host:port formatting.
func TestSOCKSClientLocalAddress(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "sc-localaddr",
		Type:      "socksclient",
		Port:      1085,
		Interface: "127.0.0.1",
	}

	client, err := NewSocksClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	addr, err := client.LocalAddress()
	if err != nil {
		t.Fatalf("Failed to get local address: %v", err)
	}

	expected := net.JoinHostPort("127.0.0.1", "1085")
	if addr != expected {
		t.Errorf("Expected address '%s', got '%s'", expected, addr)
	}
}

// TestSOCKSClientTarget tests Target() method.
// Why: SOCKS clients are one-to-many proxies, so Target should be empty.
// Design: Verifies the method returns empty string for proxy tunnels.
func TestSOCKSClientTarget(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "sc-target",
		Type:      "socksclient",
		Port:      1086,
		Interface: "127.0.0.1",
	}

	client, err := NewSocksClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	target := client.Target()
	if target != "" {
		t.Errorf("SOCKS client should have empty target (one-to-many), got '%s'", target)
	}
}

// TestSOCKSClientLoadConfig tests configuration loading from file.
// Why: Production deployments need to persist and reload configurations.
// Design: Tests successful load, validation, and error cases.
func TestSOCKSClientLoadConfig(t *testing.T) {
	// Create temporary directory for test configs
	tmpDir := t.TempDir()

	// Test case 1: Successful load from YAML
	t.Run("successful load from yaml", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "socks-client.yaml")
		// YAML format requires tunnels: map with tunnel name as key
		configContent := `tunnels:
  loaded-socks-client:
    name: loaded-socks-client
    type: socksclient
    interface: 0.0.0.0
    port: 1090
`
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		client, err := NewSocksClient(i2pconv.TunnelConfig{
			Name:      "sc-lcfg-yaml",
			Type:      "socksclient",
			Port:      1087,
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
		if client.Name() != "loaded-socks-client" {
			t.Errorf("Expected name 'loaded-socks-client', got '%s'", client.Name())
		}
		if client.TunnelConfig.Port != 1090 {
			t.Errorf("Expected port 1090, got %d", client.TunnelConfig.Port)
		}
	})

	// Test case 2: Reject config load while running
	t.Run("reject load while running", func(t *testing.T) {
		client, err := NewSocksClient(i2pconv.TunnelConfig{
			Name:      "sc-lcfg-run",
			Type:      "socksclient",
			Port:      1088,
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
    type: socksclient
    interface: 127.0.0.1
    port: 1080
`
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		err = client.LoadConfig(configPath)
		if err == nil {
			t.Error("Expected error when loading config while running")
		}
	})

	// Test case 3: Reject wrong tunnel type
	t.Run("reject wrong type", func(t *testing.T) {
		client, err := NewSocksClient(i2pconv.TunnelConfig{
			Name:      "sc-lcfg-type",
			Type:      "socksclient",
			Port:      1089,
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
    port: 1080
`
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		err = client.LoadConfig(configPath)
		if err == nil {
			t.Error("Expected error when loading wrong tunnel type")
		}
	})

	// Test case 4: Handle invalid file
	t.Run("invalid file", func(t *testing.T) {
		client, err := NewSocksClient(i2pconv.TunnelConfig{
			Name:      "sc-lcfg-inv",
			Type:      "socksclient",
			Port:      1091,
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

// TestSOCKSClientErrorTracking tests error recording and retrieval.
// Why: Production systems need error visibility for debugging.
// Design: Tests error history tracking and Error() method.
func TestSOCKSClientErrorTracking(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "sc-errors",
		Type:      "socksclient",
		Port:      1092,
		Interface: "127.0.0.1",
	}

	client, err := NewSocksClient(config, "127.0.0.1:7656")
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

// TestSOCKSClientStopBeforeStart tests stopping a non-started tunnel.
// Why: Defensive programming - stop should be safe to call anytime.
// Design: Verifies Stop() is idempotent and doesn't panic.
func TestSOCKSClientStopBeforeStart(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "sc-stop",
		Type:      "socksclient",
		Port:      1093,
		Interface: "127.0.0.1",
	}

	client, err := NewSocksClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	// Stop should not error on non-started tunnel
	if err := client.Stop(); err != nil {
		t.Errorf("Stop should not error on non-started tunnel: %v", err)
	}
}

// TestSOCKSClientPortAllocation tests that the client can bind to available ports.
// Why: Port conflicts are common deployment issues.
// Design: Uses random available port to avoid conflicts in test environment.
func TestSOCKSClientPortAllocation(t *testing.T) {
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
		Name:      "sc-port",
		Type:      "socksclient",
		Port:      port,
		Interface: "127.0.0.1",
	}

	client, err := NewSocksClient(config, "127.0.0.1:7656")
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

// TestSOCKSClientAddress tests the I2P Address() method.
// Why: Address is needed for peer sharing and management interfaces.
// Design: Verifies Address() does not panic and returns consistently.
// Note: Address may be empty if no service keys are configured.
func TestSOCKSClientAddress(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "sc-address",
		Type:      "socksclient",
		Port:      1094,
		Interface: "127.0.0.1",
	}

	client, err := NewSocksClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	// Address() should not panic; returns base32 if ServiceKeys are set,
	// or empty string if no keys are configured in the tunnel config.
	addr := client.Address()
	// Verify consistent behavior: calling Address() twice yields the same result
	addr2 := client.Address()
	if addr != addr2 {
		t.Errorf("Address() not consistent: first=%q, second=%q", addr, addr2)
	}
}
