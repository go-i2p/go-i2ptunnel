package udpconst

import (
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
