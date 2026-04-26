package udpserver

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

// TestUDPServerSetOptionsTarget verifies that SetOptions correctly updates the
// target address and that the change is reflected in Target() and Options().
// Why: Finding #7 — target was previously silently dropped by SetOptions.
func TestUDPServerSetOptionsTarget(t *testing.T) {
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8080")
	tunnel := &UDPServer{
		TunnelBase: i2ptunnel.TunnelBase{
		TunnelConfig: i2pconv.TunnelConfig{
			Name:      "us-target",
			Type:      "udpserver",
			Interface: "127.0.0.1",
			Port:      4449,
		},
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		},
		Addr:            addr,
		done:            make(chan struct{}),
	}

	origTarget := tunnel.Target()

	// Change target via SetOptions
	newTarget := "192.168.1.100:3000"
	if err := tunnel.SetOptions(map[string]string{"target": newTarget}); err != nil {
		t.Fatalf("SetOptions with valid target failed: %v", err)
	}

	// Verify Target() returns the new value
	if tunnel.Target() != newTarget {
		t.Errorf("Target() = %q, want %q", tunnel.Target(), newTarget)
	}

	// Verify Options() reflects the new value
	opts := tunnel.Options()
	if opts["target"] != newTarget {
		t.Errorf("Options()[target] = %q, want %q", opts["target"], newTarget)
	}

	// Verify it actually changed from the original
	if tunnel.Target() == origTarget {
		t.Error("Target was not actually updated from original value")
	}
}

// TestUDPServerSetOptionsTargetValidation verifies that invalid target
// addresses are rejected by SetOptions.
func TestUDPServerSetOptionsTargetValidation(t *testing.T) {
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8080")
	tunnel := &UDPServer{
		TunnelBase: i2ptunnel.TunnelBase{
		TunnelConfig: i2pconv.TunnelConfig{
			Name:      "us-validate",
			Type:      "udpserver",
			Interface: "127.0.0.1",
			Port:      4449,
		},
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		},
		Addr:            addr,
		done:            make(chan struct{}),
	}

	tests := []struct {
		name      string
		target    string
		wantError bool
	}{
		{"valid target", "127.0.0.1:3000", false},
		{"valid localhost", "localhost:9090", false},
		{"no port", "127.0.0.1", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tunnel.SetOptions(map[string]string{"target": tt.target})
			if tt.wantError && err == nil {
				t.Errorf("Expected error for target %q, got nil", tt.target)
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no error for target %q, got %v", tt.target, err)
			}
		})
	}
}

// --- Comprehensive tests (constructor-based, matching IRC server patterns) ---

// TestUDPServerCreation tests the creation of a UDP server tunnel.
// Why: Validates constructor logic and default state initialization.
// Design: Uses real SAM address to create a Garlic session; tests struct fields.
func TestUDPServerCreation(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "us-creation",
		Type:      "udpserver",
		Port:      9100,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:8080",
	}

	server, err := NewUDPServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create UDP server: %v", err)
	}
	defer server.Garlic.Close()

	// Verify initial state
	if server.Name() != "us-creation" {
		t.Errorf("Expected name 'us-creation', got '%s'", server.Name())
	}
	if server.Type() != "udpserver" {
		t.Errorf("Expected type 'udpserver', got '%s'", server.Type())
	}
	if server.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected initial status stopped, got %v", server.Status())
	}
}

