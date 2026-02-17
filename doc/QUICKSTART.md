# Quick Start Guide

Get up and running with go-i2ptunnel in minutes.

## Prerequisites

- **Go 1.24+** installed
- **Running I2P router** with SAM bridge enabled (default: `localhost:7656`)
- **Linux** recommended (macOS and Windows supported but less tested)

## Installation

### As a Go Library

```bash
go get github.com/go-i2p/go-i2ptunnel
```

### Build CLI Tools

```bash
git clone https://github.com/go-i2p/go-i2ptunnel.git
cd go-i2ptunnel
go build ./cmd/...
```

This builds all 12 tunnel CLI binaries plus the web UI.

## Configuration File

All tunnels are configured via YAML, `.properties`, or `.ini` files. YAML is recommended.

A minimal configuration file looks like:

```yaml
tunnels:
  my-tunnel:
    name: "my-tunnel"
    type: "tcpclient"
    target: "example.b32.i2p"
    port: 4444
    interface: "127.0.0.1"
```

See [examples/](../examples/) for config files for every tunnel type.

## Tunnel Types at a Glance

| Tunnel Type | Direction | Use Case |
|---|---|---|
| TCP Client | Local → I2P | Connect to an I2P TCP service |
| TCP Server | I2P → Local | Expose a local TCP service on I2P |
| HTTP Client | Local → I2P | HTTP/HTTPS proxy for browsing I2P |
| HTTP Server | I2P → Local | Expose a local web server on I2P |
| IRC Client | Local → I2P | Connect to an I2P IRC server |
| IRC Server | I2P → Local | Expose a local IRC service on I2P |
| UDP Client | Local → I2P | Send datagrams to an I2P destination |
| UDP Server | I2P → Local | Receive datagrams from I2P |
| SOCKS Client | Local → I2P | SOCKS5 proxy for arbitrary I2P destinations |
| TCP Bidirectional | Both | TCP server + SOCKS5 proxy on same keys |
| HTTP Bidirectional | Both | HTTP server + HTTP proxy on same keys |
| UDP Bidirectional | Both | UDP server + SOCKS5 proxy on same keys |

## Quick Examples

### 1. Browse I2P Websites (HTTP Proxy)

Create `http-proxy.yaml`:

```yaml
tunnels:
  i2p-browser:
    name: "i2p-browser"
    type: "httpclient"
    port: 4444
    interface: "127.0.0.1"
```

Run:

```bash
go run ./cmd/http/client -config http-proxy.yaml
```

Configure your browser to use `127.0.0.1:4444` as an HTTP proxy.
Now visit any `.i2p` address.

### 2. Host a Website on I2P (HTTP Server)

Create `web-server.yaml`:

```yaml
tunnels:
  my-site:
    name: "my-site"
    type: "httpserver"
    target: "localhost:8080"
    interface: "127.0.0.1"
```

Run your local web server on port 8080, then:

```bash
go run ./cmd/http/server -config web-server.yaml
```

The tunnel logs will show your `.b32.i2p` address. Share it with others.

### 3. Connect to an I2P IRC Server (IRC Client)

Create `irc-client.yaml`:

```yaml
tunnels:
  my-irc:
    name: "my-irc"
    type: "ircclient"
    target: "irc.echelon.i2p"
    port: 6668
    interface: "127.0.0.1"
```

Run:

```bash
go run ./cmd/irc/client -config irc-client.yaml
```

Point your IRC client at `127.0.0.1:6668`. The tunnel applies privacy
filtering (DCC blocking, hostname masking) automatically.

### 4. Expose a TCP Service (TCP Server)

Create `tcp-server.yaml`:

```yaml
tunnels:
  ssh-tunnel:
    name: "ssh-tunnel"
    type: "tcpserver"
    target: "localhost:22"
    interface: "127.0.0.1"
```

Run:

```bash
go run ./cmd/tcp/server -config tcp-server.yaml
```

Your SSH server is now accessible over I2P at the `.b32.i2p` address
printed in the logs.

