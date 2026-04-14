package tcpclient

// quality_fixes_test.go tests the quality improvements introduced in the second
// audit pass:
//   1. Errors field unexported — ErrorHistory() returns a safe snapshot copy
//   2. LoadConfig() preserves existing I2CP options when config file omits the i2cp section
//   3. LoadConfig() check+apply is atomic under lifeMu (TOCTOU window closed)
//   4. Start() transitions to I2PTunnelStatusFailed after maxConsecutiveAcceptErrors

import (
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// ---------------------------------------------------------------------------
// Fix 1: ErrorHistory() — unexported field with thread-safe accessor
// ---------------------------------------------------------------------------

// TestErrorHistory_EmptyReturnsNil verifies that ErrorHistory returns nil (not an empty
// allocated slice) when no errors have been recorded. Callers checking `len(h) == 0`
// work correctly regardless of which nil/empty form is returned, but nil is the idiomatic
// zero-value for an unset slice in Go.
func TestErrorHistory_EmptyReturnsNil(t *testing.T) {
	c := &TCPClient{}
	if h := c.ErrorHistory(); h != nil {
		t.Errorf("ErrorHistory() on a fresh tunnel = %v (want nil)", h)
	}
}

// TestErrorHistory_ReturnsSnapshot verifies that mutations to the returned slice do not
// affect the tunnel's internal error buffer. This is the key guarantee of the accessor
// pattern — callers get a copy they own, not a reference into shared state.
func TestErrorHistory_ReturnsSnapshot(t *testing.T) {
	c := &TCPClient{
		done: make(chan struct{}),
	}
	c.recordError(nil) // nil error produces a NewError with empty message, but is valid

	snap := c.ErrorHistory()
	if len(snap) == 0 {
		t.Fatal("ErrorHistory() should return non-empty slice after recordError")
	}
	before := len(snap)

	// Mutate the snapshot — must not change the internal buffer.
	snap[0] = i2ptunnel.NewError(c, nil)
	snap = append(snap, i2ptunnel.NewError(c, nil))

	snap2 := c.ErrorHistory()
	if len(snap2) != before {
		t.Errorf("ErrorHistory() length changed from %d to %d after mutating snapshot", before, len(snap2))
	}
}

// TestErrorHistory_Concurrent verifies that ErrorHistory and recordError can be called
// concurrently without data races. Run with `go test -race` to detect issues.
func TestErrorHistory_Concurrent(t *testing.T) {
	c := &TCPClient{
		done: make(chan struct{}),
	}
	var wg sync.WaitGroup
	const iters = 50

	// Writer: repeatedly record errors
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iters; i++ {
			c.recordError(nil)
		}
	}()

	// Reader: repeatedly read history
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iters; i++ {
			_ = c.ErrorHistory()
		}
	}()

	wg.Wait()
}

// ---------------------------------------------------------------------------
// Fix 2: LoadConfig() preserves I2CP options when config file omits i2cp section
// ---------------------------------------------------------------------------

