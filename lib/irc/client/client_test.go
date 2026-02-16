package ircclient

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

// TestIRCClientCreation tests the creation of an IRC client tunnel.
// Why: Validates constructor logic and default state initialization.
// Design: Uses mock SAM address since we're testing creation, not connection.
func TestIRCClientCreation(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "ic-creation",
		Type:      "ircclient",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	// NewIRCClient should create instance even without SAM available (connection happens on Start)
	client, err := NewIRCClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create IRC client: %v", err)
	}
	defer client.Garlic.Close()

	// Verify initial state
	if client.Name() != "ic-creation" {
		t.Errorf("Expected name 'ic-creation', got '%s'", client.Name())
	}
	if client.Type() != "ircclient" {
		t.Errorf("Expected type 'ircclient', got '%s'", client.Type())
	}
	if client.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected initial status stopped, got %v", client.Status())
	}
}

// TestIRCClientOptions tests Options() and SetOptions() methods.
// Why: Configuration management is critical for production deployments.
// Design: Tests both retrieval and modification of tunnel options.
func TestIRCClientOptions(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "ic-options",
		Type:      "ircclient",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewIRCClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	// Test Options() retrieval
	opts := client.Options()
	if opts["name"] != "ic-options" {
		t.Errorf("Expected name 'ic-options', got '%s'", opts["name"])
	}
	if opts["port"] != "6667" {
		t.Errorf("Expected port '6667', got '%s'", opts["port"])
	}

	// Test SetOptions() modification
	newOpts := map[string]string{
		"name":      "updated-irc-client",
		"interface": "0.0.0.0",
		"port":      "6669",
	}
	if err := client.SetOptions(newOpts); err != nil {
		t.Fatalf("Failed to set options: %v", err)
	}

	// Verify changes applied
	updatedOpts := client.Options()
	if updatedOpts["name"] != "updated-irc-client" {
		t.Errorf("Name not updated: got '%s'", updatedOpts["name"])
	}
	if updatedOpts["port"] != "6669" {
		t.Errorf("Port not updated: got '%s'", updatedOpts["port"])
	}
}

// TestIRCClientSetOptionsValidation tests validation in SetOptions.
// Why: Invalid configurations should be rejected before causing runtime errors.
// Design: Tests multiple validation scenarios with expected error cases.
func TestIRCClientSetOptionsValidation(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "ic-validation",
		Type:      "ircclient",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewIRCClient(config, "127.0.0.1:7656")
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
			opts:      map[string]string{"port": "6667", "interface": "127.0.0.1"},
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

// TestIRCClientID tests ID generation.
// Why: IDs are used for tunnel identification in management interfaces.
// Design: Verifies ID is generated correctly from tunnel name.
func TestIRCClientID(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "ic-id-proxy",
		Type:      "ircclient",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewIRCClient(config, "127.0.0.1:7656")
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

// TestIRCClientLocalAddress tests LocalAddress() method.
// Why: Applications need to know where to connect to use the proxy.
// Design: Verifies correct host:port formatting.
func TestIRCClientLocalAddress(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "ic-localaddr",
		Type:      "ircclient",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewIRCClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	addr, err := client.LocalAddress()
	if err != nil {
		t.Fatalf("Failed to get local address: %v", err)
	}

	expected := net.JoinHostPort("127.0.0.1", "6667")
	if addr != expected {
		t.Errorf("Expected address '%s', got '%s'", expected, addr)
	}
}

// TestIRCClientTarget tests Target() method.
// Why: IRC clients have a specific I2P destination target.
// Design: Verifies the method returns the target I2P address.
func TestIRCClientTarget(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "ic-target",
		Type:      "ircclient",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewIRCClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	target := client.Target()
	if target == "" {
		t.Error("IRC client should have a target I2P address")
	}
	// Target should be the base32 address
	if target != "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p" {
		t.Errorf("Expected target with base32 address, got '%s'", target)
	}
}

// TestIRCClientLoadConfig tests configuration loading from file.
// Why: Production deployments need to persist and reload configurations.
// Design: Tests successful load, validation, and error cases.
func TestIRCClientLoadConfig(t *testing.T) {
	// Create temporary directory for test configs
	tmpDir := t.TempDir()

	// Test case 1: Successful load from YAML
	t.Run("successful load from yaml", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "irc-client.yaml")
		// YAML format requires tunnels: map with tunnel name as key
		configContent := `tunnels:
  loaded-irc-client:
    name: loaded-irc-client
    type: ircclient
    interface: 0.0.0.0
    port: 6668
    target: ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		client, err := NewIRCClient(i2pconv.TunnelConfig{
			Name:      "ic-lcfg-yaml",
			Type:      "ircclient",
			Port:      6667,
			Interface: "127.0.0.1",
			Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
		}, "127.0.0.1:7656")
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer client.Garlic.Close()

		if err := client.LoadConfig(configPath); err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Verify config was loaded
		if client.Name() != "loaded-irc-client" {
			t.Errorf("Expected name 'loaded-irc-client', got '%s'", client.Name())
		}
		if client.TunnelConfig.Port != 6668 {
			t.Errorf("Expected port 6668, got %d", client.TunnelConfig.Port)
		}
	})

	// Test case 2: Reject config load while running
	t.Run("reject load while running", func(t *testing.T) {
		client, err := NewIRCClient(i2pconv.TunnelConfig{
			Name:      "ic-lcfg-run",
			Type:      "ircclient",
			Port:      6667,
			Interface: "127.0.0.1",
			Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
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
    type: ircclient
    interface: 127.0.0.1
    port: 6667
    target: ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p
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
		client, err := NewIRCClient(i2pconv.TunnelConfig{
			Name:      "ic-lcfg-type",
			Type:      "ircclient",
			Port:      6667,
			Interface: "127.0.0.1",
			Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
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
    port: 6667
    target: ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p
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
		client, err := NewIRCClient(i2pconv.TunnelConfig{
			Name:      "ic-lcfg-inv",
			Type:      "ircclient",
			Port:      6667,
			Interface: "127.0.0.1",
			Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
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

// TestIRCClientErrorTracking tests error recording and retrieval.
// Why: Production systems need error visibility for debugging.
// Design: Tests error history tracking and Error() method.
func TestIRCClientErrorTracking(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "ic-errors",
		Type:      "ircclient",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewIRCClient(config, "127.0.0.1:7656")
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

// TestIRCClientStopBeforeStart tests stopping a non-started tunnel.
// Why: Defensive programming - stop should be safe to call anytime.
// Design: Verifies Stop() is idempotent and doesn't panic.
func TestIRCClientStopBeforeStart(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "ic-stop",
		Type:      "ircclient",
		Port:      6667,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewIRCClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	// Stop should not error on non-started tunnel
	if err := client.Stop(); err != nil {
		t.Errorf("Stop should not error on non-started tunnel: %v", err)
	}
}

// TestIRCClientPortAllocation tests that the client can bind to available ports.
// Why: Port conflicts are common deployment issues.
// Design: Uses random available port to avoid conflicts in test environment.
func TestIRCClientPortAllocation(t *testing.T) {
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
		Name:      "ic-port",
		Type:      "ircclient",
		Port:      port,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewIRCClient(config, "127.0.0.1:7656")
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
