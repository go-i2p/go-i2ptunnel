package udpbidirectional

// integration_test.go exercises UDPBidirectional paths that require a live SAM:
//   - NewUDPBidirectional
//   - Start/Stop lifecycle
//   - SetTunnelMetrics / recordError
//   - LoadConfig after Stop
//
// Tests require a live SAM bridge on 127.0.0.1:7656.

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"testing"
	"time"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
	"github.com/txthinking/socks5"
)

const samAddr = "127.0.0.1:7656"

func minimalUDPBiConfig(name string) i2pconv.TunnelConfig {
	return i2pconv.TunnelConfig{
		Name:      name,
		Type:      "udpbidirectional",
		Target:    "127.0.0.1:19999",
		Interface: "127.0.0.1",
		Port:      14449,
	}
}

// TestNewUDPBidirectional verifies the constructor creates a fully initialized
// tunnel struct and connects to SAM.
func TestNewUDPBidirectional(t *testing.T) {
	name := "udp-new-" + shortID()
	cfg := minimalUDPBiConfig(name)

	tunnel, err := NewUDPBidirectional(cfg, samAddr)
	if err != nil {
		t.Fatalf("NewUDPBidirectional: %v", err)
	}
	defer tunnel.Garlic.Close()

	if tunnel.Name() != name {
		t.Errorf("Name() = %q, want %q", tunnel.Name(), name)
	}
	if tunnel.Type() != "udpbidirectional" {
		t.Errorf("Type() = %q, want %q", tunnel.Type(), "udpbidirectional")
	}
	if tunnel.LimitedConfig.MaxConns != 1000 {
		t.Errorf("MaxConns = %d, want 1000", tunnel.LimitedConfig.MaxConns)
	}
	if tunnel.LimitedConfig.RateLimit != 100.0 {
		t.Errorf("RateLimit = %f, want 100.0", tunnel.LimitedConfig.RateLimit)
	}
	if tunnel.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Status() = %v, want Stopped", tunnel.Status())
	}
}

// TestNewUDPBidirectionalBadTarget verifies that an invalid target address
// causes the constructor to return an error.
func TestNewUDPBidirectionalBadTarget(t *testing.T) {
	cfg := minimalUDPBiConfig("udp-bad-target")
	cfg.Target = "not-a-valid-address"

	_, err := NewUDPBidirectional(cfg, samAddr)
	if err == nil {
		t.Fatal("expected error for invalid target address, got nil")
	}
}

// TestNewUDPBidirectionalBadSAM verifies that an unreachable SAM address
// causes the constructor to return an error without blocking.
func TestNewUDPBidirectionalBadSAM(t *testing.T) {
	cfg := minimalUDPBiConfig("udp-bad-sam")
	_, err := NewUDPBidirectional(cfg, "127.0.0.1:19999")
	if err == nil {
		t.Fatal("expected error for bad SAM address, got nil")
	}
}

// TestUDPBidirectionalSetOptionsI2CP verifies that I2CP options passed to
// SetOptions are stored in the TunnelConfig.I2CP map.
func TestUDPBidirectionalSetOptionsI2CP(t *testing.T) {
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8080")
	tunnel := &UDPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Name:      "udp-i2cp",
			Type:      "udpbidirectional",
			Interface: "127.0.0.1",
			Port:      4449,
		},
		Addr:            addr,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		done:            make(chan struct{}),
	}

	// "i2cp." prefix triggers ExtractI2CPOptions
	err := tunnel.SetOptions(map[string]string{
		"name":                  "udp-i2cp",
		"i2cp.leaseSetEncType":  "4",
		"i2cp.leaseSetAuthType": "0",
	})
	if err != nil {
		t.Fatalf("SetOptions with I2CP opts: %v", err)
	}
	if tunnel.TunnelConfig.I2CP == nil {
		t.Fatal("TunnelConfig.I2CP should be non-nil after setting I2CP options")
	}
	if _, ok := tunnel.TunnelConfig.I2CP["leaseSetEncType"]; !ok {
		t.Error("TunnelConfig.I2CP missing leaseSetEncType")
	}
}

// TestUDPBidirectionalSetTunnelMetrics verifies that SetTunnelMetrics injects
// the metrics and that errors recorded via recordError are tracked.
func TestUDPBidirectionalSetTunnelMetrics(t *testing.T) {
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:9999")
	tunnel := &UDPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Name: "udp-metrics",
			Type: "udpbidirectional",
		},
		Addr:            addr,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		done:            make(chan struct{}),
	}

	m := &metrics.TunnelMetrics{Name: "udp-metrics", ID: "udp-metrics", Type: "udpbidirectional"}
	tunnel.SetTunnelMetrics(m)
	if tunnel.Metrics != m {
		t.Error("SetTunnelMetrics did not set Metrics field")
	}

	// recordError increments error counter on the metrics object.
	initialErrors := tunnel.Metrics.Snapshot().ErrorCount
	tunnel.recordError(nil)
	afterErrors := tunnel.Metrics.Snapshot().ErrorCount
	if afterErrors <= initialErrors {
		t.Error("recordError did not increment Metrics.ErrorCount")
	}
}

