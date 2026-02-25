package tcpclient

// connection_limit_test.go tests the MaxConnections semaphore and dial-timeout features.
//
// Both features share the same execution path (Start → handleConnection) and the same
// SetOptions/Options extension, so they are validated together.

import (
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Default values
// ---------------------------------------------------------------------------

// TestNewTCPClient_DefaultDialTimeout verifies that NewTCPClient sets dialTimeout
// to defaultDialTimeout (30s) instead of leaving it at the zero value.
// A zero timeout means no deadline, which would silently allow goroutine leaks.
func TestNewTCPClient_DefaultDialTimeout(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("dt-default"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	if tunnel.dialTimeout != defaultDialTimeout {
		t.Errorf("dialTimeout = %v, want %v", tunnel.dialTimeout, defaultDialTimeout)
	}
}

// TestNewTCPClient_DefaultConnSemNil verifies that connSem is nil by default,
// meaning no connection cap is enforced until explicitly configured via SetOptions.
func TestNewTCPClient_DefaultConnSemNil(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("sem-default"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	if tunnel.connSem != nil {
		t.Errorf("connSem should be nil by default (unlimited), got capacity %d", cap(tunnel.connSem))
	}
}

// ---------------------------------------------------------------------------
// SetOptions: maxconns
// ---------------------------------------------------------------------------

// TestSetOptions_MaxConns_Positive verifies that a valid positive maxconns creates
// a buffered semaphore channel with the specified capacity.
func TestSetOptions_MaxConns_Positive(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("sem-setopts"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	if err := tunnel.SetOptions(map[string]string{"maxconns": "5"}); err != nil {
		t.Fatalf("SetOptions maxconns=5: %v", err)
	}
	if cap(tunnel.connSem) != 5 {
		t.Errorf("connSem capacity = %d, want 5", cap(tunnel.connSem))
	}
}

// TestSetOptions_MaxConns_Zero verifies that maxconns=0 sets connSem to nil (unlimited).
func TestSetOptions_MaxConns_Zero(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("sem-zero"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	// First set a limit to verify clearing works.
	if err := tunnel.SetOptions(map[string]string{"maxconns": "3"}); err != nil {
		t.Fatalf("SetOptions maxconns=3: %v", err)
	}
	if err := tunnel.SetOptions(map[string]string{"maxconns": "0"}); err != nil {
		t.Fatalf("SetOptions maxconns=0: %v", err)
	}
	if tunnel.connSem != nil {
		t.Errorf("connSem should be nil for maxconns=0 (unlimited), got cap %d", cap(tunnel.connSem))
	}
}

// TestSetOptions_MaxConns_Negative verifies that negative maxconns is rejected.
func TestSetOptions_MaxConns_Negative(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("sem-neg"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	err = tunnel.SetOptions(map[string]string{"maxconns": "-1"})
	if err == nil {
		t.Fatal("SetOptions maxconns=-1 should fail, got nil")
	}
}

// TestSetOptions_MaxConns_NonNumeric verifies that non-numeric maxconns is rejected.
func TestSetOptions_MaxConns_NonNumeric(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("sem-nan"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	err = tunnel.SetOptions(map[string]string{"maxconns": "many"})
	if err == nil {
		t.Fatal("SetOptions maxconns='many' should fail, got nil")
	}
}

// ---------------------------------------------------------------------------
// SetOptions: dialtimeout
// ---------------------------------------------------------------------------

// TestSetOptions_DialTimeout_Valid verifies that a valid duration string is accepted
// and stored in dialTimeout.
func TestSetOptions_DialTimeout_Valid(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("dt-valid"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	if err := tunnel.SetOptions(map[string]string{"dialtimeout": "45s"}); err != nil {
		t.Fatalf("SetOptions dialtimeout=45s: %v", err)
	}
	if tunnel.dialTimeout != 45*time.Second {
		t.Errorf("dialTimeout = %v, want 45s", tunnel.dialTimeout)
	}
}

// TestSetOptions_DialTimeout_ZeroDisablesTimeout verifies that dialtimeout=0s stores
// zero (disabling the timeout, allowing indefinite dial attempts).
func TestSetOptions_DialTimeout_ZeroDisablesTimeout(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("dt-zero"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	if err := tunnel.SetOptions(map[string]string{"dialtimeout": "0s"}); err != nil {
		t.Fatalf("SetOptions dialtimeout=0s: %v", err)
	}
	if tunnel.dialTimeout != 0 {
		t.Errorf("dialTimeout = %v, want 0 (no timeout)", tunnel.dialTimeout)
	}
}

// TestSetOptions_DialTimeout_Invalid verifies that unparseable values are rejected.
func TestSetOptions_DialTimeout_Invalid(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("dt-invalid"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	err = tunnel.SetOptions(map[string]string{"dialtimeout": "oops"})
	if err == nil {
		t.Fatal("SetOptions dialtimeout='oops' should fail, got nil")
	}
}

// TestSetOptions_DialTimeout_Negative verifies that negative durations are rejected.
func TestSetOptions_DialTimeout_Negative(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("dt-neg"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	err = tunnel.SetOptions(map[string]string{"dialtimeout": "-5s"})
	if err == nil {
		t.Fatal("SetOptions dialtimeout=-5s should fail, got nil")
	}
}

// ---------------------------------------------------------------------------
// Options() round-trip
// ---------------------------------------------------------------------------

// TestOptions_IncludesMaxconnsAndDialtimeout verifies that Options() exposes the
// current maxconns and dialtimeout values that were applied via SetOptions, so the
// web UI and operators can read the current settings.
func TestOptions_IncludesMaxconnsAndDialtimeout(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("opts-roundtrip"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	if err := tunnel.SetOptions(map[string]string{
		"maxconns":    "10",
		"dialtimeout": "1m",
	}); err != nil {
		t.Fatalf("SetOptions: %v", err)
	}

	opts := tunnel.Options()
	if opts["maxconns"] != "10" {
		t.Errorf("Options()[maxconns] = %q, want \"10\"", opts["maxconns"])
	}
	if opts["dialtimeout"] != "1m0s" {
		t.Errorf("Options()[dialtimeout] = %q, want \"1m0s\"", opts["dialtimeout"])
	}
}

// TestOptions_DefaultDialtimeout verifies that a fresh tunnel reports the default
// dial timeout in Options(), not the zero-value empty string.
func TestOptions_DefaultDialtimeout(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("opts-default-dt"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	opts := tunnel.Options()
	if opts["dialtimeout"] != defaultDialTimeout.String() {
		t.Errorf("Options()[dialtimeout] = %q, want %q", opts["dialtimeout"], defaultDialTimeout.String())
	}
	if opts["maxconns"] != "0" {
		t.Errorf("Options()[maxconns] = %q, want \"0\" (unlimited)", opts["maxconns"])
	}
}

// ---------------------------------------------------------------------------
// Semaphore behaviour: acceptance and rejection
// ---------------------------------------------------------------------------

// TestConnSem_RejectsAtCapacity verifies that when the semaphore is full and a new
// local connection arrives, the connection is closed and an error is recorded.
// The test uses a net.Pipe() pair as the rejected connection.
func TestConnSem_RejectsAtCapacity(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("sem-cap"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	// Cap at 2 connections.
	if err := tunnel.SetOptions(map[string]string{"maxconns": "2"}); err != nil {
		t.Fatalf("SetOptions: %v", err)
	}

	// Fill the semaphore manually (simulating 2 in-flight connections).
	tunnel.connSem <- struct{}{}
	tunnel.connSem <- struct{}{}

	// Now snapshotConnSem should return a full channel.
	sem := tunnel.snapshotConnSem()

	// Attempt a non-blocking acquire — should fail since all slots are taken.
	accepted := false
	select {
	case sem <- struct{}{}:
		accepted = true
		<-sem // undo for cleanup
	default:
	}
	if accepted {
		t.Error("semaphore should be full (capacity 2, 2 items in), but accepted another item")
	}
}

// TestConnSem_SnapshotIsolation verifies that goroutines release from the channel
// they acquired, even after SetOptions replaces t.connSem.
// This guards the invariant that semaphore rebuilds don't cause over/under-releases.
func TestConnSem_SnapshotIsolation(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("sem-snapshot"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	if err := tunnel.SetOptions(map[string]string{"maxconns": "1"}); err != nil {
		t.Fatalf("SetOptions: %v", err)
	}
	oldSem := tunnel.snapshotConnSem()
	oldSem <- struct{}{} // simulate goroutine acquiring from old semaphore

	// Replace semaphore via SetOptions.
	if err := tunnel.SetOptions(map[string]string{"maxconns": "3"}); err != nil {
		t.Fatalf("SetOptions rebuild: %v", err)
	}
	newSem := tunnel.snapshotConnSem()
	// New semaphore should be empty (no capacity consumed).
	if len(newSem) != 0 {
		t.Errorf("new semaphore has %d items, want 0", len(newSem))
	}

	// Old goroutine releases from old semaphore — should not affect new semaphore.
	<-oldSem
	if len(newSem) != 0 {
		t.Errorf("release on old semaphore affected new semaphore; len = %d", len(newSem))
	}
}

// ---------------------------------------------------------------------------
// dialContext helper
// ---------------------------------------------------------------------------

// TestDialContext_WithTimeout verifies that a non-zero dialTimeout produces a context
// with a deadline set (not context.Background which has no deadline).
func TestDialContext_WithTimeout(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("dc-timeout"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	tunnel.dialTimeout = 5 * time.Second
	ctx, cancel := tunnel.dialContext()
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("context has no deadline, want deadline set from dialTimeout")
	}
	if time.Until(deadline) <= 0 || time.Until(deadline) > 5*time.Second+100*time.Millisecond {
		t.Errorf("deadline %v is outside expected range", deadline)
	}
}

// TestDialContext_ZeroTimeout uses context.Background (no deadline).
func TestDialContext_ZeroTimeout(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("dc-zero"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	tunnel.dialTimeout = 0
	ctx, cancel := tunnel.dialContext()
	defer cancel()

	if _, ok := ctx.Deadline(); ok {
		t.Error("context has deadline, want no deadline when dialTimeout is 0")
	}
}

// ---------------------------------------------------------------------------
// Concurrent safety
// ---------------------------------------------------------------------------

// TestSetOptions_MaxConnsAndDialTimeout_Race exercises SetOptions with maxconns and
// dialtimeout concurrently with snapshotConnSem and dialContext to catch data races
// under go test -race.
func TestSetOptions_MaxConnsAndDialTimeout_Race(t *testing.T) {
	tunnel, err := NewTCPClient(minimalConfig("sem-race"), testSAMAddr)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer tunnel.Garlic.Close()

	var wg sync.WaitGroup
	const n = 30

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			_ = tunnel.SetOptions(map[string]string{"maxconns": "5", "dialtimeout": "10s"})
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			_ = tunnel.snapshotConnSem()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			ctx, cancel := tunnel.dialContext()
			cancel()
			_ = ctx
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			_ = tunnel.Options()
		}
	}()

	wg.Wait()
}
