# Go I2PTunnel

A Go implementation of I2P tunneling services with support for TCP, HTTP, UDP, and IRC protocols. Built on SAMv3, this project provides encrypted anonymous network tunnels with filtering, rate-limiting, and encrypted leaseSet capabilities.

## Features

### Server Tunnels
- TCP Server - Standard TCP port forwarding
- HTTP Server - Web service hosting
- IRC Server - Chat service hosting
- UDP Server - Datagram forwarding (Non-standard, uses onramp "hybrid1" mode)

### Client Tunnels
- TCP Client - Direct connection tunneling
- HTTP Proxy - Web browsing support
- SOCKS5 Proxy - Multi-protocol proxy
- IRC Client - Chat connectivity
- UDP Client - Datagram tunneling (Non-standard, uses onramp "hybrid1" mode)

### P2P Tunnels
- UDP Bidirectional - Datagram Forwarding and SOCKS5 Client on same keys (non-standard, uses onramp "hybrid2" mode)
- TCP Bidirectional - TCP Forwarding and SOCKS5 Client on same keys
- HTTP Bidirectional - HTTP Server and HTTP client on same keys

### VPN/Network Interface Support

For TUN/WireGuard-based VPN tunneling, see our dedicated implementation at [github.com/go-i2p/wireguard](https://github.com/go-i2p/wireguard).

## Installation

```bash
go get github.com/go-i2p/go-i2ptunnel
```

## Logging

This project uses the enhanced `github.com/go-i2p/logger` logging system, providing structured logging with configurable verbosity and fast-fail mode for debugging.

### Environment Variables

- **`DEBUG_I2P`**: Control logging verbosity
  - `debug` - Verbose debugging information
  - `warn` - Warnings and errors only
  - `error` - Errors only
  - Unset or empty - Logging disabled (default)

- **`WARNFAIL_I2P`**: Enable fast-fail mode for testing
  - `true` - Warnings and errors become fatal
  - `false` or unset - Normal operation (default)

### Usage Examples

```bash
# Enable debug logging
export DEBUG_I2P=debug
go run cmd/web/main.go

# Enable warnings only
export DEBUG_I2P=warn
./your-program

# Enable fast-fail mode for testing
export WARNFAIL_I2P=true
go test ./...

# Run with no logging (production default)
./your-program
```

### Structured Logging Features

The logger provides:

- Zero-impact when disabled - No performance overhead in production
- Structured fields for searchable logs
- Error context tracking with `WithError()`
- Rich metadata support with `WithField()` and `WithFields()`
- Integration with logrus for advanced features

## Documentation

- **[Quick Start Guide](doc/QUICKSTART.md)** — Get running in minutes with examples for every tunnel type
- **[Configuration Reference](doc/CONFIGURATION.md)** — Complete option documentation
- **[Deployment Guide](doc/DEPLOYMENT.md)** — Production setup with systemd, logging, security
- **[Example Configs](examples/)** — Ready-to-use YAML configs for all 12 tunnel types

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