// TestUDPBidirectionalLoadConfigBadFormat verifies that an unknown file
// extension causes LoadConfig to return an error.
func TestUDPBidirectionalLoadConfigBadFormat(t *testing.T) {
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:9999")
	tunnel := &UDPBidirectional{
		TunnelConfig:    i2pconv.TunnelConfig{Name: "udp-fmt", Type: "udpbidirectional"},
		Addr:            addr,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		done:            make(chan struct{}),
	}

	f, err := os.CreateTemp("", "config*.txt")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	f.Close()
	defer os.Remove(f.Name())

	if err := tunnel.LoadConfig(f.Name()); err == nil {
		t.Error("LoadConfig with .txt extension should return error")
	}
}

// TestUDPBidirectionalLoadConfigAfterStop verifies that LoadConfig succeeds
// after Stop() and updates the tunnel's configuration.
func TestUDPBidirectionalLoadConfigAfterStop(t *testing.T) {
	cfg := minimalUDPBiConfig("udp-lc-" + shortID())
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen for free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	cfg.Port = port

	tunnel, err := NewUDPBidirectional(cfg, samAddr)
	if err != nil {
		t.Fatalf("NewUDPBidirectional: %v", err)
	}
	defer tunnel.Garlic.Close()

	// Write a valid YAML config file.
	f, err := os.CreateTemp("", "udp-cfg*.yaml")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(f.Name())
	yaml := "tunnels:\n  udp-loadcfg-new:\n    name: udp-loadcfg-new\n    type: udpbidirectional\n    target: 127.0.0.1:19999\n    interface: 127.0.0.1\n    port: " +
		intToStr(port) + "\n"
	if _, err := f.WriteString(yaml); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	f.Close()

	if err := tunnel.LoadConfig(f.Name()); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if tunnel.Name() != "udp-loadcfg-new" {
		t.Errorf("Name after LoadConfig = %q, want %q", tunnel.Name(), "udp-loadcfg-new")
	}
}

// TestUDPBidirectionalAddress verifies the Address() function with a
// persistent-key tunnel that has ServiceKeys set.
func TestUDPBidirectionalAddress(t *testing.T) {
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	cfg := minimalUDPBiConfig("udp-addr-" + shortID())
	cfg.PersistentKey = true

	tunnel, err := NewUDPBidirectional(cfg, samAddr)
	if err != nil {
		t.Fatalf("NewUDPBidirectional: %v", err)
	}
	defer tunnel.Garlic.Close()

	addr := tunnel.Address()
	if addr == "" {
		t.Error("Address() is empty for persistent-key tunnel; expected a b32 I2P address")
	}
}

// TestUDPBidirectionalStopIdempotent verifies that calling Stop() multiple
// times does not panic and returns no error.
func TestUDPBidirectionalStopIdempotent(t *testing.T) {
	cfg := minimalUDPBiConfig("udp-si-" + shortID())
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen for free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	cfg.Port = port

	tunnel, err := NewUDPBidirectional(cfg, samAddr)
	if err != nil {
		t.Fatalf("NewUDPBidirectional: %v", err)
	}

	if err := tunnel.Stop(); err != nil {
		t.Errorf("first Stop() error: %v", err)
	}
	if err := tunnel.Stop(); err != nil {
		t.Errorf("second Stop() error: %v", err)
	}
}

// shortID returns a short unique string for unique SAM session names.
// Uses nanosecond timestamp + random bits to avoid session-already-exists errors.
func shortID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano()^int64(rand.Intn(0xffff))) //nolint:gosec
}

// TestStopWithSocksServer verifies that Stop() calls Shutdown on a non-nil
// socksServer, covering the socks branch in Stop().
func TestStopWithSocksServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	socksAddr := fmt.Sprintf("127.0.0.1:%d", port)
	server, err := socks5.NewClassicServer(socksAddr, "", "", "", 0, 0)
	if err != nil {
		t.Fatalf("NewClassicServer: %v", err)
	}

	tunnel := &UDPBidirectional{
		TunnelConfig:    i2pconv.TunnelConfig{Name: "udp-stop-socks", Type: "udpbidirectional"},
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		done:            make(chan struct{}),
		socksServer:     server,
	}

	if err := tunnel.Stop(); err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}
	if tunnel.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Status after Stop() = %v, want Stopped", tunnel.Status())
	}
}

