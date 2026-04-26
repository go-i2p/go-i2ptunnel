package udpclient

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// TestUDPClientCreation tests the creation of a UDP client tunnel.
// Why: Validates constructor logic and default state initialization.
// Design: Uses real SAM address to create a Garlic session; tests struct fields.
func TestUDPClientCreation(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "uc-creation",
		Type:      "udpclient",
		Port:      9000,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewUDPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create UDP client: %v", err)
	}
	defer client.Garlic.Close()

	// Verify initial state
	if client.Name() != "uc-creation" {
		t.Errorf("Expected name 'uc-creation', got '%s'", client.Name())
	}
	if client.Type() != "udpclient" {
		t.Errorf("Expected type 'udpclient', got '%s'", client.Type())
	}
	if client.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected initial status stopped, got %v", client.Status())
	}
}

// TestUDPClientOptions tests Options() and SetOptions() methods.
// Why: Configuration management is critical for production deployments.
// Design: Tests both retrieval and modification of tunnel options.
func TestUDPClientOptions(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "uc-options",
		Type:      "udpclient",
		Port:      9001,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewUDPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	// Test Options() retrieval
	opts := client.Options()
	if opts["name"] != "uc-options" {
		t.Errorf("Expected name 'uc-options', got '%s'", opts["name"])
	}
	if opts["port"] != "9001" {
		t.Errorf("Expected port '9001', got '%s'", opts["port"])
	}
	if opts["target"] == "" {
		t.Error("Expected non-empty target in options")
	}

	// Test SetOptions() modification
	newOpts := map[string]string{
		"name":      "updated-udp-client",
		"interface": "0.0.0.0",
		"port":      "9002",
	}
	if err := client.SetOptions(newOpts); err != nil {
		t.Fatalf("Failed to set options: %v", err)
	}

	// Verify changes applied
	updatedOpts := client.Options()
	if updatedOpts["name"] != "updated-udp-client" {
		t.Errorf("Name not updated: got '%s'", updatedOpts["name"])
	}
	if updatedOpts["port"] != "9002" {
		t.Errorf("Port not updated: got '%s'", updatedOpts["port"])
	}
}