// TestUDPServerOptions tests Options() and SetOptions() methods.
// Why: Configuration management is critical for production deployments.
// Design: Tests both retrieval and modification of tunnel options.
func TestUDPServerOptions(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "us-options",
		Type:      "udpserver",
		Port:      9101,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewUDPServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	// Test Options() retrieval
	opts := server.Options()
	if opts["name"] != "us-options" {
		t.Errorf("Expected name 'us-options', got '%s'", opts["name"])
	}
	if opts["port"] != "9101" {
		t.Errorf("Expected port '9101', got '%s'", opts["port"])
	}
	if opts["target"] == "" {
		t.Error("Expected non-empty target in options")
	}

	// Test SetOptions() modification
	newOpts := map[string]string{
		"name":      "updated-udp-server",
		"interface": "0.0.0.0",
		"port":      "9102",
	}
	if err := server.SetOptions(newOpts); err != nil {
		t.Fatalf("Failed to set options: %v", err)
	}

	// Verify changes applied
	updatedOpts := server.Options()
	if updatedOpts["name"] != "updated-udp-server" {
		t.Errorf("Name not updated: got '%s'", updatedOpts["name"])
	}
	if updatedOpts["port"] != "9102" {
		t.Errorf("Port not updated: got '%s'", updatedOpts["port"])
	}
}

