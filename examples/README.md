# Example Configuration Files

This directory contains ready-to-use YAML configuration files for every
go-i2ptunnel tunnel type. Copy and modify these for your deployment.

## Client Tunnels

| File | Description |
|---|---|
| [tcpclient.yaml](tcpclient.yaml) | Connect to a remote I2P TCP service |
| [httpclient.yaml](httpclient.yaml) | HTTP/HTTPS proxy for browsing I2P |
| [ircclient.yaml](ircclient.yaml) | Connect to an I2P IRC server |
| [udpclient.yaml](udpclient.yaml) | Forward datagrams to an I2P destination |
| [socksclient.yaml](socksclient.yaml) | SOCKS5 proxy for any I2P destination |

## Server Tunnels

| File | Description |
|---|---|
| [tcpserver.yaml](tcpserver.yaml) | Expose a local TCP service on I2P |
| [httpserver.yaml](httpserver.yaml) | Expose a local web server on I2P |
| [ircserver.yaml](ircserver.yaml) | Expose a local IRC server on I2P |
| [udpserver.yaml](udpserver.yaml) | Receive I2P datagrams and forward locally |

## Bidirectional (P2P) Tunnels

| File | Description |
|---|---|
| [tcpbidirectional.yaml](tcpbidirectional.yaml) | TCP server + SOCKS5 proxy on same keys |
| [httpbidirectional.yaml](httpbidirectional.yaml) | HTTP server + HTTP proxy on same keys |
| [udpbidirectional.yaml](udpbidirectional.yaml) | UDP server + SOCKS5 proxy on same keys |

## Usage

```bash
# Run any example with its corresponding CLI tool
go run ./cmd/http/client -config examples/httpclient.yaml
go run ./cmd/tcp/server -config examples/tcpserver.yaml

# Or with a custom SAM address
go run ./cmd/http/client -config examples/httpclient.yaml -sam 192.168.1.100:7656
```

## Notes

- All examples bind to `127.0.0.1` for security.
- Replace `example.b32.i2p` with actual I2P destination addresses.
- Server tunnels need a local service running on the configured `target` port.
- See [doc/CONFIGURATION.md](../doc/CONFIGURATION.md) for the full option reference.