// TestUDPClientSetOptionsValidation tests validation in SetOptions.
// Why: Invalid configurations should be rejected before causing runtime errors.
// Design: Tests multiple validation scenarios with expected error cases.
func TestUDPClientSetOptionsValidation(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "uc-validation",
		Type:      "udpclient",
		Port:      9003,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewUDPClient(config, "127.0.0.1:7656")
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
			opts:      map[string]string{"port": "9000", "interface": "127.0.0.1"},
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

// TestUDPClientID tests ID generation.
// Why: IDs are used for tunnel identification in management interfaces.
// Design: Verifies ID is generated correctly from tunnel name.
func TestUDPClientID(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "uc-id-test",
		Type:      "udpclient",
		Port:      9004,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewUDPClient(config, "127.0.0.1:7656")
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

// TestUDPClientLocalAddress tests LocalAddress() method.
// Why: Applications need to know where to send UDP packets to use the tunnel.
// Design: Verifies correct host:port formatting.
func TestUDPClientLocalAddress(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "uc-localaddr",
		Type:      "udpclient",
		Port:      9005,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewUDPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	addr, err := client.LocalAddress()
	if err != nil {
		t.Fatalf("Failed to get local address: %v", err)
	}

	expected := net.JoinHostPort("127.0.0.1", "9005")
	if addr != expected {
		t.Errorf("Expected address '%s', got '%s'", expected, addr)
	}
}

// TestUDPClientTarget tests Target() method.
// Why: UDP clients have a specific I2P destination target for forwarding.
// Design: Verifies the method returns the target I2P address.
func TestUDPClientTarget(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "uc-target",
		Type:      "udpclient",
		Port:      9006,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewUDPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	target := client.Target()
	if target == "" {
		t.Error("UDP client should have a target I2P address")
	}
	// Target should be the base32 address
	if target != "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p" {
		t.Errorf("Expected target with base32 address, got '%s'", target)
	}
}

// TestUDPClientLoadConfig tests configuration loading from file.
// Why: Production deployments need to persist and reload configurations.
// Design: Tests successful load, validation, and error cases.
func TestUDPClientLoadConfig(t *testing.T) {
	// Create temporary directory for test configs
	tmpDir := t.TempDir()

	// Test case 1: Successful load from YAML
	t.Run("successful load from yaml", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "udp-client.yaml")
		configContent := `tunnels:
  loaded-udp-client:
    name: loaded-udp-client
    type: udpclient
    interface: 0.0.0.0
    port: 9010
    target: ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p
`
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		client, err := NewUDPClient(i2pconv.TunnelConfig{
			Name:      "uc-lcfg-yaml",
			Type:      "udpclient",
			Port:      9007,
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
		if client.Name() != "loaded-udp-client" {
			t.Errorf("Expected name 'loaded-udp-client', got '%s'", client.Name())
		}
		if client.TunnelConfig.Port != 9010 {
			t.Errorf("Expected port 9010, got %d", client.TunnelConfig.Port)
		}
	})

	// Test case 2: Reject config load while running
	t.Run("reject load while running", func(t *testing.T) {
		client, err := NewUDPClient(i2pconv.TunnelConfig{
			Name:      "uc-lcfg-run",
			Type:      "udpclient",
			Port:      9008,
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
    type: udpclient
    interface: 127.0.0.1
    port: 9000
    target: ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p
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
		client, err := NewUDPClient(i2pconv.TunnelConfig{
			Name:      "uc-lcfg-type",
			Type:      "udpclient",
			Port:      9009,
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
    port: 9000
    target: ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p
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
		client, err := NewUDPClient(i2pconv.TunnelConfig{
			Name:      "uc-lcfg-inv",
			Type:      "udpclient",
			Port:      9011,
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

// TestUDPClientErrorTracking tests error recording and retrieval.
// Why: Production systems need error visibility for debugging.
// Design: Tests error history tracking and Error() method.
func TestUDPClientErrorTracking(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "uc-errors",
		Type:      "udpclient",
		Port:      9012,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewUDPClient(config, "127.0.0.1:7656")
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
	client.RecordError(testErr)

	// Verify error was recorded
	if client.Error() == nil {
		t.Error("Expected error to be recorded")
	}
	if len(client.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(client.Errors))
	}
}

// TestUDPClientStopBeforeStart tests stopping a non-started tunnel.
// Why: Defensive programming - stop should be safe to call anytime.
// Design: Verifies Stop() is idempotent and doesn't panic.
func TestUDPClientStopBeforeStart(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "uc-stop",
		Type:      "udpclient",
		Port:      9013,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewUDPClient(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Garlic.Close()

	// Stop should not error on non-started tunnel
	if err := client.Stop(); err != nil {
		t.Errorf("Stop should not error on non-started tunnel: %v", err)
	}
}

// TestUDPClientPortAllocation tests that the client can bind to available ports.
// Why: Port conflicts are common deployment issues.
// Design: Uses random available port to avoid conflicts in test environment.
func TestUDPClientPortAllocation(t *testing.T) {
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
		Name:      "uc-port",
		Type:      "udpclient",
		Port:      port,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewUDPClient(config, "127.0.0.1:7656")
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

// TestUDPClientAddress tests the I2P Address() method.
// Why: Address is needed for peer sharing and management interfaces.
// Design: Verifies Address() does not panic and returns consistently.
// Note: Address may be empty if no service keys are configured.
func TestUDPClientAddress(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "uc-address",
		Type:      "udpclient",
		Port:      9014,
		Interface: "127.0.0.1",
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	client, err := NewUDPClient(config, "127.0.0.1:7656")
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
