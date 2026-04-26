package tcpserver

import (
	"os"
	"sync"
	"testing"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/onramp"
)

// TestStopIdempotent verifies that calling Stop() multiple times does not panic.
func TestStopIdempotent(t *testing.T) {
	tunnel := &TCPServer{
		TunnelBase: i2ptunnel.TunnelBase{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		},
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
		TunnelBase: i2ptunnel.TunnelBase{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		},
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
		TunnelBase: i2ptunnel.TunnelBase{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		},
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

// TestStopClosesGarlic verifies that Stop() calls Garlic.Close() when Garlic is non-nil.
func TestStopClosesGarlic(t *testing.T) {
	tunnel := &TCPServer{
		done: make(chan struct{}),
	}
	tunnel.Garlic = &onramp.Garlic{}

	didPanic := func() (panicked bool) {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		tunnel.Stop()
		return false
	}()

	if !didPanic {
		t.Fatal("expected Stop() to call Garlic.Close() on non-nil Garlic")
	}
}

// TestStartNilGarlic verifies Start() panics when Garlic is nil.
func TestStartNilGarlic(t *testing.T) {
	tunnel := &TCPServer{
		done: make(chan struct{}),
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Start() with nil Garlic should panic")
		}
	}()
	tunnel.Start()
}

// TestConcurrentStop verifies that concurrent Stop() calls don't race.
func TestConcurrentStop(t *testing.T) {
	tunnel := &TCPServer{
		TunnelBase: i2ptunnel.TunnelBase{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		},
		done:            make(chan struct{}),
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tunnel.Stop()
		}()
	}
	wg.Wait()
	if tunnel.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected stopped status, got %v", tunnel.Status())
	}
}

// TestLoadConfigNonexistentFile verifies LoadConfig returns error for missing files.
func TestLoadConfigNonexistentFile(t *testing.T) {
	tunnel := &TCPServer{
		TunnelBase: i2ptunnel.TunnelBase{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		},
		done:            make(chan struct{}),
	}
	err := tunnel.LoadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("LoadConfig should fail for nonexistent file")
	}
}

// TestLoadConfigWrongType verifies LoadConfig rejects mismatched tunnel type.
func TestLoadConfigWrongType(t *testing.T) {
	tunnel := &TCPServer{
		TunnelBase: i2ptunnel.TunnelBase{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		},
		done:            make(chan struct{}),
	}
	dir := t.TempDir()
	cfg := dir + "/wrong.yaml"
	os.WriteFile(cfg, []byte("type: httpclient\nname: wrong\nport: 8080\ninterface: 127.0.0.1\n"), 0600)
	err := tunnel.LoadConfig(cfg)
	if err == nil {
		t.Fatal("LoadConfig should reject wrong tunnel type")
	}
}
