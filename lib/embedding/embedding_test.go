package embedding

import (
	"errors"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// --- shared mock ---

// mockTunnel implements i2ptunnel.I2PTunnel for unit testing the embedding API.
type mockTunnel struct {
	mu sync.Mutex

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

func (m *mockTunnel) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = true
	return m.startErr
}
func (m *mockTunnel) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
	return m.stopErr
}
func (m *mockTunnel) LoadConfig(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configLoaded = true
	return m.loadConfigErr
}
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
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastSetOpts = opts
	return m.setOptsErr
}
func (m *mockTunnel) LocalAddress() (string, error) {
	return m.localAddrVal, m.localAddrErr
}

func (m *mockTunnel) isStopped() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopped
}
func (m *mockTunnel) isConfigLoaded() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.configLoaded
}

// --- Wrap / option tests ---

// TestWrapInjectsLeaseSetKey verifies that WithLeaseSetKey calls SetOptions
// on the underlying tunnel during construction.
func TestWrapInjectsLeaseSetKey(t *testing.T) {
	const key = "fakeBase64Key=="
	mock := &mockTunnel{}
	tun, err := Wrap(mock, WithLeaseSetKey(key))
	if err != nil {
		t.Fatalf("Wrap error: %v", err)
	}
	if tun == nil {
		t.Fatal("expected non-nil Tunnel")
	}
	mock.mu.Lock()
	got := mock.lastSetOpts["i2cp.leaseSetPrivKey"]
	mock.mu.Unlock()
	if got != key {
		t.Errorf("leaseSetPrivKey = %q, want %q", got, key)
	}
}

// TestWrapLeaseSetKeySetOptionsError verifies that a SetOptions failure during
// construction surfaces as an error from Wrap.
func TestWrapLeaseSetKeySetOptionsError(t *testing.T) {
	mock := &mockTunnel{setOptsErr: errors.New("inject error")}
	_, err := Wrap(mock, WithLeaseSetKey("somekey"))
	if err == nil {
		t.Fatal("expected error when SetOptions fails")
	}
	if !strings.Contains(err.Error(), "inject error") {
		t.Errorf("error %q should mention the underlying cause", err)
	}
}

// TestWrapDefaultSAMAddr verifies that the default SAM address is applied.
func TestWrapDefaultSAMAddr(t *testing.T) {
	mock := &mockTunnel{}
	tun, err := Wrap(mock)
	if err != nil {
		t.Fatalf("Wrap error: %v", err)
	}
	if tun.samAddr != "127.0.0.1:7656" {
		t.Errorf("samAddr = %q, want 127.0.0.1:7656", tun.samAddr)
	}
}

// TestWithSAMAddr verifies that WithSAMAddr overrides the default.
func TestWithSAMAddr(t *testing.T) {
	mock := &mockTunnel{}
	tun, err := Wrap(mock, WithSAMAddr("10.0.0.1:7656"))
	if err != nil {
		t.Fatalf("Wrap error: %v", err)
	}
	if tun.samAddr != "10.0.0.1:7656" {
		t.Errorf("samAddr = %q, want 10.0.0.1:7656", tun.samAddr)
	}
}

// TestTunnelAccessor verifies that Tunnel() returns the underlying I2PTunnel.
func TestTunnelAccessor(t *testing.T) {
	mock := &mockTunnel{}
	tun, _ := Wrap(mock)
	if tun.Tunnel() != mock {
		t.Error("Tunnel() did not return the wrapped mock")
	}
}

// TestNameDelegates verifies that Name() delegates to the underlying tunnel.
func TestNameDelegates(t *testing.T) {
	mock := &mockTunnel{}
	tun, _ := Wrap(mock)
	if tun.Name() != "mock-tunnel" {
		t.Errorf("Name() = %q, want mock-tunnel", tun.Name())
	}
}

// --- FromConfigFile error path ---

// TestFromConfigFileInvalidPath verifies that a nonexistent config path
// returns a descriptive error from FromConfigFile.
func TestFromConfigFileInvalidPath(t *testing.T) {
	_, err := FromConfigFile("/nonexistent/path/tunnel.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent config file")
	}
	if !strings.Contains(err.Error(), "embedding:") {
		t.Errorf("error %q should carry embedding: prefix", err)
	}
}

// --- Reload tests ---

// TestReloadStopError verifies that a Stop failure aborts Reload with an error.
func TestReloadStopError(t *testing.T) {
	mock := &mockTunnel{stopErr: os.ErrClosed}
	tun, _ := Wrap(mock)
	tun.configPath = "/fake/path.yaml"
	err := tun.Reload()
	if err == nil {
		t.Error("expected error when Stop fails during Reload")
	}
}

// TestReloadSuccessInPlace verifies the happy path: Stop → LoadConfig → tunnel unchanged.
func TestReloadSuccessInPlace(t *testing.T) {
	mock := &mockTunnel{}
	tun, _ := Wrap(mock)
	tun.configPath = "/fake/path.yaml"

	if err := tun.Reload(); err != nil {
		t.Fatalf("Reload error: %v", err)
	}
	if !mock.isStopped() {
		t.Error("expected Stop() to have been called")
	}
	if !mock.isConfigLoaded() {
		t.Error("expected LoadConfig() to have been called")
	}
	// Tunnel instance unchanged for in-place reload
	if tun.tunnel != mock {
		t.Error("expected same tunnel instance after in-place reload")
	}
}

