package tcpserver

import (
	"sync"
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

// TestRestartAfterStop verifies that after Stop(), the done channel and stopOnce
// can be reset (as Start() now does) so the tunnel is restartable.
func TestRestartAfterStop(t *testing.T) {
	tunnel := &TCPServer{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		done:            make(chan struct{}),
	}

	tunnel.Stop()
	select {
	case <-tunnel.done:
	default:
		t.Fatal("done channel should be closed after first Stop()")
	}

	tunnel.done = make(chan struct{})
	tunnel.stopOnce = sync.Once{}

	select {
	case <-tunnel.done:
		t.Fatal("done channel should be open after reset")
	default:
	}

	tunnel.Stop()
	select {
	case <-tunnel.done:
	default:
		t.Fatal("done channel should be closed after second Stop()")
	}
	if tunnel.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected status stopped after restart cycle, got %v", tunnel.Status())
	}
}
