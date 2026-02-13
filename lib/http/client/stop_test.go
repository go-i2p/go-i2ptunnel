package httpclient

import (
	"testing"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// TestStopIdempotent verifies that calling Stop() multiple times does not panic.
// HTTPClient uses a mutex-guarded Stop() with a nil check on Server, plus sync.Once
// for the done channel. When Server is nil (never started), Stop() is a no-op.
func TestStopIdempotent(t *testing.T) {
	tunnel := &HTTPClient{
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
