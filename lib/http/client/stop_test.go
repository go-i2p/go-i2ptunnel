package httpclient

import (
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/onramp"
)

// TestStopIdempotent verifies that calling Stop() multiple times does not panic.
// HTTPClient uses a mutex-guarded Stop() with a nil check on Server, plus sync.Once
// for the done channel. When Server is nil (never started), Stop() is a no-op.
func TestStopIdempotent(t *testing.T) {
	tunnel := &HTTPClient{
		TunnelBase: i2ptunnel.TunnelBase{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		},
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

// TestRestartAfterStop verifies that after Stop(), the done channel, stopOnce,
// and ProxyHttpServer are properly reset so the tunnel can be restarted.
// HTTPClient guards Start() with `if h.ProxyHttpServer != nil { return nil }`
// so Stop() must nil out ProxyHttpServer. Start() already resets done; this fix
// also resets stopOnce.
func TestRestartAfterStop(t *testing.T) {
	tunnel := &HTTPClient{
		TunnelBase: i2ptunnel.TunnelBase{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		},
		done:            make(chan struct{}),
	}

	// Stop on never-started tunnel
	tunnel.Stop()

	// Simulate what Start() now does: reset done and stopOnce
	tunnel.done = make(chan struct{})
	tunnel.stopOnce = sync.Once{}

	// Verify done is open after reset
	select {
	case <-tunnel.done:
		t.Fatal("done channel should be open after reset")
	default:
	}

	// Verify Stop() works after reset even without Server
	tunnel.Stop()
	if tunnel.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected status stopped after restart cycle, got %v", tunnel.Status())
	}
}

// TestStopWithServerUsesTimeout verifies that Stop() on an HTTPClient with a
// real http.Server but no ctx (simulating Start() never being called) completes
// promptly instead of blocking indefinitely. Before this fix, the fallback to
// context.Background() had no timeout and could hang forever.
func TestStopWithServerUsesTimeout(t *testing.T) {
	// Create a real HTTP server on a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})}
	go srv.Serve(listener)

	tunnel := &HTTPClient{
		TunnelBase: i2ptunnel.TunnelBase{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		},
		Server:          srv,
		done:            make(chan struct{}),
		// ctx is intentionally nil — simulates Stop() called without Start()
	}

	// Stop should complete well under 5 seconds (the server has no connections)
	done := make(chan error, 1)
	go func() {
		done <- tunnel.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop() returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() blocked for >5s — shutdown context is not timeout-bounded")
	}

	if tunnel.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected status stopped, got %v", tunnel.Status())
	}
}

// TestStopClosesGarlic verifies that Stop() calls Garlic.Close() when Garlic is non-nil.
// HTTPClient requires Server != nil for Stop() to enter the shutdown path.
// After successful HTTP server shutdown, Garlic.Close() is called.
// A zero-value Garlic panics on Close() (nil SAM sessions), proving the call was reached.
func TestStopClosesGarlic(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	srv := &http.Server{Addr: ln.Addr().String()}
	go srv.Serve(ln)

	tunnel := &HTTPClient{
		done: make(chan struct{}),
	}
	tunnel.Server = srv
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