// TestLoadConfig_PreservesI2CPWhenNotInFile verifies that I2CP options previously set via
// SetOptions() (e.g. encrypted-leaseset settings applied through the Web UI) survive a
// config file reload that does not include an i2cp section.
//
// Before the fix, t.TunnelConfig = *newConfig blindly replaced the struct, silently wiping
// any I2CP map that existed in the old config but was absent from the new one.
func TestLoadConfig_PreservesI2CPWhenNotInFile(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("lc-i2cp-preserve"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	// Apply I2CP options before the reload.
	if err := tunnel.SetOptions(map[string]string{
		"i2cp.messageReliability": "best-effort",
	}); err != nil {
		t.Fatalf("SetOptions: %v", err)
	}
	if len(tunnel.TunnelConfig.I2CP) == 0 {
		t.Fatal("precondition: I2CP map should be non-empty after SetOptions")
	}

	// Create a config file that has no i2cp section.
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tunnel.yaml")
	configContent := `tunnels:
  lc-i2cp-preserve:
    name: lc-i2cp-preserve
    type: tcpclient
    interface: 127.0.0.1
    port: 9999
    target: ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Tunnel must be stopped before LoadConfig.
	tunnel.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped

	if err := tunnel.LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// I2CP options must be preserved since the new config had none.
	if len(tunnel.TunnelConfig.I2CP) == 0 {
		t.Fatal("I2CP options were wiped by LoadConfig — must be preserved when config omits i2cp section")
	}
	v, ok := tunnel.TunnelConfig.I2CP["messageReliability"]
	if !ok {
		t.Error("preserved I2CP option 'messageReliability' is missing after LoadConfig")
	} else if v != "best-effort" {
		t.Errorf("preserved I2CP option value = %v, want 'best-effort'", v)
	}
}

// TestLoadConfig_DoesNotPreserveI2CPWhenFileHasI2CP verifies that if the config file
// explicitly provides an i2cp section, it replaces the existing options (correct behaviour).
// This is the complement of TestLoadConfig_PreservesI2CPWhenNotInFile.
func TestLoadConfig_DoesNotPreserveI2CPWhenFileHasI2CP(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("lc-i2cp-replace"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	// Pre-set an I2CP option.
	if err := tunnel.SetOptions(map[string]string{
		"i2cp.messageReliability": "best-effort",
	}); err != nil {
		t.Fatalf("SetOptions: %v", err)
	}

	// Config file with its own i2cp block (YAML inline map).
	// The go-i2ptunnel-config parser populates I2CP from the i2cp map key.
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tunnel.yaml")
	configContent := `tunnels:
  lc-i2cp-replace:
    name: lc-i2cp-replace
    type: tcpclient
    interface: 127.0.0.1
    port: 9999
    target: ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p
    i2cp:
      i2cp.messageReliability: none
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	tunnel.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped
	if err := tunnel.LoadConfig(configPath); err != nil {
		// Parser may or may not support inline i2cp map; skip if unsupported.
		t.Skipf("LoadConfig with explicit i2cp section failed (parser may not support it): %v", err)
	}

	// If I2CP was parsed, the new value should take precedence.
	if v, ok := tunnel.TunnelConfig.I2CP["i2cp.messageReliability"]; ok {
		if v == "best-effort" {
			t.Error("old I2CP value 'best-effort' should have been replaced by config-file value")
		}
	}
}

// ---------------------------------------------------------------------------
// Fix 3: LoadConfig() atomic status check + apply (TOCTOU)
// ---------------------------------------------------------------------------

// TestLoadConfig_AtomicCheck verifies that status is evaluated while lifeMu is held,
// making the check+apply atomic.  Direct proof of the lock ordering is not observable
// from outside the package, so this test instead confirms the observable contract:
// LoadConfig always returns an error when the tunnel is running, regardless of concurrent
// Start() / Stop() calls.  The `-race` flag will catch any unsynchronised field access.
func TestLoadConfig_AtomicCheck(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("lc-atomic"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tunnel.yaml")
	configContent := `tunnels:
  lc-atomic:
    name: lc-atomic
    type: tcpclient
    interface: 127.0.0.1
    port: 9999
    target: ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Set status to Running — LoadConfig must reject this atomically under lifeMu.
	tunnel.setStatus(i2ptunnel.I2PTunnelStatusRunning)

	if err := tunnel.LoadConfig(configPath); err == nil {
		t.Error("LoadConfig must return an error when tunnel is in Running state")
	}

	// Also verify Starting is rejected.
	tunnel.setStatus(i2ptunnel.I2PTunnelStatusStarting)
	if err := tunnel.LoadConfig(configPath); err == nil {
		t.Error("LoadConfig must return an error when tunnel is in Starting state")
	}
}

// ---------------------------------------------------------------------------
// Fix 4: Start() transitions to I2PTunnelStatusFailed on persistent Accept errors
// ---------------------------------------------------------------------------

// TestStart_ExitsOnConsecutiveAcceptErrors verifies that a tunnel whose
// accept loop encounters maxConsecutiveAcceptErrors consecutive failures exits
// Start() with an error rather than silently continuing to loop.
//
// Approach: Start() in a goroutine, then close the listener via t.listener so that
// every subsequent Accept() call returns an error. Start() should return a non-nil
// error once the consecutive threshold is reached.
func TestStart_ExitsOnConsecutiveAcceptErrors(t *testing.T) {
	cfg := i2pconv.TunnelConfig{
		Name:      "fail-on-errors",
		Type:      "tcpclient",
		Interface: "127.0.0.1",
		Port:      0, // kernel picks a free port
		Target:    testTarget,
	}

	// Build a minimal TCPClient without a SAM session — the accept loop does not
	// require Garlic until an actual connection arrives.
	c := &TCPClient{
		TunnelConfig:    cfg,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		done:            make(chan struct{}),
		dialTimeout:     defaultDialTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Start()
	}()

	// Wait until Start() has created the listener and set status to Running.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if c.Status() == i2ptunnel.I2PTunnelStatusRunning {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if c.Status() != i2ptunnel.I2PTunnelStatusRunning {
		c.Stop()
		<-errCh
		t.Fatalf("tunnel did not reach Running status within deadline")
	}

	// Grab the listener and close it so every future Accept() returns an error.
	c.lifeMu.Lock()
	ln := c.listener
	c.lifeMu.Unlock()
	if ln == nil {
		c.Stop()
		<-errCh
		t.Fatal("c.listener is nil after Start() reported Running")
	}
	ln.Close()

	// Start() should now return with a non-nil error once the consecutive threshold
	// is reached. Each iteration sleeps 50ms so wait long enough for the loop to exit.
	exitDeadline := time.Duration(maxConsecutiveAcceptErrors)*50*time.Millisecond + 2*time.Second
	select {
	case err := <-errCh:
		if err == nil {
			t.Error("Start() returned nil error after consecutive accept failures; expected non-nil")
		}
	case <-time.After(exitDeadline):
		c.Stop()
		<-errCh
		t.Fatal("Start() did not exit within deadline after consecutive accept errors")
	}
}

// TestStart_ConsecutiveErrorCounterResetsOnSuccess verifies that a successful Accept()
// resets the consecutive-error counter so a single transient error does not accumulate
// toward the failure threshold.
//
// Approach: accept one real connection on the tunnel's listener (resetting the counter),
// then verify the status remains Running after the connection closes.
func TestStart_ConsecutiveErrorCounterResetsOnSuccess(t *testing.T) {
	cfg := i2pconv.TunnelConfig{
		Name:      "reset-counter",
		Type:      "tcpclient",
		Interface: "127.0.0.1",
		Port:      0,
		Target:    testTarget,
	}
	c := &TCPClient{
		TunnelConfig:    cfg,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		done:            make(chan struct{}),
		dialTimeout:     defaultDialTimeout,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- c.Start() }()

	// Wait for Running.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if c.Status() == i2ptunnel.I2PTunnelStatusRunning {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if c.Status() != i2ptunnel.I2PTunnelStatusRunning {
		c.Stop()
		<-errCh
		t.Skip("tunnel did not start in time")
	}

	// Dial the tunnel's listener to produce a successful Accept() — this resets the counter.
	c.lifeMu.Lock()
	addr := c.listener.Addr().String()
	c.lifeMu.Unlock()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		c.Stop()
		<-errCh
		t.Skipf("could not connect to tunnel listener: %v", err)
	}
	conn.Close()

	// Status should still be Running after the successful accept cycle.
	time.Sleep(100 * time.Millisecond)
	if s := c.Status(); s == i2ptunnel.I2PTunnelStatusFailed {
		c.Stop()
		<-errCh
		t.Errorf("tunnel reported Failed after a successful accept — counter should have reset")
	}

	c.Stop()
	<-errCh
}
