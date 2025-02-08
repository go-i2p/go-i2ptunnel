# udpconst
--
    import "github.com/go-i2p/go-i2ptunnel/lib/udp/const"


## Usage

```go
var DatagramForwardConfig = &config.ForwardConfig{
	BufferSize:     32 * 1024,
	IdleTimeout:    30 * time.Second,
	MaxPacketSize:  65507 / 6,
	EnableMetrics:  true,
	ShutdownSignal: make(chan struct{}),
}
```