### 5. SOCKS5 Proxy for Any I2P Destination

Create `socks-proxy.yaml`:

```yaml
tunnels:
  socks:
    name: "socks"
    type: "socksclient"
    port: 4447
    interface: "127.0.0.1"
```

Run:

```bash
go run ./cmd/socks/client -config socks-proxy.yaml
```

Use as a SOCKS5 proxy at `127.0.0.1:4447` for any application that
supports SOCKS5.

### 6. Bidirectional Tunnel (Serve and Connect)

Create `p2p-tcp.yaml`:

```yaml
tunnels:
  p2p:
    name: "p2p"
    type: "tcpbidirectional"
    target: "localhost:8080"
    port: 4447
    interface: "127.0.0.1"
```

Run:

```bash
go run ./cmd/tcp/bidirectional -config p2p-tcp.yaml
```

This creates a TCP server forwarding to `localhost:8080` **and** a SOCKS5
proxy on port 4447, both sharing the same I2P identity (keys).

## Web UI

The web UI provides a graphical interface for managing all tunnels.

```bash
# Create a config directory
mkdir -p tunnels/

# Copy config files into it (one per tunnel)
cp http-proxy.yaml tunnels/
cp web-server.yaml tunnels/

# Start the web UI
go run ./cmd/web -config tunnels/ -host localhost -port 8089
```

Open `http://localhost:8089` in your browser. The dashboard shows all
configured tunnels with start/stop controls and configuration editing.

## CLI Reference

Every tunnel binary accepts the same flags:

| Flag | Default | Description |
|---|---|---|
| `-config` | *(required)* | Path to tunnel configuration file |
| `-sam` | `127.0.0.1:7656` | SAM bridge address |

### Signal Handling

| Signal | Action |
|---|---|
| `SIGINT` / `SIGTERM` | Graceful shutdown |
| `SIGHUP` | Hot config reload (stop → reload → start) |

### Example with Custom SAM Address

```bash
go run ./cmd/http/client -config proxy.yaml -sam 192.168.1.100:7656
```

## Using as a Go Library

```go
package main

import (
    "fmt"
    "log"

    "github.com/go-i2p/go-i2ptunnel/lib/loader"
)

func main() {
    // Load a tunnel from a config file
    tunnel, err := loader.Load("tunnel.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // Start the tunnel
    if err := tunnel.Start(); err != nil {
        log.Fatal(err)
    }
    defer tunnel.Stop()

    // Print the I2P address
    fmt.Println("I2P address:", tunnel.Address())
    fmt.Println("Local address:", tunnel.LocalAddress())

    // Block forever (or handle signals)
    select {}
}
```

### Creating Tunnels Programmatically

```go
package main

import (
    "log"

    i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
    httpclient "github.com/go-i2p/go-i2ptunnel/lib/http/client"
)

func main() {
    config := i2pconv.TunnelConfig{
        Name:      "my-proxy",
        Type:      "httpclient",
        Port:      4444,
        Interface: "127.0.0.1",
    }

    proxy, err := httpclient.NewHTTPClient(config, "127.0.0.1:7656")
    if err != nil {
        log.Fatal(err)
    }

    if err := proxy.Start(); err != nil {
        log.Fatal(err)
    }
    defer proxy.Stop()

    log.Println("HTTP proxy listening on 127.0.0.1:4444")
    select {}
}
```

## Environment Variables

| Variable | Values | Description |
|---|---|---|
| `DEBUG_I2P` | `debug`, `warn`, `error`, empty | Logging verbosity (empty = disabled) |
| `WARNFAIL_I2P` | `true`, `false` | Fast-fail mode for testing |

## Next Steps

- **[Configuration Reference](CONFIGURATION.md)** — All options for every tunnel type
- **[Deployment Guide](DEPLOYMENT.md)** — Systemd units, production setup, monitoring
- **[CONTRIBUTING.md](../CONTRIBUTING.md)** — Project structure and development guide
