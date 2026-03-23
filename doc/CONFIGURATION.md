# Configuration Reference

Complete reference for all go-i2ptunnel configuration options.

## Configuration Formats

go-i2ptunnel supports three configuration formats, auto-detected by file extension:

| Format | Extensions | Recommended |
|---|---|---|
| YAML | `.yaml`, `.yml` | ✅ Yes |
| Properties | `.properties` | Java I2P compatible |
| INI | `.ini` | Simple key-value |

## YAML Structure

All YAML configs follow this structure:

```yaml
tunnels:
  tunnel-name:
    name: "tunnel-name"
    type: "tcpclient"
    # ... options
```

The top-level `tunnels` map can contain multiple tunnel definitions, but CLI
tools load only the first tunnel entry.

## Common Options

These options apply to **all** tunnel types.

| Option | Type | Required | Default | Description |
|---|---|---|---|---|
| `name` | string | ✅ | — | Tunnel name. Used to generate the tunnel ID and SAM session name. Must be non-empty. |
| `type` | string | ✅ | — | Tunnel type identifier. See [Valid Types](#valid-types). |
| `port` | integer | — | `0` | Local listen port. Range: 1–65535. Ports below 1024 require root privileges and trigger a validation warning. Port `0` means auto-assign. |
| `interface` | string | — | `""` | Local bind address. Must be a valid IP address. Empty string binds to all interfaces. `127.0.0.1` is recommended for security. |

### Valid Types

| Type String | Aliases | Tunnel |
|---|---|---|
| `tcpclient` | — | TCP Client |
| `tcpserver` | — | TCP Server |
| `httpclient` | — | HTTP Proxy |
| `httpserver` | — | HTTP Server |
| `ircclient` | — | IRC Client |
| `ircserver` | — | IRC Server |
| `udpclient` | — | UDP Client |
| `udpserver` | — | UDP Server |
| `socksclient` | `socks` | SOCKS5 Proxy |
| `tcpbidirectional` | — | TCP Bidirectional |
| `httpbidirectional` | — | HTTP Bidirectional |
| `udpbidirectional` | — | UDP Bidirectional |

## Client Tunnel Options

These options apply to **client tunnels** that connect to a fixed I2P
destination: `tcpclient`, `ircclient`, `udpclient`.

| Option | Type | Required | Default | Description |
|---|---|---|---|---|
| `target` | string | ✅ | — | Remote I2P destination address. Accepts `.b32.i2p` (base32) or full base64 destination. Validated by `ValidateI2PAddress()`. |

## Server Tunnel Options

These options apply to **server tunnels** that expose a local service on I2P:
`tcpserver`, `httpserver`, `ircserver`, `udpserver`.

`target` is a top-level field. `maxconns` and `ratelimit` must be placed under
the `options:` key so they are read at tunnel construction time:

| Option | Type | Required | Default | Description |
|---|---|---|---|-----------|
| `target` | string | ✅ | — | Local service address in `host:port` format (e.g., `localhost:8080`). Validated by `ValidateAddress()`. |
| `options.maxconns` | integer | — | `1000` | Maximum concurrent connections. `0` = unlimited. Values above 10,000 trigger a validation warning. |
| `options.ratelimit` | float | — | `100.0` | Maximum new connections per second. `0` = no rate limit. |

## Proxy Tunnel Options

These options apply to **proxy tunnels** that connect to many destinations:
`httpclient`, `socksclient`.

Proxy tunnels have **no `target`** option — the destination is determined per
request from the client.

### HTTP Client (Proxy) Specific

| Option | Type | Required | Default | Description |
|---|---|---|---|---|
| `jumpservice` | string | — | `http://stats.i2p/cgi-bin/jump.cgi` | Jump service URL for resolving human-readable `.i2p` hostnames (e.g., `forum.i2p`). Set to empty string to disable. Only needed for names that are not `.b32.i2p` addresses. |

## Bidirectional Tunnel Options

Bidirectional tunnels combine a **server** and a **client proxy** on the same
I2P identity (keys).

| Type | Server Component | Client Component |
|---|---|---|
| `tcpbidirectional` | TCP server → `target` | SOCKS5 proxy on `port` |
| `httpbidirectional` | HTTP server → `target` | HTTP proxy on `port` |
| `udpbidirectional` | UDP server → `target` | SOCKS5 proxy on `port` |

Bidirectional tunnels accept **both** server and client options:

| Option | Type | Description |
|---|---|---|
| `target` | string | Local service address for the server component |
| `port` | integer | Local listen port for the client proxy component |
| `maxconns` | integer | Connection limit for the server component |
| `ratelimit` | float | Rate limit for the server component |

## I2CP Options (Encrypted LeaseSets)

I2CP options control advanced I2P features, primarily encrypted LeaseSets.
They are specified under the `i2cp` key in YAML, or with the `i2cp.` prefix
in flat key-value formats.

### YAML Format

```yaml
tunnels:
  my-tunnel:
    name: "my-tunnel"
    type: "httpserver"
    target: "localhost:8080"
    i2cp:
      leaseSetType: "5"
      leaseSetEncType: "4,0"
      leaseSetAuthType: "2"
      leaseSetPrivKey: "base64-encoded-key"
```

### Flat Key-Value Format (Options Map)

When using `SetOptions()` or `.properties` files:

```properties
i2cp.leaseSetType=5
i2cp.leaseSetEncType=4,0
i2cp.leaseSetAuthType=2
i2cp.leaseSetPrivKey=base64-encoded-key
```

### I2CP Option Reference

| Option | Values | Default | Description |
|---|---|---|---|
| `leaseSetType` | `1`, `3`, `5` | `1` (Standard) | LeaseSet type. `1` = Standard, `3` = Encrypted, `5` = Meta. |
| `leaseSetEncType` | Comma-separated integers | `4,0` | Encryption types. `4` = ECIES-X25519-AEAD-Ratchet, `0` = ElGamal. |
| `leaseSetAuthType` | `0`, `1`, `2` | `0` (None) | Authentication type. `0` = None, `1` = DH, `2` = PSK. |
| `leaseSetPrivKey` | Base64 string | — | Private key for encrypted LeaseSet. Required when `leaseSetType` is `3` or `5`. |

### Encrypted LeaseSet Quick Reference

**Standard (default)** — No encryption, visible to all:

```yaml
i2cp:
  leaseSetType: "1"
```

**Encrypted** — Hidden from unauthorized peers:

```yaml
i2cp:
  leaseSetType: "3"
  leaseSetEncType: "4,0"
  leaseSetAuthType: "2"
  leaseSetPrivKey: "your-base64-key"
```

## Validation Rules

All options are validated when set via `SetOptions()` or loaded via
`LoadConfig()`. Invalid values are rejected with descriptive error messages.

### Port Validation

- Range: 1–65535
- Warning: Ports below 1024 require root/elevated privileges
- Port `0` is allowed and means auto-assign

### I2P Address Validation

- Must be non-empty
- Must be a valid I2P destination (`.b32.i2p` or base64)
- Validated by `i2pkeys.I2PAddr` parsing

### Network Address Validation

- Must be valid `host:port` format
- Host must be a valid hostname or IP
- Port must be in range 1–65535

### Connection Limit Validation

- `maxconns`: Must be ≥ 0. Warning if > 10,000.
- `ratelimit`: Must be ≥ 0.

### Interface Validation

- Must be a valid IP address or empty string
- Empty string binds to all interfaces

## Complete YAML Examples

### HTTP Proxy (Browser Access)

```yaml
tunnels:
  browser-proxy:
    name: "browser-proxy"
    type: "httpclient"
    port: 4444
    interface: "127.0.0.1"
```

### Web Server with Encrypted LeaseSet

```yaml
tunnels:
  hidden-site:
    name: "hidden-site"
    type: "httpserver"
    target: "localhost:8080"
    interface: "127.0.0.1"
    i2cp:
      leaseSetType: "3"
      leaseSetEncType: "4,0"
      leaseSetAuthType: "0"
```

### IRC Client with Custom Jump Service

```yaml
tunnels:
  irc:
    name: "irc"
    type: "ircclient"
    target: "irc.echelon.i2p"
    port: 6668
    interface: "127.0.0.1"
```

### TCP Server with Rate Limiting

```yaml
tunnels:
  rate-limited-service:
    name: "rate-limited-service"
    type: "tcpserver"
    target: "localhost:22"
    interface: "127.0.0.1"
    options:
      maxconns: 50
      ratelimit: 10.0
```

### SOCKS5 Proxy

```yaml
tunnels:
  socks:
    name: "socks"
    type: "socksclient"
    port: 4447
    interface: "127.0.0.1"
```

### TCP Bidirectional (P2P)

```yaml
tunnels:
  p2p:
    name: "p2p"
    type: "tcpbidirectional"
    target: "localhost:8080"
    port: 4447
    interface: "127.0.0.1"
```

## Properties File Format

For Java I2P compatibility, `.properties` files are supported:

```properties
tunnel.name=my-tunnel
tunnel.type=tcpclient
tunnel.target=example.b32.i2p
tunnel.port=4444
tunnel.interface=127.0.0.1
i2cp.leaseSetEncType=4,0
```

## Encrypted LeaseSet Authentication

I2P blinded destinations (encrypted LeaseSets) can require a credential before
a client may connect to them. go-i2ptunnel supports runtime credential injection
so the private key is never stored in the config file.

### Authentication Types

| `leaseSetAuthType` | Name | Credential required |
|---|---|---|
| `0` | None (default) | No credential needed |
| `1` | DH (Diffie-Hellman) | `i2cp.leaseSetPrivKey` |
| `2` | PSK (Pre-Shared Key) | `i2cp.leaseSetPrivKey` |

### Interactive Key Entry (CLI)

When `leaseSetAuthType` is `1` or `2`, all `go-i2ptunnel-*` CLI binaries prompt
for the Base64-encoded private key on startup:

```
$ go-i2ptunnel-httpclient -config blinded.yaml
Tunnel "http-client" requires encrypted LeaseSet authentication (PSK).
Enter i2cp.leaseSetPrivKey (Base64): <key entered here>
Starting HTTP client tunnel "http-client" on 127.0.0.1:4444
```

The key is read from standard input. On Linux/macOS you can pipe it in
non-interactively:

```sh
echo "Base64EncodedKeyHere==" | go-i2ptunnel-httpclient -config blinded.yaml
```

### Example YAML Config for Blinded Destination

```yaml
tunnels:
  blinded-http:
    name: "blinded-http"
    type: "httpclient"
    port: 4444
    interface: "127.0.0.1"
    i2cp:
      leaseSetType: "5"
      leaseSetEncType: "4,0"
      leaseSetAuthType: "2"
      # Do NOT add leaseSetPrivKey here — supply it interactively at startup
      # or via SetOptions() for programmatic usage.
```

### Programmatic Usage

For applications embedding go-i2ptunnel as a library, inject the credential via
`SetOptions` before calling `Start()`:

```go
tunnel, _ := loader.Load("blinded.yaml", "127.0.0.1:7656")
tunnel.SetOptions(map[string]string{
    "i2cp.leaseSetPrivKey": base64Key,
})
tunnel.Start()
```

## Next Steps

- **[Quick Start Guide](QUICKSTART.md)** — Get running quickly
- **[Deployment Guide](DEPLOYMENT.md)** — Production setup with systemd
