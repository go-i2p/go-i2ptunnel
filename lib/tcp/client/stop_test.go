package tcpclient

import (
	"testing"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// TestStopIdempotent verifies that calling Stop() multiple times does not panic.
// Why: The done channel is closed in Stop(). Without sync.Once, a second call to
// close() panics with "close of closed channel". This test ensures the fix works.
func TestStopIdempotent(t *testing.T) {
	tunnel := &TCPClient{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		done:            make(chan struct{}),
	}

	// First stop should succeed
	if err := tunnel.Stop(); err != nil {
		t.Fatalf("First Stop() returned error: %v", err)
	}
	if tunnel.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected status stopped after first Stop(), got %v", tunnel.Status())
	}

	// Second stop should NOT panic and should succeed
	if err := tunnel.Stop(); err != nil {
		t.Fatalf("Second Stop() returned error: %v", err)
	}
	if tunnel.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected status stopped after second Stop(), got %v", tunnel.Status())
	}

	// Third stop — still safe
	if err := tunnel.Stop(); err != nil {
		t.Fatalf("Third Stop() returned error: %v", err)
	}
}

// TestDoneChannelSignaling verifies the done channel is properly closed on Stop().
func TestDoneChannelSignaling(t *testing.T) {
	tunnel := &TCPClient{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		done:            make(chan struct{}),
	}

	// Channel should not be closed initially
	select {
	case <-tunnel.done:
		t.Fatal("done channel should not be closed before Stop()")
	default:
		// expected
	}

	tunnel.Stop()

	// Channel should now be closed
	select {
	case <-tunnel.done:
		// expected — channel is closed
	default:
		t.Fatal("done channel should be closed after Stop()")
	}
}
