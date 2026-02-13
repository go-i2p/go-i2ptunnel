package tcpserver

import (
	"testing"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// TestStopIdempotent verifies that calling Stop() multiple times does not panic.
func TestStopIdempotent(t *testing.T) {
	tunnel := &TCPServer{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		done:            make(chan struct{}),
	}

	if err := tunnel.Stop(); err != nil {
		t.Fatalf("First Stop() returned error: %v", err)
	}
	if tunnel.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected status stopped, got %v", tunnel.Status())
	}

	// Second stop should NOT panic
	if err := tunnel.Stop(); err != nil {
		t.Fatalf("Second Stop() returned error: %v", err)
	}
}

// TestDoneChannelSignaling verifies the done channel is properly closed on Stop().
func TestDoneChannelSignaling(t *testing.T) {
	tunnel := &TCPServer{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		done:            make(chan struct{}),
	}

	select {
	case <-tunnel.done:
		t.Fatal("done channel should not be closed before Stop()")
	default:
	}

	tunnel.Stop()

	select {
	case <-tunnel.done:
		// expected
	default:
		t.Fatal("done channel should be closed after Stop()")
	}
}
