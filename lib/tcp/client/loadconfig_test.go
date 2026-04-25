package tcpclient

import (
	"os"
	"path/filepath"
	"testing"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// TestLoadConfig_Success verifies LoadConfig correctly loads and applies configuration
func TestLoadConfig_Success(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tunnel.yaml")

	// YAML format requires tunnels: map with tunnel name as key
	configContent := `tunnels:
  test-tcp-client:
    name: test-tcp-client
    type: tcpclient
    interface: 127.0.0.1
    port: 9999
    target: ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Create a stopped tunnel
	initialConfig := i2pconv.TunnelConfig{
		Name:      "tclc-success",
		Type:      "tcpclient",
		Interface: "0.0.0.0",
		Port:      8888,
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	tunnel, err := NewTCPClient(initialConfig, "localhost:7656")
	if err != nil {
		t.Fatalf("Failed to create tunnel: %v", err)
	}
	defer tunnel.Garlic.Close()

	// Ensure tunnel is stopped
	tunnel.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped

	// Load new configuration
	if err := tunnel.LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify configuration was updated
	if tunnel.TunnelConfig.Name != "test-tcp-client" {
		t.Errorf("Expected name 'test-tcp-client', got '%s'", tunnel.TunnelConfig.Name)
	}
	if tunnel.TunnelConfig.Port != 9999 {
		t.Errorf("Expected port 9999, got %d", tunnel.TunnelConfig.Port)
	}
	if tunnel.TunnelConfig.Interface != "127.0.0.1" {
		t.Errorf("Expected interface '127.0.0.1', got '%s'", tunnel.TunnelConfig.Interface)
	}
}

// TestLoadConfig_RunningTunnel verifies LoadConfig fails when tunnel is running
func TestLoadConfig_RunningTunnel(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tunnel.yaml")

	configContent := `tunnels:
  test-tunnel:
    name: test-tunnel
    type: tcpclient
    interface: 127.0.0.1
    port: 9999
    target: ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	initialConfig := i2pconv.TunnelConfig{
		Name:      "tclc-running",
		Type:      "tcpclient",
		Interface: "0.0.0.0",
		Port:      8888,
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	tunnel, err := NewTCPClient(initialConfig, "localhost:7656")
	if err != nil {
		t.Fatalf("Failed to create tunnel: %v", err)
	}
	defer tunnel.Garlic.Close()

	// Set tunnel to running state
	tunnel.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning

	// Attempt to load config should fail
	err = tunnel.LoadConfig(configPath)
	if err == nil {
		t.Fatal("Expected LoadConfig to fail for running tunnel, but it succeeded")
	}

	// Verify error message contains expected text
	if err.Error() != "cannot load config while tunnel is running - stop tunnel first" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
} // TestLoadConfig_InvalidFile verifies LoadConfig handles missing files gracefully
func TestLoadConfig_InvalidFile(t *testing.T) {
	initialConfig := i2pconv.TunnelConfig{
		Name:      "tclc-invfile",
		Type:      "tcpclient",
		Interface: "0.0.0.0",
		Port:      8888,
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	tunnel, err := NewTCPClient(initialConfig, "localhost:7656")
	if err != nil {
		t.Fatalf("Failed to create tunnel: %v", err)
	}
	defer tunnel.Garlic.Close()

	tunnel.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped

	// Try to load non-existent file
	err = tunnel.LoadConfig("/nonexistent/path/tunnel.yaml")
	if err == nil {
		t.Fatal("Expected LoadConfig to fail for non-existent file, but it succeeded")
	}
}

// TestLoadConfig_WrongType verifies LoadConfig rejects config with wrong tunnel type
func TestLoadConfig_WrongType(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tunnel.yaml")

	// Create config with tcpserver type instead of tcpclient
	configContent := `tunnels:
  test-tunnel:
    name: test-tunnel
    type: tcpserver
    interface: 127.0.0.1
    port: 9999
    target: localhost:80
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	initialConfig := i2pconv.TunnelConfig{
		Name:      "tclc-wrongtype",
		Type:      "tcpclient",
		Interface: "0.0.0.0",
		Port:      8888,
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	tunnel, err := NewTCPClient(initialConfig, "localhost:7656")
	if err != nil {
		t.Fatalf("Failed to create tunnel: %v", err)
	}
	defer tunnel.Garlic.Close()

	tunnel.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped

	// Try to load config with wrong type
	err = tunnel.LoadConfig(configPath)
	if err == nil {
		t.Fatal("Expected LoadConfig to fail for wrong tunnel type, but it succeeded")
	}

	// Verify error message mentions type mismatch
	expectedMsg := "config file contains tcpserver tunnel, expected tcpclient"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
} // TestLoadConfig_InvalidTarget verifies LoadConfig validates target addresses
func TestLoadConfig_InvalidTarget(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tunnel.yaml")

	// Create config with invalid I2P address
	configContent := `tunnels:
  test-tunnel:
    name: test-tunnel
    type: tcpclient
    interface: 127.0.0.1
    port: 9999
    target: invalid-address-not-base32
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	initialConfig := i2pconv.TunnelConfig{
		Name:      "tclc-invtarget",
		Type:      "tcpclient",
		Interface: "0.0.0.0",
		Port:      8888,
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	tunnel, err := NewTCPClient(initialConfig, "localhost:7656")
	if err != nil {
		t.Fatalf("Failed to create tunnel: %v", err)
	}
	defer tunnel.Garlic.Close()

	tunnel.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped

	// Try to load config with invalid target
	err = tunnel.LoadConfig(configPath)
	if err == nil {
		t.Fatal("Expected LoadConfig to fail for invalid target address, but it succeeded")
	}
}

// TestLoadConfig_PropertiesFormat verifies LoadConfig works with .properties format
func TestLoadConfig_PropertiesFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tunnel.properties")

	// Create .properties format config
	// Properties format uses different field names: listenPort instead of port
	configContent := `type=tcpclient
name=test-properties-tunnel
interface=127.0.0.1
listenPort=7777
targetDestination=ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	initialConfig := i2pconv.TunnelConfig{
		Name:      "tclc-props",
		Type:      "tcpclient",
		Interface: "0.0.0.0",
		Port:      8888,
		Target:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p",
	}

	tunnel, err := NewTCPClient(initialConfig, "localhost:7656")
	if err != nil {
		t.Fatalf("Failed to create tunnel: %v", err)
	}
	defer tunnel.Garlic.Close()

	tunnel.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped

	// Load properties config
	if err := tunnel.LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig failed for .properties format: %v", err)
	}

	// Verify config was loaded
	if tunnel.TunnelConfig.Port != 7777 {
		t.Errorf("Expected port 7777, got %d", tunnel.TunnelConfig.Port)
	}
}
