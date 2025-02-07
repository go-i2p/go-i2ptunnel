package udpconst

import (
	"time"

	"github.com/go-i2p/go-forward/config"
)

var DatagramForwardConfig = &config.ForwardConfig{
	BufferSize:     32 * 1024, // 32KB buffer
	IdleTimeout:    30 * time.Second,
	MaxPacketSize:  65507 / 6, // Max UDP packet size
	EnableMetrics:  true,
	ShutdownSignal: make(chan struct{}),
}
