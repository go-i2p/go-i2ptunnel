package shared

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

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
	setOptsErr    error

	opts        map[string]string
	lastSetOpts map[string]string
}

func (m *mockTunnel) Start() error                      { m.started = true; return m.startErr }
func (m *mockTunnel) Stop() error                       { m.stopped = true; return m.stopErr }
func (m *mockTunnel) Name() string                      { return "mock-tunnel" }
func (m *mockTunnel) ID() string                        { return "mock-tunnel" }
func (m *mockTunnel) Type() string                      { return "mock" }
func (m *mockTunnel) Address() string                   { return "" }
func (m *mockTunnel) Target() string                    { return "" }
func (m *mockTunnel) Error() error                      { return nil }
func (m *mockTunnel) Status() i2ptunnel.I2PTunnelStatus { return i2ptunnel.I2PTunnelStatusStopped }

func (m *mockTunnel) Options() map[string]string {
	if m.opts != nil {
		return m.opts
	}
	return map[string]string{}
}
func (m *mockTunnel) SetOptions(opts map[string]string) error {
	m.lastSetOpts = opts
	return m.setOptsErr
}

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

// TestPromptLeaseSetCredentialNoAuth verifies that tunnels with no auth type
// (the common case) skip the prompt and return nil.
func TestPromptLeaseSetCredentialNoAuth(t *testing.T) {
	tunnel := &mockTunnel{
		opts: map[string]string{"i2cp.leaseSetAuthType": "0"},
	}
	var out bytes.Buffer
	err := promptLeaseSetCredential(tunnel, strings.NewReader(""), &out)
	if err != nil {
		t.Fatalf("expected nil for auth type 0, got: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for auth type 0, got: %q", out.String())
	}
	if tunnel.lastSetOpts != nil {
		t.Error("SetOptions should not have been called for auth type 0")
	}
}

// TestPromptLeaseSetCredentialMissingAuthType verifies that a tunnel with no
// i2cp.leaseSetAuthType option also skips the prompt.
func TestPromptLeaseSetCredentialMissingAuthType(t *testing.T) {
	tunnel := &mockTunnel{}
	err := promptLeaseSetCredential(tunnel, strings.NewReader(""), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("expected nil when leaseSetAuthType is absent, got: %v", err)
	}
}

// TestPromptLeaseSetCredentialDH verifies that auth type "1" (DH) prompts for
// the key and passes it to SetOptions.
func TestPromptLeaseSetCredentialDH(t *testing.T) {
	const fakeKey = "abc123fakeBase64Key=="
	tunnel := &mockTunnel{
		opts: map[string]string{"i2cp.leaseSetAuthType": "1"},
	}
	var out bytes.Buffer
	err := promptLeaseSetCredential(tunnel, strings.NewReader(fakeKey+"\n"), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tunnel.lastSetOpts == nil {
		t.Fatal("SetOptions was not called")
	}
	if got := tunnel.lastSetOpts["i2cp.leaseSetPrivKey"]; got != fakeKey {
		t.Errorf("SetOptions got key %q, want %q", got, fakeKey)
	}
	if !strings.Contains(out.String(), "DH") {
		t.Errorf("expected DH in prompt output, got: %q", out.String())
	}
}

// TestPromptLeaseSetCredentialPSK verifies that auth type "2" (PSK) prompts
// with the correct type name and passes the key to SetOptions.
func TestPromptLeaseSetCredentialPSK(t *testing.T) {
	const fakeKey = "pskKey999=="
	tunnel := &mockTunnel{
		opts: map[string]string{"i2cp.leaseSetAuthType": "2"},
	}
	var out bytes.Buffer
	err := promptLeaseSetCredential(tunnel, strings.NewReader(fakeKey+"\n"), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tunnel.lastSetOpts["i2cp.leaseSetPrivKey"]; got != fakeKey {
		t.Errorf("SetOptions got key %q, want %q", got, fakeKey)
	}
	if !strings.Contains(out.String(), "PSK") {
		t.Errorf("expected PSK in prompt output, got: %q", out.String())
	}
}

// TestPromptLeaseSetCredentialEmptyInput verifies that an empty key returns an error.
func TestPromptLeaseSetCredentialEmptyInput(t *testing.T) {
	tunnel := &mockTunnel{
		opts: map[string]string{"i2cp.leaseSetAuthType": "2"},
	}
	err := promptLeaseSetCredential(tunnel, strings.NewReader("\n"), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for empty key input")
	}
}

// TestPromptLeaseSetCredentialSetOptionsError verifies that a SetOptions error
// is propagated to the caller.
func TestPromptLeaseSetCredentialSetOptionsError(t *testing.T) {
	tunnel := &mockTunnel{
		opts:       map[string]string{"i2cp.leaseSetAuthType": "1"},
		setOptsErr: errors.New("connection refused"),
	}
	err := promptLeaseSetCredential(tunnel, strings.NewReader("somekey\n"), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error when SetOptions fails")
	}
}

// --- Thread-safe mock for signal-based tests ---

// signalMockTunnel is a concurrency-safe mock for testing startAndWait,
// which calls methods from multiple goroutines (Start in a goroutine,
// reload/Stop from the signal-handling goroutine).
type signalMockTunnel struct {
	mu            sync.Mutex
	stopped       bool
	started       bool
	configLoaded  bool
	loadConfigErr error
}

func (s *signalMockTunnel) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = true
	return nil
}

func (s *signalMockTunnel) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	return nil
}

