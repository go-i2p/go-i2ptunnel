# Go I2PTunnel

A Go implementation of I2P tunneling services with support for TCP, HTTP, UDP, and IRC protocols. Built on SAMv3, this project provides encrypted anonymous network tunnels with filtering, rate-limiting, and encrypted leaseSet capabilities.

## Features

### Server Tunnels
- TCP Server - Standard TCP port forwarding
- HTTP Server - Web service hosting
- IRC Server - Chat service hosting
- UDP Server - Datagram forwarding (Non-standard)

### Client Tunnels
- TCP Client - Direct connection tunneling
- HTTP Proxy - Web browsing support
- SOCKS5 Proxy - Multi-protocol proxy
- IRC Client - Chat connectivity
- UDP Client - Datagram tunneling
- TUN Device - Network interface tunneling (Linux)

## Installation

```bash
go get github.com/go-i2p/go-i2ptunnel
```

## Usage

### TCP Server Example
```go
import "github.com/go-i2p/go-i2ptunnel/lib/tcp/server"

tunnel := tcpserver.NewTCPServer(map[string]string{
    "name": "my-service",
    "port": "8080",
})

if err := tunnel.Start(); err != nil {
    log.Fatal(err)
}
```

### HTTP Proxy Example
```go
import "github.com/go-i2p/go-i2ptunnel/lib/http/client"

proxy := httpclient.NewHTTPProxy(map[string]string{
    "name": "http-proxy",
    "port": "8118",
})

if err := proxy.Start(); err != nil {
    log.Fatal(err)
}
```

## Configuration

Each tunnel type supports these common options:
- `name` - Tunnel identifier
- `type` - Tunnel type (server/client)
- `port` - Local port to bind
- `host` - Local host to bind (default: 127.0.0.1)
- `keys` - Path to key file
- `destination` - Target I2P address (clients only)

## Contributing

1. Check our [CONTRIBUTING.md](CONTRIBUTING.md)
2. Fork the repository
3. Create feature branch
4. Implement changes
5. Add tests
6. Submit PR

## Testing

```bash
go test ./...
```

Review test output and ensure all tunnel types function correctly.

## License

MIT License

## Acknowledgements

- Based on the I2P Project's tunnel specifications
- Uses SAMv3 protocol for I2P connectivity