// TestUDPServerSetOptionsValidationComprehensive tests validation in SetOptions.
// Why: Invalid configurations should be rejected before causing runtime errors.
// Design: Tests multiple validation scenarios with expected error cases.
func TestUDPServerSetOptionsValidationComprehensive(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "us-validation2",
		Type:      "udpserver",
		Port:      9103,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:8080",
	}

	server, err := NewUDPServer(config, "127.0.0.1:7656")
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
			opts:      map[string]string{"port": "9100", "interface": "127.0.0.1"},
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
		{
			name:      "valid target update",
			opts:      map[string]string{"target": "127.0.0.1:5000"},
			wantError: false,
		},
		{
			name:      "invalid target - no port",
			opts:      map[string]string{"target": "127.0.0.1"},
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

// TestUDPServerID tests ID generation.
// Why: IDs are used for tunnel identification in management interfaces.
// Design: Verifies ID is generated correctly from tunnel name.
func TestUDPServerID(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "us-id-test",
		Type:      "udpserver",
		Port:      9104,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:8080",
	}

	server, err := NewUDPServer(config, "127.0.0.1:7656")
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

// TestUDPServerLocalAddress tests LocalAddress() method.
// Why: Management interfaces need to know the listening address.
// Design: Verifies correct host:port formatting.
func TestUDPServerLocalAddress(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "us-localaddr",
		Type:      "udpserver",
		Port:      9105,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewUDPServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	addr, err := server.LocalAddress()
	if err != nil {
		t.Fatalf("Failed to get local address: %v", err)
	}

	expected := net.JoinHostPort("127.0.0.1", "9105")
	if addr != expected {
		t.Errorf("Expected address '%s', got '%s'", expected, addr)
	}
}

// TestUDPServerTargetComprehensive tests Target() method via constructor.
// Why: UDP server forwards to a specific local service target.
// Design: Verifies the target address reflects the configured forward target.
func TestUDPServerTargetComprehensive(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "us-target2",
		Type:      "udpserver",
		Port:      9106,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewUDPServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	target := server.Target()
	// Target() should return the resolved local forward target address
	expectedTarget := "127.0.0.1:9090"
	if target != expectedTarget {
		t.Errorf("Expected target '%s', got '%s'", expectedTarget, target)
	}
}

// TestUDPServerLoadConfig tests configuration loading from file.
// Why: Production deployments need to persist and reload configurations.
// Design: Tests successful load, validation, and error cases.
func TestUDPServerLoadConfig(t *testing.T) {
	// Create temporary directory for test configs
	tmpDir := t.TempDir()

	// Test case 1: Successful load from YAML
	t.Run("successful load from yaml", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "udp-server.yaml")
		configContent := `tunnels:
  loaded-udp-server:
    name: loaded-udp-server
    type: udpserver
    interface: 0.0.0.0
    port: 9110
    target: 127.0.0.1:9999
`
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		server, err := NewUDPServer(i2pconv.TunnelConfig{
			Name:      "us-lcfg-yaml",
			Type:      "udpserver",
			Port:      9107,
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
		if server.Name() != "loaded-udp-server" {
			t.Errorf("Expected name 'loaded-udp-server', got '%s'", server.Name())
		}
		if server.TunnelConfig.Port != 9110 {
			t.Errorf("Expected port 9110, got %d", server.TunnelConfig.Port)
		}
		if server.Target() != "127.0.0.1:9999" {
			t.Errorf("Expected target '127.0.0.1:9999', got '%s'", server.Target())
		}
	})

	// Test case 2: Reject config load while running
	t.Run("reject load while running", func(t *testing.T) {
		server, err := NewUDPServer(i2pconv.TunnelConfig{
			Name:      "us-lcfg-run",
			Type:      "udpserver",
			Port:      9108,
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
    type: udpserver
    interface: 127.0.0.1
    port: 9100
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
		server, err := NewUDPServer(i2pconv.TunnelConfig{
			Name:      "us-lcfg-type",
			Type:      "udpserver",
			Port:      9109,
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
    type: tcpserver
    interface: 127.0.0.1
    port: 9100
    target: 127.0.0.1:8080
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
		server, err := NewUDPServer(i2pconv.TunnelConfig{
			Name:      "us-lcfg-tgt",
			Type:      "udpserver",
			Port:      9111,
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
    type: udpserver
    interface: 127.0.0.1
    port: 9100
    target: invalid-target-no-port
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
		server, err := NewUDPServer(i2pconv.TunnelConfig{
			Name:      "us-lcfg-inv",
			Type:      "udpserver",
			Port:      9112,
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

// TestUDPServerErrorTracking tests error recording and retrieval.
// Why: Production systems need error visibility for debugging.
// Design: Tests error history tracking and Error() method.
func TestUDPServerErrorTracking(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "us-errors",
		Type:      "udpserver",
		Port:      9113,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewUDPServer(config, "127.0.0.1:7656")
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
	server.RecordError(testErr)

	// Verify error was recorded
	if server.Error() == nil {
		t.Error("Expected error to be recorded")
	}
	if len(server.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(server.Errors))
	}
}

// TestUDPServerStopBeforeStart tests stopping a non-started tunnel.
// Why: Defensive programming - stop should be safe to call anytime.
// Design: Verifies Stop() is idempotent and doesn't panic.
func TestUDPServerStopBeforeStart(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "us-stop",
		Type:      "udpserver",
		Port:      9114,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewUDPServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	// Stop should not panic on non-started tunnel
	if err := server.Stop(); err != nil {
		t.Errorf("Stop should not error on non-started tunnel: %v", err)
	}
}

// TestUDPServerPortAllocation tests that the server can bind to available ports.
// Why: Port conflicts are common deployment issues.
// Design: Uses random available port to avoid conflicts in test environment.
func TestUDPServerPortAllocation(t *testing.T) {
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
		Name:      "us-port",
		Type:      "udpserver",
		Port:      port,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewUDPServer(config, "127.0.0.1:7656")
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

// TestUDPServerAddress tests the I2P Address() method.
// Why: Address is needed for peer sharing and management interfaces.
// Design: Verifies Address() does not panic and returns consistently.
// Note: Address may be empty if no service keys are configured.
func TestUDPServerAddress(t *testing.T) {
	config := i2pconv.TunnelConfig{
		Name:      "us-address",
		Type:      "udpserver",
		Port:      9115,
		Interface: "127.0.0.1",
		Target:    "127.0.0.1:9090",
	}

	server, err := NewUDPServer(config, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Garlic.Close()

	// Address() should not panic; returns base32 if ServiceKeys are set,
	// or empty string if no keys are configured in the tunnel config.
	addr := server.Address()
	// Verify consistent behavior: calling Address() twice yields the same result
	addr2 := server.Address()
	if addr != addr2 {
		t.Errorf("Address() not consistent: first=%q, second=%q", addr, addr2)
	}
}
