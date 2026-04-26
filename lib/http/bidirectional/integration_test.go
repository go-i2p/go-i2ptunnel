package httpbidirectional

// integration_test.go exercises HTTPBidirectional code paths requiring a live SAM bridge or
// simple struct construction:
//   - NewHTTPBidirectional
//   - SetTunnelMetrics / recordError
//   - Address (with and without persistent key)
//   - SetOptions error-return branches
//   - LoadConfig error-return branches
//
// Tests that require a live SAM bridge need 127.0.0.1:7656.

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"testing"
	"time"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	httpclient "github.com/go-i2p/go-i2ptunnel/lib/http/client"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
)

const httpSamAddr = "127.0.0.1:7656"

func minimalHTTPBiConfig(name string) i2pconv.TunnelConfig {
	return i2pconv.TunnelConfig{
		Name:      name,
		Type:      "httpbidirectional",
		Target:    "127.0.0.1:19999",
		Interface: "127.0.0.1",
		Port:      14450,
	}
}

// httpShortID returns a short unique string for SAM session names.
func httpShortID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano()^int64(rand.Intn(0xffff))) //nolint:gosec
}

// TestNewHTTPBidirectional verifies the constructor creates a fully initialized
// tunnel and connects to SAM.
func TestNewHTTPBidirectional(t *testing.T) {
	name := "http-bi-new-" + httpShortID()
	cfg := minimalHTTPBiConfig(name)

	tunnel, err := NewHTTPBidirectional(cfg, httpSamAddr)
	if err != nil {
		t.Fatalf("NewHTTPBidirectional: %v", err)
	}
	defer tunnel.Garlic.Close()

	if tunnel.Name() != name {
		t.Errorf("Name() = %q, want %q", tunnel.Name(), name)
	}
	if tunnel.Type() != "httpbidirectional" {
		t.Errorf("Type() = %q, want %q", tunnel.Type(), "httpbidirectional")
	}
	if tunnel.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Status() = %v, want Stopped", tunnel.Status())
	}
	if tunnel.LimitedConfig.MaxConns != 1000 {
		t.Errorf("MaxConns = %d, want 1000", tunnel.LimitedConfig.MaxConns)
	}
}

// TestNewHTTPBidirectionalBadTarget verifies that an invalid target address
// causes the constructor to return an error.
func TestNewHTTPBidirectionalBadTarget(t *testing.T) {
	cfg := minimalHTTPBiConfig("http-bi-bad-tgt")
	cfg.Target = "not-a-valid-address"
	_, err := NewHTTPBidirectional(cfg, httpSamAddr)
	if err == nil {
		t.Fatal("expected error for invalid target address, got nil")
	}
}

// TestNewHTTPBidirectionalBadSAM verifies that an unreachable SAM address
// causes the constructor to return an error.
func TestNewHTTPBidirectionalBadSAM(t *testing.T) {
	cfg := minimalHTTPBiConfig("http-bi-bad-sam")
	_, err := NewHTTPBidirectional(cfg, "127.0.0.1:19999")
	if err == nil {
		t.Fatal("expected error for bad SAM address, got nil")
	}
}

// TestHTTPBidirectionalSetTunnelMetrics verifies that SetTunnelMetrics injects
// the metrics tracker and that recordError increments the error counter.
func TestHTTPBidirectionalSetTunnelMetrics(t *testing.T) {
	tunnel := &HTTPBidirectional{
		TunnelBase: i2ptunnel.TunnelBase{
		TunnelConfig:    i2pconv.TunnelConfig{Name: "http-bi-metrics", Type: "httpbidirectional"},
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		},
		done:            make(chan struct{}),
	}

	m := &metrics.TunnelMetrics{Name: "http-bi-metrics", ID: "http-bi-metrics", Type: "httpbidirectional"}
	tunnel.SetTunnelMetrics(m)
	if tunnel.Metrics != m {
		t.Error("SetTunnelMetrics did not set Metrics field")
	}

	initial := tunnel.Metrics.Snapshot().ErrorCount
	tunnel.RecordError(nil)
	after := tunnel.Metrics.Snapshot().ErrorCount
	if after <= initial {
		t.Error("recordError did not increment Metrics.ErrorCount")
	}
}

// TestHTTPBidirectionalAddress_Persistent verifies that Address() returns a
// non-empty b32 address for a persistent-key tunnel.
func TestHTTPBidirectionalAddress_Persistent(t *testing.T) {
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	cfg := minimalHTTPBiConfig("http-bi-addr-" + httpShortID())
	cfg.PersistentKey = true

	tunnel, err := NewHTTPBidirectional(cfg, httpSamAddr)
	if err != nil {
		t.Fatalf("NewHTTPBidirectional: %v", err)
	}
	defer tunnel.Garlic.Close()

	addr := tunnel.Address()
	if addr == "" {
		t.Error("Address() is empty for persistent-key tunnel; expected a b32 I2P address")
	}
}

