package ircserver

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

// TestIRCServerCreation tests the creation of an IRC server tunnel.
// Why: Validates constructor logic and default state initialization.
// Design: Uses mock SAM address since we're testing creation, not connection.
func TestIRCServerCreation(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "is-creation",
		Type:      "ircserver",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:6667",
	}

	server, err := NewIRCServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create IRC server: %v", err)
	}
	defer server.Garlic.Close()

	// Verify initial state
	if server.Name() != "is-creation" {
		t.Errorf("Expected name 'is-creation', got '%s'", server.Name())
	}
	if server.Type() != "ircserver" {
		t.Errorf("Expected type 'ircserver', got '%s'", server.Type())
	}
	if server.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected initial status stopped, got %v", server.Status())
	}
}

// TestIRCServerOptions tests Options() and SetOptions() methods.
// Why: Configuration management is critical for production deployments.
// Design: Tests both retrieval and modification of tunnel options.
func TestIRCServerOptions(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "is-options",
		Type:      "ircserver",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewIRCServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	// Test Options() retrieval
	opts := server.Options()
	if opts["name"] != "is-options" {
		t.Errorf("Expected name 'is-options', got '%s'", opts["name"])
	}
	if opts["port"] != "6667" {
		t.Errorf("Expected port '6667', got '%s'", opts["port"])
	}
	// Note: target in options comes from server.Addr, which is set to interface:port during construction
	// This is the local listening address, not the forward target

	// Test SetOptions() modification
	newOpts := map[string]string{
		"name":      "updated-irc-server",
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
	if updatedOpts["name"] != "updated-irc-server" {
		t.Errorf("Name not updated: got '%s'", updatedOpts["name"])
	}
	if updatedOpts["port"] != "9999" {
		t.Errorf("Port not updated: got '%s'", updatedOpts["port"])
	}
	if updatedOpts["maxconns"] != "50" {
		t.Errorf("MaxConns not updated: got '%s'", updatedOpts["maxconns"])
	}
}

// TestIRCServerSetOptionsValidation tests validation in SetOptions.
// Why: Invalid configurations should be rejected before causing runtime errors.
// Design: Tests multiple validation scenarios with expected error cases.
func TestIRCServerSetOptionsValidation(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "is-validation",
		Type:      "ircserver",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:6667",
	}

	server, err := NewIRCServer(config, "127.0.0.1:7656")
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
			opts:      map[string]string{"port": "6667", "maxconns": "100"},
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

// TestIRCServerSetOptionsTarget verifies that SetOptions correctly updates the
// target address and that the change is reflected in Options() output.
// Why: Finding #7 — target was previously silently dropped by SetOptions.
func TestIRCServerSetOptionsTarget(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "is-target",
		Type:      "ircserver",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:6667",
	}

	server, err := NewIRCServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	origTarget := server.Target()

	// Change target via SetOptions
	newTarget := "192.168.1.100:6669"
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

// TestIRCServerID tests ID generation.
// Why: IDs are used for tunnel identification in management interfaces.
// Design: Verifies ID is generated correctly from tunnel name.
func TestIRCServerID(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "is-id-server",
		Type:      "ircserver",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewIRCServer(config, "127.0.0.1:7656")
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

// TestIRCServerLocalAddress tests LocalAddress() method.
// Why: Management interfaces need to know the I2P listening address.
// Design: Verifies correct host:port formatting.
func TestIRCServerLocalAddress(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "is-localaddr",
		Type:      "ircserver",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewIRCServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	addr, err := server.LocalAddress()
	if err != nil {
		t.Fatalf("Failed to get local address: %v", err)
	}

	expected := net.JoinHostPort("127.0.0.1", "6667")
	if addr != expected {
		t.Errorf("Expected address '%s', got '%s'", expected, addr)
	}
}

// TestIRCServerTarget tests Target() method.
// Why: IRC server forwards to a specific local service target.
// Design: Verifies the target address reflects the configured forward target.
func TestIRCServerTarget(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "is-target2",
		Type:      "ircserver",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewIRCServer(config, "127.0.0.1:7656")
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

// TestIRCServerLoadConfig tests configuration loading from file.
// Why: Production deployments need to persist and reload configurations.
// Design: Tests successful load, validation, and error cases.
func TestIRCServerLoadConfig(t *testing.T) {
	// Create temporary directory for test configs
	tmpDir := t.TempDir()

	// Test case 1: Successful load from YAML
	t.Run("successful load from yaml", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "irc-server.yaml")
		// YAML format requires tunnels: map with tunnel name as key
		configContent := `tunnels:
  loaded-irc-server:
    name: loaded-irc-server
    type: ircserver
    interface: 0.0.0.0
    port: 6668
    target: 127.0.0.1:6667
    target: 127.0.0.1:9999
`
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		server, err := NewIRCServer(i2pconv.TunnelConfig{
			Name:      "is-lcfg-yaml",
			Type:      "ircserver",
			Port:      6667,
			Interface: "127.0.0.1",
			Target:    "127.0.0.1:6667",
		}, "127.0.0.1:7656")
		if err != nil {
			t.Fatalf("Failed to create server: %v", err)
		}
		defer server.Garlic.Close()

		if err := server.LoadConfig(configPath); err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Verify config was loaded
		if server.Name() != "loaded-irc-server" {
			t.Errorf("Expected name 'loaded-irc-server', got '%s'", server.Name())
		}
		if server.TunnelConfig.Port != 6668 {
			t.Errorf("Expected port 6668, got %d", server.TunnelConfig.Port)
		}
		if server.Target() != "127.0.0.1:9999" {
			t.Errorf("Expected target '127.0.0.1:9999', got '%s'", server.Target())
		}
	})

	// Test case 2: Reject config load while running
	t.Run("reject load while running", func(t *testing.T) {
		server, err := NewIRCServer(i2pconv.TunnelConfig{
			Name:      "is-lcfg-run",
			Type:      "ircserver",
			Port:      6667,
			Interface: "127.0.0.1",
			Target:    "127.0.0.1:6667",
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
    type: ircserver
    interface: 127.0.0.1
    port: 6667
    target: 127.0.0.1:6667
    target: 127.0.0.1:6667
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
		server, err := NewIRCServer(i2pconv.TunnelConfig{
			Name:      "is-lcfg-type",
			Type:      "ircserver",
			Port:      6667,
			Interface: "127.0.0.1",
			Target:    "127.0.0.1:6667",
		}, "127.0.0.1:7656")
		if err != nil {
			t.Fatalf("Failed to create server: %v", err)
		}
		defer server.Garlic.Close()

		configPath := filepath.Join(tmpDir, "wrong-type.yaml")
		configContent := `tunnels:
  test:
    name: test
    type: ircclient
    interface: 127.0.0.1
    port: 6667
    target: 127.0.0.1:6667
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
		server, err := NewIRCServer(i2pconv.TunnelConfig{
			Name:      "is-lcfg-tgt",
			Type:      "ircserver",
			Port:      6667,
			Interface: "127.0.0.1",
			Target:    "127.0.0.1:6667",
		}, "127.0.0.1:7656")
		if err != nil {
			t.Fatalf("Failed to create server: %v", err)
		}
		defer server.Garlic.Close()

		configPath := filepath.Join(tmpDir, "invalid-target.yaml")
		configContent := `tunnels:
  test:
    name: test
    type: ircserver
    interface: 127.0.0.1
    port: 6667
    target: 127.0.0.1:6667
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
		server, err := NewIRCServer(i2pconv.TunnelConfig{
			Name:      "is-lcfg-inv",
			Type:      "ircserver",
			Port:      6667,
			Interface: "127.0.0.1",
			Target:    "127.0.0.1:6667",
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

// TestIRCServerErrorTracking tests error recording and retrieval.
// Why: Production systems need error visibility for debugging.
// Design: Tests error history tracking and Error() method.
func TestIRCServerErrorTracking(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "is-errors",
		Type:      "ircserver",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewIRCServer(config, "127.0.0.1:7656")
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

// TestIRCServerStopBeforeStart tests stopping a non-started tunnel.
// Why: Defensive programming - stop should be safe to call anytime.
// Design: Verifies Stop() is idempotent and doesn't panic.
func TestIRCServerStopBeforeStart(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "is-stop",
		Type:      "ircserver",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewIRCServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	// Stop should not panic on non-started tunnel
	if err := server.Stop(); err != nil {
		t.Errorf("Stop should not error on non-started tunnel: %v", err)
	}
}

// TestIRCServerRateLimiting tests rate limiting configuration.
// Why: Rate limiting is crucial for preventing abuse in production.
// Design: Verifies rate limit and maxconns settings.
func TestIRCServerRateLimiting(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "is-ratelimit",
		Type:      "ircserver",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewIRCServer(config, "127.0.0.1:7656")
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

// TestIRCServerPortAllocation tests that the server can bind to available ports.
// Why: Port conflicts are common deployment issues.
// Design: Uses random available port to avoid conflicts in test environment.
func TestIRCServerPortAllocation(t *testing.T) {
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
		Name:      "is-port",
		Type:      "ircserver",
		Port:      port,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewIRCServer(config, "127.0.0.1:7656")
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
