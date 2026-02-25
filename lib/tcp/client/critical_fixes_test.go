package tcpclient

// Tests for the four critical bugs fixed in tcpclient.go:
//   1. Target() nil-pointer panic guard
//   2. stream.Forward error recorded instead of silently dropped
//   3. SetOptions() data race: validate-first, write-under-lifeMu pattern
//   4. LoadConfig() race: t.I2PTunnelStatus read replaced with mutex-protected status variable

import (
	"sync"
	"testing"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// ---------------------------------------------------------------------------
// Fix 1: Target() nil-pointer panic guard
// ---------------------------------------------------------------------------

// TestTarget_NilAddr ensures Target() returns "" when I2PAddr is nil instead of panicking.
// Before the fix, calling Target() on a zero-value TCPClient (e.g. in test harnesses or
// when NewTCPClient fails mid-way) would nil-dereference inside handleConnection goroutines
// and bring down the whole process — unrecoverable because goroutine panics cannot be caught
// by the caller.
func TestTarget_NilAddr(t *testing.T) {
	c := &TCPClient{} // I2PAddr is nil
	got := c.Target()
	if got != "" {
		t.Errorf("Target() with nil I2PAddr = %q, want \"\"", got)
	}
}

// TestTarget_NonNilAddr ensures Target() still returns the base32 address when set.
// This is a regression guard so the nil-check doesn't break the normal path.
func TestTarget_NonNilAddr(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("target-test"), "localhost:7656")
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	got := tunnel.Target()
	if got == "" {
		t.Error("Target() returned empty string for a properly initialised tunnel")
	}
	// Must end in .b32.i2p — standard SAMv3 base32 suffix
	if len(got) < 9 || got[len(got)-8:] != ".b32.i2p" {
		t.Errorf("Target() = %q, expected *.b32.i2p format", got)
	}
}

// ---------------------------------------------------------------------------
// Fix 2: handleConnection error recording (stream.Forward return value)
// ---------------------------------------------------------------------------

// TestHandleConnection_RecordsTargetMissingError verifies that if Target() returns ""
// (nil I2PAddr), handleConnection records an error rather than silently no-oping or
// panicking.  This is the only unit-testable slice of the fix because stream.Forward
// itself requires a live I2P session.
func TestHandleConnection_RecordsNoTargetError(t *testing.T) {
	c := &TCPClient{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		done:            make(chan struct{}),
		// I2PAddr intentionally nil → Target() returns ""
	}

	// Use a self-connected local pipe so handleConnection has a valid net.Conn.
	server, client := localPipe(t)
	defer server.Close()
	defer client.Close()

	c.handleConnection(client) // must not panic

	if len(c.ErrorHistory()) == 0 {
		t.Error("handleConnection should record an error when Target() is empty, but ErrorHistory() is empty")
	}
}

// ---------------------------------------------------------------------------
// Fix 3: SetOptions() data race — writes protected by lifeMu
// ---------------------------------------------------------------------------

// TestSetOptions_ConcurrentWithStatus verifies that SetOptions can be called
// concurrently without data races (detectable via `go test -race`).
// The test hammers SetOptions and Status() from separate goroutines to expose
// any unsynchronised field access.
func TestSetOptions_ConcurrentWithStatus(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("setopts-race"), "localhost:7656")
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	var wg sync.WaitGroup
	const iterations = 50

	// Writer goroutine: repeatedly set the port option
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = tunnel.SetOptions(map[string]string{"port": "9876"})
		}
	}()

	// Reader goroutine: concurrently read status (accesses same struct)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = tunnel.Status()
		}
	}()

	wg.Wait()
}

// TestSetOptions_InvalidPortRejected verifies validation still fires before any write.
func TestSetOptions_InvalidPortRejected(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("setopts-validate"), "localhost:7656")
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	originalPort := tunnel.TunnelConfig.Port
	err = tunnel.SetOptions(map[string]string{"port": "99999"}) // > 65535
	if err == nil {
		t.Fatal("SetOptions accepted out-of-range port, want error")
	}
	if tunnel.TunnelConfig.Port != originalPort {
		t.Errorf("Port was mutated to %d on validation failure; want original %d",
			tunnel.TunnelConfig.Port, originalPort)
	}
}

// TestSetOptions_PartialValidationFailureNoSideEffect verifies that when one option
// in a batch is invalid, none of the preceding valid options are applied (validate-first
// ensures atomicity: either all options accepted or none written).
func TestSetOptions_PartialValidationFailureNoSideEffect(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("setopts-partial"), "localhost:7656")
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	originalName := tunnel.TunnelConfig.Name
	originalPort := tunnel.TunnelConfig.Port

	// "name" is valid but "port" is invalid.  Under the old code, name would be written
	// before port validation failed.  Under the new code, all validation happens first
	// so the name must remain unchanged on failure.
	err = tunnel.SetOptions(map[string]string{
		"name": "updated-name",
		"port": "not-a-number",
	})
	if err == nil {
		t.Fatal("SetOptions should fail on invalid port")
	}
	if tunnel.TunnelConfig.Name != originalName {
		t.Errorf("Name changed to %q despite validation failure; atomicity broken", tunnel.TunnelConfig.Name)
	}
	if tunnel.TunnelConfig.Port != originalPort {
		t.Errorf("Port changed to %d despite validation failure", tunnel.TunnelConfig.Port)
	}
}

// ---------------------------------------------------------------------------
// Fix 4: LoadConfig() uses `status` variable, not t.I2PTunnelStatus directly
// ---------------------------------------------------------------------------

// TestLoadConfig_ErrorMessageUsesStatusVariable checks that the error returned
// by LoadConfig while running contains the correct status string.
// This is a regression guard for the race: previously the error format read
// t.I2PTunnelStatus without holding statusMu, which is a data race under -race.
// The fix captures status := t.Status() first and reuses that value in the message.
func TestLoadConfig_ErrorMessageUsesStatusVariable(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("lc-status-msg"), "localhost:7656")
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	tunnel.setStatus(i2ptunnel.I2PTunnelStatusRunning)

	loadErr := tunnel.LoadConfig("/tmp/nonexistent.yaml")
	if loadErr == nil {
		t.Fatal("Expected LoadConfig to fail when tunnel is running")
	}
	const want = "cannot load config while tunnel is running - stop tunnel first"
	if loadErr.Error() != want {
		t.Errorf("LoadConfig error = %q, want %q", loadErr.Error(), want)
	}
}

// TestLoadConfig_ErrorMessageStarting mirrors the above for I2PTunnelStatusStarting.
func TestLoadConfig_ErrorMessageStarting(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("lc-starting-msg"), "localhost:7656")
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	tunnel.setStatus(i2ptunnel.I2PTunnelStatusStarting)

	loadErr := tunnel.LoadConfig("/tmp/nonexistent.yaml")
	if loadErr == nil {
		t.Fatal("Expected LoadConfig to fail when tunnel is starting")
	}
	const want = "cannot load config while tunnel is starting - stop tunnel first"
	if loadErr.Error() != want {
		t.Errorf("LoadConfig error = %q, want %q", loadErr.Error(), want)
	}
}