// TestHTTPBidirectionalAddress_NonPersistent verifies Address() returns ""
// when ServiceKeys is nil (non-persistent tunnel).
func TestHTTPBidirectionalAddress_NonPersistent(t *testing.T) {
	tunnel := &HTTPBidirectional{
		done: make(chan struct{}),
		// Garlic is nil — simulates non-persistent tunnel with no keys
	}
	if got := tunnel.Address(); got != "" {
		t.Errorf("Address() = %q for nil Garlic, want empty", got)
	}
}

// TestHTTPBidirectionalSetOptionsErrors covers error-return branches in SetOptions.
func TestHTTPBidirectionalSetOptionsErrors(t *testing.T) {
	makeHTTP := func() *HTTPBidirectional {
		addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
		return &HTTPBidirectional{
			TunnelBase: i2ptunnel.TunnelBase{
			TunnelConfig: i2pconv.TunnelConfig{
				Name:      "http-bi-setopt",
				Type:      "httpbidirectional",
				Interface: "127.0.0.1",
				Port:      4450,
			},
			I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
			},
			Addr:            addr,
			done:            make(chan struct{}),
		}
	}

	tests := []struct {
		name string
		opts map[string]string
	}{
		{"bad interface", map[string]string{"interface": "not-an-ip"}},
		{"bad port", map[string]string{"port": "notanumber"}},
		{"bad maxconns", map[string]string{"maxconns": "notanumber"}},
		{"bad ratelimit", map[string]string{"ratelimit": "notanumber"}},
		{"empty target", map[string]string{"target": ""}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := makeHTTP()
			if err := h.SetOptions(tc.opts); err == nil {
				t.Errorf("SetOptions(%v) expected error, got nil", tc.opts)
			}
		})
	}
}

// TestHTTPBidirectionalSetOptionsI2CP verifies that I2CP options are stored
// in TunnelConfig.I2CP after SetOptions.
func TestHTTPBidirectionalSetOptionsI2CP(t *testing.T) {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	tunnel := &HTTPBidirectional{
		TunnelBase: i2ptunnel.TunnelBase{
		TunnelConfig: i2pconv.TunnelConfig{
			Name:      "http-bi-i2cp",
			Type:      "httpbidirectional",
			Interface: "127.0.0.1",
			Port:      4450,
		},
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		},
		Addr:            addr,
		done:            make(chan struct{}),
	}

	err := tunnel.SetOptions(map[string]string{
		"i2cp.leaseSetEncType":  "4",
		"i2cp.leaseSetAuthType": "0",
	})
	if err != nil {
		t.Fatalf("SetOptions with I2CP opts: %v", err)
	}
	if tunnel.TunnelConfig.I2CP == nil {
		t.Fatal("TunnelConfig.I2CP should be non-nil after setting I2CP options")
	}
}