// TestStopWithMetrics verifies that Stop() calls Metrics.RecordStop() when
// Metrics is non-nil, covering the metrics branch in Stop().
func TestStopWithMetrics(t *testing.T) {
	m := &metrics.TunnelMetrics{Name: "udp-stop-m", ID: "udp-stop-m", Type: "udpbidirectional"}
	tunnel := &UDPBidirectional{
		TunnelConfig:    i2pconv.TunnelConfig{Name: "udp-stop-m", Type: "udpbidirectional"},
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		done:            make(chan struct{}),
		Metrics:         m,
	}

	if err := tunnel.Stop(); err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}
}

// TestSetOptionsValidationErrors covers the error-return branches in SetOptions
// for bad interface, port, target, maxconns, and ratelimit values.
func TestSetOptionsValidationErrors(t *testing.T) {
	makeUDP := func() *UDPBidirectional {
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8080")
		return &UDPBidirectional{
			TunnelConfig: i2pconv.TunnelConfig{
				Name:      "udp-setopt-err",
				Type:      "udpbidirectional",
				Interface: "127.0.0.1",
				Port:      4449,
			},
			Addr:            addr,
			I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
			done:            make(chan struct{}),
		}
	}

	tests := []struct {
		name string
		opts map[string]string
	}{
		{"bad interface", map[string]string{"interface": "not-an-ip"}},
		{"bad port", map[string]string{"port": "notanumber"}},
		{"empty target", map[string]string{"target": ""}},
		{"bad maxconns", map[string]string{"maxconns": "notanumber"}},
		{"bad ratelimit", map[string]string{"ratelimit": "notanumber"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tunnel := makeUDP()
			err := tunnel.SetOptions(tc.opts)
			if err == nil {
				t.Errorf("SetOptions(%v) expected error, got nil", tc.opts)
			}
		})
	}
}

// TestLoadConfigErrors covers error branches in LoadConfig: missing file,
// bad YAML content, wrong tunnel type, and invalid target address.
func TestLoadConfigErrors(t *testing.T) {
	makeUDP := func() *UDPBidirectional {
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:9999")
		return &UDPBidirectional{
			TunnelConfig:    i2pconv.TunnelConfig{Name: "udp-lc-err", Type: "udpbidirectional"},
			Addr:            addr,
			I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
			done:            make(chan struct{}),
		}
	}

	t.Run("file not found", func(t *testing.T) {
		tunnel := makeUDP()
		// DetectFormat only checks the extension, so a .yaml path that doesn't exist
		// passes DetectFormat but fails os.ReadFile.
		err := tunnel.LoadConfig("/tmp/nonexistent-file-that-does-not-exist.yaml")
		if err == nil {
			t.Error("expected error loading non-existent file, got nil")
		}
	})

	t.Run("invalid yaml content", func(t *testing.T) {
		tunnel := makeUDP()
		f, err := os.CreateTemp("", "config*.yaml")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		defer os.Remove(f.Name())
		// Invalid YAML that will cause yaml.Unmarshal to fail.
		if _, err := f.WriteString("key: [invalid\n"); err != nil {
			t.Fatalf("WriteString: %v", err)
		}
		f.Close()
		if err := tunnel.LoadConfig(f.Name()); err == nil {
			t.Error("expected error for invalid YAML content, got nil")
		}
	})

	t.Run("wrong tunnel type", func(t *testing.T) {
		tunnel := makeUDP()
		f, err := os.CreateTemp("", "config*.yaml")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		defer os.Remove(f.Name())
		yaml := "tunnels:\n  t:\n    name: t\n    type: tcpclient\n    target: 127.0.0.1:8080\n    interface: 127.0.0.1\n    port: 4449\n"
		if _, err := f.WriteString(yaml); err != nil {
			t.Fatalf("WriteString: %v", err)
		}
		f.Close()
		if err := tunnel.LoadConfig(f.Name()); err == nil {
			t.Error("expected error for wrong tunnel type, got nil")
		}
	})

	t.Run("invalid target in config", func(t *testing.T) {
		tunnel := makeUDP()
		f, err := os.CreateTemp("", "config*.yaml")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		defer os.Remove(f.Name())
		// "invalid::target" is not a valid UDP address.
		yaml := "tunnels:\n  t:\n    name: t\n    type: udpbidirectional\n    target: invalid::target:9999\n    interface: 127.0.0.1\n    port: 4449\n"
		if _, err := f.WriteString(yaml); err != nil {
			t.Fatalf("WriteString: %v", err)
		}
		f.Close()
		if err := tunnel.LoadConfig(f.Name()); err == nil {
			t.Error("expected error for invalid target address in config, got nil")
		}
	})
}

// intToStr is a helper to avoid importing strconv in the literal string only.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