// TestReloadNoConfigPath verifies that Reload with no config path stops the
// tunnel but does not error (Wrap case — no file to reload from).
func TestReloadNoConfigPath(t *testing.T) {
	mock := &mockTunnel{}
	tun, _ := Wrap(mock)
	// configPath is empty by design

	if err := tun.Reload(); err != nil {
		t.Fatalf("Reload with empty configPath should not error: %v", err)
	}
	if !mock.isStopped() {
		t.Error("expected Stop() to have been called")
	}
}

// TestReloadLoadConfigFallback verifies that when LoadConfig fails, Reload
// attempts to load a fresh tunnel from the config file.
// Both loader.Load and LoadConfig will fail (no real SAM / nonexistent path),
// so we expect an error, but Stop() must have been called.
func TestReloadLoadConfigFallback(t *testing.T) {
	mock := &mockTunnel{loadConfigErr: os.ErrInvalid}
	tun, _ := Wrap(mock)
	tun.configPath = "/nonexistent/config.yaml"

	err := tun.Reload()
	if err == nil {
		t.Error("expected error when LoadConfig fails and loader.Load also fails")
	}
	if !mock.isStopped() {
		t.Error("expected Stop() to have been called before the reload attempt")
	}
}

// --- Start / Stop delegation ---

// TestStartStop verifies that Start and Stop delegate to the underlying tunnel.
func TestStartStop(t *testing.T) {
	mock := &mockTunnel{}
	tun, _ := Wrap(mock)

	if err := tun.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if !mock.started {
		t.Error("expected Start() to have been called on mock")
	}

	if err := tun.Stop(); err != nil {
		t.Fatalf("Stop error: %v", err)
	}
	if !mock.isStopped() {
		t.Error("expected Stop() to have been called on mock")
	}
}

// TestStartError verifies that Start propagates errors from the underlying tunnel.
func TestStartError(t *testing.T) {
	mock := &mockTunnel{startErr: errors.New("SAM connect failed")}
	tun, _ := Wrap(mock)
	err := tun.Start()
	if err == nil {
		t.Fatal("expected error from Start()")
	}
	if !strings.Contains(err.Error(), "SAM connect failed") {
		t.Errorf("error %q should mention SAM connect failed", err)
	}
}

// --- Run signal tests ---

// TestRunSIGTERM verifies that Run returns cleanly on SIGTERM.
func TestRunSIGTERM(t *testing.T) {
	mock := &mockTunnel{}
	tun, _ := Wrap(mock)

	errCh := make(chan error, 1)
	go func() { errCh <- tun.Run() }()

	time.Sleep(100 * time.Millisecond)
	syscall.Kill(os.Getpid(), syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected clean shutdown, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for shutdown")
	}
	if !mock.isStopped() {
		t.Error("expected tunnel to be stopped on SIGTERM")
	}
}

// TestRunSIGINT verifies that Run returns cleanly on SIGINT.
func TestRunSIGINT(t *testing.T) {
	mock := &mockTunnel{}
	tun, _ := Wrap(mock)

	errCh := make(chan error, 1)
	go func() { errCh <- tun.Run() }()

	time.Sleep(100 * time.Millisecond)
	syscall.Kill(os.Getpid(), syscall.SIGINT)

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected clean shutdown, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for shutdown")
	}
}

// TestRunSIGHUPReload verifies that SIGHUP triggers Stop → LoadConfig → restart.
func TestRunSIGHUPReload(t *testing.T) {
	mock := &mockTunnel{}
	tun, _ := Wrap(mock)
	tun.configPath = "/fake/path.yaml"

	errCh := make(chan error, 1)
	go func() { errCh <- tun.Run() }()

	time.Sleep(100 * time.Millisecond)
	syscall.Kill(os.Getpid(), syscall.SIGHUP)
	time.Sleep(150 * time.Millisecond)

	if !mock.isStopped() {
		t.Error("expected Stop() during SIGHUP reload")
	}
	if !mock.isConfigLoaded() {
		t.Error("expected LoadConfig() during SIGHUP reload")
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected clean shutdown after reload, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for shutdown after reload")
	}
}

// TestRunSIGHUPReloadFailureContinues verifies that a failed reload does not
// kill Run — SIGTERM still produces a clean shutdown.
func TestRunSIGHUPReloadFailureContinues(t *testing.T) {
	mock := &mockTunnel{loadConfigErr: errors.New("parse error")}
	tun, _ := Wrap(mock)
	tun.configPath = "/nonexistent/config.yaml"

	errCh := make(chan error, 1)
	go func() { errCh <- tun.Run() }()

	time.Sleep(100 * time.Millisecond)
	syscall.Kill(os.Getpid(), syscall.SIGHUP)
	time.Sleep(200 * time.Millisecond)

	// Run should still be alive; clean shutdown via SIGTERM.
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected clean shutdown after failed reload, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for shutdown after failed reload")
	}
}

// TestRunStartError verifies that a Start() error is propagated from Run.
func TestRunStartError(t *testing.T) {
	mock := &mockTunnel{startErr: errors.New("SAM connect failed")}
	tun, _ := Wrap(mock)

	errCh := make(chan error, 1)
	go func() { errCh <- tun.Run() }()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error from Run when Start fails")
		}
		if !strings.Contains(err.Error(), "SAM connect failed") {
			t.Errorf("error %q should mention SAM connect failed", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Run to return error")
	}
}