// TestHTTPBidirectionalLoadConfigErrors covers error-return branches in LoadConfig.
func TestHTTPBidirectionalLoadConfigErrors(t *testing.T) {
	makeHTTP := func() *HTTPBidirectional {
		addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9999")
		return &HTTPBidirectional{
			TunnelBase: i2ptunnel.TunnelBase{
			TunnelConfig:    i2pconv.TunnelConfig{Name: "http-bi-lc-err", Type: "httpbidirectional"},
			I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
			},
			Addr:            addr,
			done:            make(chan struct{}),
		}
	}

	t.Run("bad extension", func(t *testing.T) {
		h := makeHTTP()
		f, err := os.CreateTemp("", "config*.txt")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		f.Close()
		defer os.Remove(f.Name())
		if err := h.LoadConfig(f.Name()); err == nil {
			t.Error("expected error for .txt extension, got nil")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		h := makeHTTP()
		if err := h.LoadConfig("/tmp/nonexistent-http-bi-file.yaml"); err == nil {
			t.Error("expected error loading non-existent file, got nil")
		}
	})

	t.Run("invalid yaml content", func(t *testing.T) {
		h := makeHTTP()
		f, err := os.CreateTemp("", "config*.yaml")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		defer os.Remove(f.Name())
		if _, err := f.WriteString("key: [invalid\n"); err != nil {
			t.Fatalf("WriteString: %v", err)
		}
		f.Close()
		if err := h.LoadConfig(f.Name()); err == nil {
			t.Error("expected error for invalid YAML, got nil")
		}
	})

	t.Run("wrong tunnel type", func(t *testing.T) {
		h := makeHTTP()
		f, err := os.CreateTemp("", "config*.yaml")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		defer os.Remove(f.Name())
		yaml := "tunnels:\n  t:\n    name: t\n    type: tcpclient\n    target: 127.0.0.1:8080\n    interface: 127.0.0.1\n    port: 4450\n"
		if _, err := f.WriteString(yaml); err != nil {
			t.Fatalf("WriteString: %v", err)
		}
		f.Close()
		if err := h.LoadConfig(f.Name()); err == nil {
			t.Error("expected error for wrong tunnel type, got nil")
		}
	})

	t.Run("invalid target in config", func(t *testing.T) {
		h := makeHTTP()
		f, err := os.CreateTemp("", "config*.yaml")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		defer os.Remove(f.Name())
		yaml := "tunnels:\n  t:\n    name: t\n    type: httpbidirectional\n    target: invalid::target:9999\n    interface: 127.0.0.1\n    port: 4450\n"
		if _, err := f.WriteString(yaml); err != nil {
			t.Fatalf("WriteString: %v", err)
		}
		f.Close()
		if err := h.LoadConfig(f.Name()); err == nil {
			t.Error("expected error for invalid target address in config, got nil")
		}
	})

	t.Run("running tunnel", func(t *testing.T) {
		h := makeHTTP()
		h.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning
		if err := h.LoadConfig("/tmp/any.yaml"); err == nil {
			t.Error("expected error loading config while running, got nil")
		}
	})
}

// TestHTTPBidirectionalLoadConfigSuccess verifies LoadConfig succeeds with
// a valid YAML file for the httpbidirectional type.
func TestHTTPBidirectionalLoadConfigSuccess(t *testing.T) {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9999")
	h := &HTTPBidirectional{
		TunnelBase: i2ptunnel.TunnelBase{
		TunnelConfig:    i2pconv.TunnelConfig{Name: "http-bi-lc-ok", Type: "httpbidirectional"},
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		},
		Addr:            addr,
		done:            make(chan struct{}),
	}

	f, err := os.CreateTemp("", "config*.yaml")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(f.Name())
	yaml := "tunnels:\n  newname:\n    name: newname\n    type: httpbidirectional\n    target: 127.0.0.1:19999\n    interface: 127.0.0.1\n    port: 4450\n"
	if _, err := f.WriteString(yaml); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	f.Close()

	if err := h.LoadConfig(f.Name()); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if h.Name() != "newname" {
		t.Errorf("Name after LoadConfig = %q, want %q", h.Name(), "newname")
	}
}

// TestHTTPBidirectionalSetOptionsOutproxy verifies that SetOptions creates a new
// Outproxy when opts["outproxy"] is set and Outproxy is nil (covers nil-init branch).
func TestHTTPBidirectionalSetOptionsOutproxy(t *testing.T) {
	h := &HTTPBidirectional{
		TunnelBase: i2ptunnel.TunnelBase{
		TunnelConfig:    i2pconv.TunnelConfig{Name: "http-bi-outproxy", Type: "httpbidirectional"},
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		},
		done:            make(chan struct{}),
		Outproxy:        nil, // explicitly nil so the nil-init branch is taken
	}

	err := h.SetOptions(map[string]string{"outproxy": "outproxy.i2p"})
	if err != nil {
		t.Fatalf("SetOptions outproxy: %v", err)
	}
	if h.Outproxy == nil {
		t.Fatal("Outproxy should be non-nil after SetOptions with outproxy key")
	}
	if h.Outproxy.Address != "outproxy.i2p" {
		t.Errorf("Outproxy.Address = %q, want %q", h.Outproxy.Address, "outproxy.i2p")
	}
}

// TestHTTPBidirectionalSetOptionsOutproxyEnabled covers the nil-init branch of outproxy.enabled.
func TestHTTPBidirectionalSetOptionsOutproxyEnabled(t *testing.T) {
	h := &HTTPBidirectional{
		TunnelBase: i2ptunnel.TunnelBase{
		TunnelConfig:    i2pconv.TunnelConfig{Name: "http-bi-op-enabled", Type: "httpbidirectional"},
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		},
		done:            make(chan struct{}),
		Outproxy:        nil,
	}

	err := h.SetOptions(map[string]string{"outproxy.enabled": "true"})
	if err != nil {
		t.Fatalf("SetOptions outproxy.enabled: %v", err)
	}
	if h.Outproxy == nil {
		t.Fatal("Outproxy should be non-nil after SetOptions with outproxy.enabled key")
	}
	if !h.Outproxy.Enabled {
		t.Error("Outproxy.Enabled should be true")
	}
}

// TestHTTPBidirectionalOptionsEnabledOutproxy verifies Options() returns
// "outproxy.enabled"="true" when Outproxy.Enabled is true.
func TestHTTPBidirectionalOptionsEnabledOutproxy(t *testing.T) {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9999")
	h := &HTTPBidirectional{
		TunnelBase: i2ptunnel.TunnelBase{
		TunnelConfig:    i2pconv.TunnelConfig{Name: "http-bi-opts-op", Type: "httpbidirectional"},
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		},
		Addr:            addr,
		done:            make(chan struct{}),
		Outproxy: &httpclient.Outproxy{
			Address: "outproxy.i2p",
			Enabled: true,
		},
	}

	opts := h.Options()
	if opts["outproxy.enabled"] != "true" {
		t.Errorf("Options()[outproxy.enabled] = %q, want %q", opts["outproxy.enabled"], "true")
	}
}

// TestHTTPBidirectionalStopWithListenerAndCancel verifies that Stop() correctly
// closes a non-nil listener and calls cancel (covers those two branches).
func TestHTTPBidirectionalStopWithListenerAndCancel(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	h := &HTTPBidirectional{
		TunnelBase: i2ptunnel.TunnelBase{
		TunnelConfig:    i2pconv.TunnelConfig{Name: "http-bi-stop-ln", Type: "httpbidirectional"},
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		},
		done:            make(chan struct{}),
		listener:        ln,
		ctx:             ctx,
		cancel:          cancel,
	}

	if err := h.Stop(); err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}
	if h.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Status after Stop() = %v, want Stopped", h.Status())
	}
}