func (s *signalMockTunnel) LoadConfig(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configLoaded = true
	return s.loadConfigErr
}

func (s *signalMockTunnel) Name() string    { return "signal-mock" }
func (s *signalMockTunnel) ID() string      { return "signal-mock" }
func (s *signalMockTunnel) Type() string    { return "mock" }
func (s *signalMockTunnel) Address() string { return "" }
func (s *signalMockTunnel) Target() string  { return "" }
func (s *signalMockTunnel) Error() error    { return nil }
func (s *signalMockTunnel) Status() i2ptunnel.I2PTunnelStatus {
	return i2ptunnel.I2PTunnelStatusStopped
}
func (s *signalMockTunnel) Options() map[string]string              { return map[string]string{} }
func (s *signalMockTunnel) SetOptions(opts map[string]string) error { return nil }
func (s *signalMockTunnel) LocalAddress() (string, error)           { return "127.0.0.1:9999", nil }

func (s *signalMockTunnel) isStopped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped
}

func (s *signalMockTunnel) isConfigLoaded() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.configLoaded
}

// TestStartAndWaitSIGHUPReload verifies that SIGHUP triggers a config reload cycle
// (stop → LoadConfig → restart) and that the tunnel continues running afterward.
func TestStartAndWaitSIGHUPReload(t *testing.T) {
	tunnel := &signalMockTunnel{}

	errCh := make(chan error, 1)
	go func() {
		errCh <- startAndWait(tunnel, "test", "/tmp/fake.yaml", "127.0.0.1:7656")
	}()

	// Allow time for signal.Notify registration
	time.Sleep(100 * time.Millisecond)

	// Trigger SIGHUP → reload path
	syscall.Kill(os.Getpid(), syscall.SIGHUP)
	time.Sleep(100 * time.Millisecond)

	if !tunnel.isStopped() {
		t.Error("Expected tunnel to be stopped during SIGHUP reload")
	}
	if !tunnel.isConfigLoaded() {
		t.Error("Expected LoadConfig to be called during reload")
	}

	// Clean shutdown via SIGTERM
	syscall.Kill(os.Getpid(), syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Expected clean shutdown, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for shutdown")
	}
}

// TestStartAndWaitSIGHUPReloadFailure verifies that a failed reload (LoadConfig error
// plus loader.Load error on nonexistent path) prints an error and continues running —
// SIGTERM still produces a clean shutdown afterward.
func TestStartAndWaitSIGHUPReloadFailure(t *testing.T) {
	tunnel := &signalMockTunnel{loadConfigErr: errors.New("config parse error")}

	errCh := make(chan error, 1)
	go func() {
		errCh <- startAndWait(tunnel, "test", "/nonexistent/config.yaml", "127.0.0.1:7656")
	}()

	time.Sleep(100 * time.Millisecond)

	// SIGHUP → reload will fail (LoadConfig errors, loader.Load fails on /nonexistent)
	syscall.Kill(os.Getpid(), syscall.SIGHUP)
	time.Sleep(200 * time.Millisecond)

	// Process should still be running; send SIGTERM for clean shutdown
	syscall.Kill(os.Getpid(), syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Expected clean shutdown after failed reload, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for shutdown after failed reload")
	}
}

// TestStartAndWaitSIGTERM verifies that SIGTERM triggers a graceful shutdown.
func TestStartAndWaitSIGTERM(t *testing.T) {
	tunnel := &signalMockTunnel{}

	errCh := make(chan error, 1)
	go func() {
		errCh <- startAndWait(tunnel, "test", "/tmp/fake.yaml", "127.0.0.1:7656")
	}()

	time.Sleep(100 * time.Millisecond)
	syscall.Kill(os.Getpid(), syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Expected clean shutdown, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for shutdown")
	}

	if !tunnel.isStopped() {
		t.Error("Expected tunnel to be stopped on SIGTERM")
	}
}

// TestStartAndWaitStartError verifies that a Start() error is propagated.
func TestStartAndWaitStartError(t *testing.T) {
	tunnel := &mockTunnel{startErr: errors.New("SAM connect failed")}

	err := startAndWait(tunnel, "test", "/tmp/fake.yaml", "127.0.0.1:7656")
	if err == nil {
		t.Fatal("Expected error from Start()")
	}
	if !strings.Contains(err.Error(), "SAM connect failed") {
		t.Errorf("Expected 'SAM connect failed' in error, got: %v", err)
	}
}
