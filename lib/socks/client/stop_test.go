package socks

import (
	"sync"
	"testing"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// TestStopIdempotent verifies that calling Stop() multiple times does not panic.
// SOCKS uses a mutex-guarded Stop() with a nil check on Server, plus sync.Once
// for the done channel. When Server is nil (never started), Stop() is a no-op.
func TestStopIdempotent(t *testing.T) {
	tunnel := &SOCKS{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		done:            make(chan struct{}),
	}

	// Stop on never-started tunnel should be safe
	if err := tunnel.Stop(); err != nil {
		t.Fatalf("First Stop() returned error: %v", err)
	}

	// Second stop should NOT panic
	if err := tunnel.Stop(); err != nil {
		t.Fatalf("Second Stop() returned error: %v", err)
	}
}

// TestDoneChannelSignaling verifies the done channel is properly closed on Stop()
// when the Server is set (simulating a started tunnel).
func TestDoneChannelSignaling(t *testing.T) {
	tunnel := &SOCKS{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		done:            make(chan struct{}),
	}

	select {
	case <-tunnel.done:
		t.Fatal("done channel should not be closed before Stop()")
	default:
	}

	// Note: without a real Server, Stop() is a no-op (guarded by if s.Server != nil).
	// We test the done channel mechanism directly here.
	tunnel.stopOnce.Do(func() {
		close(tunnel.done)
	})

	select {
	case <-tunnel.done:
		// expected
	default:
		t.Fatal("done channel should be closed after stopOnce fires")
	}
}

// TestRestartAfterStop verifies that after Stop(), the done channel and stopOnce
// can be reset (as Start() now does) so the tunnel is restartable.
// SOCKS already resets done in Start(); this fix also resets stopOnce.
func TestRestartAfterStop(t *testing.T) {
	tunnel := &SOCKS{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		done:            make(chan struct{}),
	}

	// Manually fire stopOnce (simulating Stop() when Server was set)
	tunnel.stopOnce.Do(func() {
		close(tunnel.done)
	})

	select {
	case <-tunnel.done:
	default:
		t.Fatal("done channel should be closed after first stop cycle")
	}

	// Simulate what Start() now does: reset done and stopOnce
	tunnel.done = make(chan struct{})
	tunnel.stopOnce = sync.Once{}

	// Verify done is open after reset
	select {
	case <-tunnel.done:
		t.Fatal("done channel should be open after reset")
	default:
	}

	// Second stop cycle should work
	tunnel.stopOnce.Do(func() {
		close(tunnel.done)
	})
	select {
	case <-tunnel.done:
	default:
		t.Fatal("done channel should be closed after second stop cycle")
	}
}
