package tcpserver

import (
	"fmt"
	"net"
	"os"
	"testing"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/onramp"
)

func newTestServer(t *testing.T) *TCPServer {
	t.Helper()
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9090")
	return &TCPServer{
		TunnelConfig: i2pconv.TunnelConfig{
			Name:      "test-server",
			Type:      "tcpserver",
			Interface: "127.0.0.1",
			Port:      4449,
		},
		Addr:            addr,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		LimitedConfig: limitedlistener.LimitedConfig{
			MaxConns:  100,
			RateLimit: 10,
		},
		done: make(chan struct{}),
	}
}

func TestSetTunnelMetricsTCPServer(t *testing.T) {
	ts := newTestServer(t)
	m := &metrics.TunnelMetrics{}
	ts.SetTunnelMetrics(m)
	if ts.Metrics != m {
		t.Error("SetTunnelMetrics did not set the Metrics field")
	}
}

func TestRecordErrorTCPServer(t *testing.T) {
	ts := newTestServer(t)
	// Without metrics — must not panic
	ts.recordError(fmt.Errorf("test error"))
	if ts.Error() == nil {
		t.Error("Error() should be non-nil after recording an error")
	}

	// With metrics — must call RecordError without panic
	ts.SetTunnelMetrics(&metrics.TunnelMetrics{})
	ts.recordError(fmt.Errorf("second error"))
	if ts.Error() == nil {
		t.Error("Error() should remain non-nil after second recorded error")
	}
}

func TestAddressNilGarlicTCPServer(t *testing.T) {
	ts := newTestServer(t)
	// nil Garlic → empty string, no panic
	if got := ts.Address(); got != "" {
		t.Errorf("Address() with nil Garlic = %q, want \"\"", got)
	}
}

func TestAddressNilServiceKeysTCPServer(t *testing.T) {
	ts := newTestServer(t)
	// non-nil Garlic but nil ServiceKeys → empty string, no panic
	ts.Garlic = &onramp.Garlic{}
	if got := ts.Address(); got != "" {
		t.Errorf("Address() with nil ServiceKeys = %q, want \"\"", got)
	}
}

func TestErrorNilInitialTCPServer(t *testing.T) {
	ts := newTestServer(t)
	if ts.Error() != nil {
		t.Error("Error() should be nil on a fresh tunnel")
	}
}

func TestLocalAddressTCPServer(t *testing.T) {
	ts := newTestServer(t)
	got, err := ts.LocalAddress()
	if err != nil {
		t.Fatalf("LocalAddress() returned error: %v", err)
	}
	want := "127.0.0.1:4449"
	if got != want {
		t.Errorf("LocalAddress() = %q, want %q", got, want)
	}
}

func TestNameTCPServer(t *testing.T) {
	ts := newTestServer(t)
	if got := ts.Name(); got != "test-server" {
		t.Errorf("Name() = %q, want %q", got, "test-server")
	}
}

func TestTypeTCPServer(t *testing.T) {
	ts := newTestServer(t)
	if got := ts.Type(); got != "tcpserver" {
		t.Errorf("Type() = %q, want %q", got, "tcpserver")
	}
}

func TestIDTCPServer(t *testing.T) {
	ts := newTestServer(t)
	if got := ts.ID(); got == "" {
		t.Error("ID() should not be empty")
	}
}

