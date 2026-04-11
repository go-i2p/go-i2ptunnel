package udpconst

import (
	"testing"
	"time"
)

func TestNewDatagramForwardConfig(t *testing.T) {
	cfg := NewDatagramForwardConfig()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.BufferSize != 32*1024 {
		t.Errorf("BufferSize = %d, want %d", cfg.BufferSize, 32*1024)
	}
	if cfg.IdleTimeout != 30*time.Second {
		t.Errorf("IdleTimeout = %v, want %v", cfg.IdleTimeout, 30*time.Second)
	}
	if cfg.MaxPacketSize != 65507/6 {
		t.Errorf("MaxPacketSize = %d, want %d", cfg.MaxPacketSize, 65507/6)
	}
	if !cfg.EnableMetrics {
		t.Error("EnableMetrics should be true")
	}
	if cfg.ShutdownSignal == nil {
		t.Error("ShutdownSignal channel should not be nil")
	}
}

func TestNewDatagramForwardConfigIndependence(t *testing.T) {
	cfg1 := NewDatagramForwardConfig()
	cfg2 := NewDatagramForwardConfig()
	if cfg1 == cfg2 {
		t.Error("each call should return an independent config")
	}
	// Closing one shutdown channel should not affect the other.
	close(cfg1.ShutdownSignal)
	select {
	case <-cfg2.ShutdownSignal:
		t.Error("closing cfg1's ShutdownSignal should not close cfg2's")
	default:
		// expected
	}
}
