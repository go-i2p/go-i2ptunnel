package shared

import (
	"os"
	"testing"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// mockTunnel implements i2ptunnel.I2PTunnel for unit testing the runner logic.
type mockTunnel struct {
	stopped      bool
	started      bool
	configLoaded bool

	stopErr       error
	startErr      error
	loadConfigErr error
	localAddrVal  string
	localAddrErr  error
}

func (m *mockTunnel) Start() error                       { m.started = true; return m.startErr }
func (m *mockTunnel) Stop() error                        { m.stopped = true; return m.stopErr }
func (m *mockTunnel) Name() string                       { return "mock-tunnel" }
func (m *mockTunnel) ID() string                         { return "mock-tunnel" }
func (m *mockTunnel) Type() string                       { return "mock" }
func (m *mockTunnel) Address() string                    { return "" }
func (m *mockTunnel) Target() string                     { return "" }
func (m *mockTunnel) Options() map[string]string         { return nil }
func (m *mockTunnel) SetOptions(map[string]string) error { return nil }
func (m *mockTunnel) Error() error                       { return nil }
func (m *mockTunnel) Status() i2ptunnel.I2PTunnelStatus  { return i2ptunnel.I2PTunnelStatusStopped }

func (m *mockTunnel) LoadConfig(path string) error {
	m.configLoaded = true
	return m.loadConfigErr
}

func (m *mockTunnel) LocalAddress() (string, error) {
	return m.localAddrVal, m.localAddrErr
}

// TestLocalAddrWithError tests localAddr when the tunnel returns an error.
// Why: localAddr is used in display messages and must not panic on error.
func TestLocalAddrWithError(t *testing.T) {
	tunnel := &mockTunnel{localAddrErr: os.ErrNotExist}
	addr := localAddr(tunnel)
	if addr != "(unknown)" {
		t.Errorf("Expected '(unknown)', got '%s'", addr)
	}
}

// TestLocalAddrSuccess tests localAddr with a working tunnel.
// Why: Verifies the happy path returns the correct address.
func TestLocalAddrSuccess(t *testing.T) {
	tunnel := &mockTunnel{localAddrVal: "127.0.0.1:8080"}
	addr := localAddr(tunnel)
	if addr != "127.0.0.1:8080" {
		t.Errorf("Expected '127.0.0.1:8080', got '%s'", addr)
	}
}

// TestReloadStopError tests reload when Stop() fails.
// Why: Reload must handle stop failures gracefully and report them.
func TestReloadStopError(t *testing.T) {
	tunnel := &mockTunnel{stopErr: os.ErrClosed}
	_, err := reload(tunnel, "test", "/nonexistent", "127.0.0.1:7656")
	if err == nil {
		t.Error("Expected error when stop fails during reload")
	}
}

// TestReloadLoadConfigFallback tests reload when LoadConfig fails.
// Why: If in-place reload fails, the runner should attempt to create a fresh tunnel.
// Design: Both LoadConfig and loader.Load fail (no real SAM), so we expect an error.
func TestReloadLoadConfigFallback(t *testing.T) {
	tunnel := &mockTunnel{loadConfigErr: os.ErrInvalid}
	_, err := reload(tunnel, "test", "/nonexistent/config.yaml", "127.0.0.1:7656")
	if err == nil {
		t.Error("Expected error when both LoadConfig and Load fail")
	}
}

// TestReloadSuccess tests reload when LoadConfig succeeds.
// Why: Verifies the happy path of in-place config reload.
func TestReloadSuccess(t *testing.T) {
	tunnel := &mockTunnel{}
	reloaded, err := reload(tunnel, "test", "/nonexistent", "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if reloaded != tunnel {
		t.Error("Expected same tunnel instance after in-place reload")
	}
	if !tunnel.stopped {
		t.Error("Expected Stop() to have been called")
	}
	if !tunnel.configLoaded {
		t.Error("Expected LoadConfig() to have been called")
	}
}