func TestSetOptionsValidationTCPServer(t *testing.T) {
	tests := []struct {
		name    string
		opts    map[string]string
		wantErr bool
	}{
		{"empty name rejected", map[string]string{"name": ""}, true},
		{"valid name accepted", map[string]string{"name": "new-name"}, false},
		{"invalid interface rejected", map[string]string{"interface": "not_an_ip"}, true},
		{"valid interface accepted", map[string]string{"interface": "127.0.0.1"}, false},
		{"empty interface accepted", map[string]string{"interface": ""}, false},
		{"non-numeric port rejected", map[string]string{"port": "abc"}, true},
		{"out-of-range port rejected", map[string]string{"port": "99999"}, true},
		{"valid port accepted", map[string]string{"port": "8080"}, false},
		{"non-integer maxconns rejected", map[string]string{"maxconns": "abc"}, true},
		{"negative maxconns rejected", map[string]string{"maxconns": "-1"}, true},
		{"zero maxconns accepted", map[string]string{"maxconns": "0"}, false},
		{"positive maxconns accepted", map[string]string{"maxconns": "50"}, false},
		{"non-float ratelimit rejected", map[string]string{"ratelimit": "abc"}, true},
		{"negative ratelimit rejected", map[string]string{"ratelimit": "-1"}, true},
		{"zero ratelimit accepted", map[string]string{"ratelimit": "0"}, false},
		{"positive ratelimit accepted", map[string]string{"ratelimit": "15.5"}, false},
		{"i2cp option on nil I2CP map", map[string]string{"i2cp.leaseSetType": "4"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t)
			err := ts.SetOptions(tt.opts)
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestSetOptionsI2CPWithExistingMap verifies that SetOptions merges I2CP options
// into a pre-existing map rather than replacing it.
func TestSetOptionsI2CPWithExistingMap(t *testing.T) {
	ts := newTestServer(t)
	ts.TunnelConfig.I2CP = map[string]interface{}{"existing": "value"}
	if err := ts.SetOptions(map[string]string{"i2cp.leaseSetType": "4"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts.TunnelConfig.I2CP["existing"] != "value" {
		t.Error("existing I2CP key was overwritten instead of preserved")
	}
}

// TestStopWithListener verifies Stop() closes the stored listener.
func TestStopWithListener(t *testing.T) {
	// Use a real localhost listener to verify Close() is called.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	ts := newTestServer(t)
	ts.listener = l

	if err := ts.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	// After Stop, the listener should be closed; Accept() should fail.
	_, acceptErr := l.Accept()
	if acceptErr == nil {
		t.Error("expected Accept() to fail after Stop() closed the listener")
	}
}

// TestStopRecordsMetrics verifies Stop() calls Metrics.RecordStop() without panic.
func TestStopRecordsMetrics(t *testing.T) {
	ts := newTestServer(t)
	m := &metrics.TunnelMetrics{}
	ts.SetTunnelMetrics(m)
	if err := ts.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if ts.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Status after Stop = %v, want Stopped", ts.Status())
	}
}

func writeTCPServerYAML(t *testing.T, tunnelName, tunnelType, target, iface string, port int) string {
	t.Helper()
	content := fmt.Sprintf("tunnels:\n  %s:\n    name: %q\n    type: %q\n    interface: %q\n",
		tunnelName, tunnelName, tunnelType, iface)
	if target != "" {
		content += fmt.Sprintf("    target: %q\n", target)
	}
	if port > 0 {
		content += fmt.Sprintf("    port: %d\n", port)
	}
	f, err := os.CreateTemp(t.TempDir(), "*.yaml")
	if err != nil {
		t.Fatalf("os.CreateTemp: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("f.WriteString: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestLoadConfigTCPServer(t *testing.T) {
	t.Run("running tunnel rejects reload", func(t *testing.T) {
		ts := newTestServer(t)
		ts.setStatus(i2ptunnel.I2PTunnelStatusRunning)
		if err := ts.LoadConfig("/any"); err == nil {
			t.Error("expected error when tunnel is running, got nil")
		}
	})

	t.Run("starting tunnel rejects reload", func(t *testing.T) {
		ts := newTestServer(t)
		ts.setStatus(i2ptunnel.I2PTunnelStatusStarting)
		if err := ts.LoadConfig("/any"); err == nil {
			t.Error("expected error when tunnel is starting, got nil")
		}
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		ts := newTestServer(t)
		if err := ts.LoadConfig("/nonexistent/config.yaml"); err == nil {
			t.Error("expected error for nonexistent file, got nil")
		}
	})

	t.Run("wrong tunnel type returns error", func(t *testing.T) {
		ts := newTestServer(t)
		path := writeTCPServerYAML(t, "wrong", "httpclient", "", "127.0.0.1", 4444)
		if err := ts.LoadConfig(path); err == nil {
			t.Error("expected error for wrong tunnel type, got nil")
		}
	})

	t.Run("invalid YAML returns error", func(t *testing.T) {
		ts := newTestServer(t)
		f, err := os.CreateTemp(t.TempDir(), "*.yaml")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		f.WriteString(":::not valid yaml:::")
		f.Close()
		if err := ts.LoadConfig(f.Name()); err == nil {
			t.Error("expected error for invalid YAML, got nil")
		}
	})

	t.Run("invalid target address returns error", func(t *testing.T) {
		ts := newTestServer(t)
		path := writeTCPServerYAML(t, "bad-target", "tcpserver", "notahost", "127.0.0.1", 0)
		if err := ts.LoadConfig(path); err == nil {
			t.Error("expected error for invalid target address, got nil")
		}
	})

	t.Run("valid config loads successfully", func(t *testing.T) {
		ts := newTestServer(t)
		path := writeTCPServerYAML(t, "reloaded", "tcpserver", "127.0.0.1:9999", "127.0.0.1", 0)
		if err := ts.LoadConfig(path); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ts.TunnelConfig.Name != "reloaded" {
			t.Errorf("Name after LoadConfig = %q, want %q", ts.TunnelConfig.Name, "reloaded")
		}
		if ts.Target() != "127.0.0.1:9999" {
			t.Errorf("Target after LoadConfig = %q, want %q", ts.Target(), "127.0.0.1:9999")
		}
	})
}