// TestDialContextBarePHost covers the DialContext branch where addr has no port
// (host == ""), causing the fallback host = addr.
// A clearnet bare hostname is used so the test safely hits dialOutproxy (no Garlic needed).
func TestDialContextBareHost(t *testing.T) {
	h := &HTTPBidirectional{
		done:     make(chan struct{}),
		Outproxy: nil, // clearnet rejected
	}
	// Pass a bare clearnet hostname (no port) — net.SplitHostPort returns host="" so
	// the code sets host = addr (the coverage target), then routes to dialOutproxy
	// because it's not an I2P address — safe without a live Garlic.
	_, err := h.DialContext(context.Background(), "tcp", "example.com")
	if err == nil {
		t.Error("expected error dialing clearnet without outproxy, got nil")
	}
}

// TestResolveJump_NeedsNoJump verifies that resolveJump returns the address
// unchanged when NeedsJump is false (b32.i2p address), covering that branch.
func TestResolveJump_NeedsNoJump(t *testing.T) {
	// Use NewJumpService with an unreachable URL — we only need Jump != nil.
	srv := httpclient.NewJumpService(nil, "http://unreachable.invalid/")

	h := &HTTPBidirectional{
		done: make(chan struct{}),
		Jump: srv,
	}

	// B32 addresses do NOT need jump (NeedsJump returns false → return addr unchanged)
	addr := "example.b32.i2p:80"
	result := h.resolveJump(addr)
	if result != addr {
		t.Errorf("resolveJump(%q) = %q, want unchanged %q", addr, result, addr)
	}
}

// TestResolveJump_LookupFails verifies that when Jump.Lookup returns an error,
// resolveJump logs and returns the original address.
func TestResolveJump_LookupFails(t *testing.T) {
	// Use http.DefaultClient with an unreachable URL so Lookup will fail quickly.
	srv := httpclient.NewJumpService(nil, "http://unreachable.invalid/")

	h := &HTTPBidirectional{
		done: make(chan struct{}),
		Jump: srv,
	}

	// human-readable .i2p hostname without port: NeedsJump=true, Lookup fails → return addr
	addr := "forums.i2p"
	result := h.resolveJump(addr)
	// When lookup fails, resolveJump returns the original address
	if result != addr {
		t.Errorf("resolveJump(%q) = %q, want %q (lookup error → return addr)", addr, result, addr)
	}
}

// TestHTTPBidirectionalStopIdempotentSAM verifies Stop after a SAM-connected
// construction works cleanly.
func TestHTTPBidirectionalStopIdempotentSAM(t *testing.T) {
	cfg := minimalHTTPBiConfig("http-bi-stop-" + httpShortID())
	tunnel, err := NewHTTPBidirectional(cfg, httpSamAddr)
	if err != nil {
		t.Fatalf("NewHTTPBidirectional: %v", err)
	}

	if err := tunnel.Stop(); err != nil {
		t.Errorf("first Stop() error: %v", err)
	}
	if err := tunnel.Stop(); err != nil {
		t.Errorf("second Stop() error: %v", err)
	}
}
