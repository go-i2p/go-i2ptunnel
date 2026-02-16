package udpserver

import (
	"net"
	"testing"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// TestUDPServerSetOptionsTarget verifies that SetOptions correctly updates the
// target address and that the change is reflected in Target() and Options().
// Why: Finding #7 — target was previously silently dropped by SetOptions.
func TestUDPServerSetOptionsTarget(t *testing.T) {
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8080")
	tunnel := &UDPServer{
		TunnelConfig: i2pconv.TunnelConfig{
			Name:      "us-target",
			Type:      "udpserver",
			Interface: "127.0.0.1",
			Port:      4449,
		},
		Addr:            addr,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
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
		TunnelConfig: i2pconv.TunnelConfig{
			Name:      "us-validate",
			Type:      "udpserver",
			Interface: "127.0.0.1",
			Port:      4449,
		},
		Addr:            addr,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
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
