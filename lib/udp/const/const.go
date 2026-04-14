// Package udpconst defines shared constants for UDP tunnel implementations.
package udpconst

import (
	"math/rand/v2"
	"time"

	"github.com/go-i2p/go-forward/config"
)

// NewDatagramForwardConfig returns a fresh ForwardConfig with default values
// for UDP datagram forwarding. Each caller gets an independent config and
// ShutdownSignal channel, preventing cross-tunnel interference when one
// tunnel is stopped.
func NewDatagramForwardConfig() *config.ForwardConfig {
	return &config.ForwardConfig{
		BufferSize:     32 * 1024, // 32KB buffer
		IdleTimeout:    30 * time.Second,
		MaxPacketSize:  65507 / 6, // Max UDP packet size
		EnableMetrics:  true,
		ShutdownSignal: make(chan struct{}),
	}
}

// Backoff constants for UDP error retry loops.
const (
	// MinBackoff is the initial retry delay after the first error.
	MinBackoff = 50 * time.Millisecond
	// MaxBackoff is the ceiling for exponential backoff.
	MaxBackoff = 5 * time.Second
)

// NextBackoff computes the next backoff duration using exponential backoff
// with jitter. The returned duration is clamped to [MinBackoff, MaxBackoff].
// current should be the previous backoff duration (use MinBackoff initially).
func NextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > MaxBackoff {
		next = MaxBackoff
	}
	if next < MinBackoff {
		next = MinBackoff
	}
	// Add ±25% jitter to prevent thundering herd.
	jitter := time.Duration(rand.Int64N(int64(next)/2)) - next/4
	return next + jitter
}
