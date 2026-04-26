# Embedding I2PTunnel in a Go Application

The `lib/embedding` package provides a high-level API for running I2P tunnels inside Go programs. It handles lifecycle management (start, stop, reload), optional Prometheus metrics, and OS signal routing so that your `main()` can be as small as two lines.

Import path:

```go
import "github.com/go-i2p/go-i2ptunnel/lib/embedding"
```

---

## Table of Contents

1. [HTTP Proxy Client](#1-http-proxy-client)
2. [SOCKS5 Proxy Client](#2-socks5-proxy-client)
3. [Programmatic Construction with Wrap](#3-programmatic-construction-with-wrap)
4. [Encrypted LeaseSet (WithLeaseSetKey)](#4-encrypted-leaseset-withleasesetkey)
5. [Prometheus Metrics (WithMetricsAddr)](#5-prometheus-metrics-withmetricsaddr)
6. [Manual Lifecycle — Start / Stop without Run](#6-manual-lifecycle--start--stop-without-run)
7. [Running Multiple Tunnels](#7-running-multiple-tunnels)
8. [Hot Reload with SIGHUP](#8-hot-reload-with-sighup)

---

## 1. HTTP Proxy Client

An HTTP proxy client lets applications (and browsers) browse I2P sites by connecting to a local HTTP proxy port.

### Minimal — from a config file

```go
package main

import (
    "log"

    "github.com/go-i2p/go-i2ptunnel/lib/embedding"
)

func main() {
    t, err := embedding.FromConfigFile("httpclient.yaml")
    if err != nil {
        log.Fatal(err)
    }
    // Run blocks until SIGINT or SIGTERM. SIGHUP triggers a config reload.
    log.Fatal(t.Run())
}
```

`httpclient.yaml` (see [examples/httpclient.yaml](../examples/httpclient.yaml)):

```yaml
tunnels:
  http-proxy:
    name: "http-proxy"
    type: "httpclient"
    port: 4444
    interface: "127.0.0.1"
```

Set your browser's HTTP proxy to `127.0.0.1:4444` and you can browse `.i2p` sites.

### With options — custom SAM address and metrics

```go
package main

import (
    "log"

    "github.com/go-i2p/go-i2ptunnel/lib/embedding"
)

func main() {
    t, err := embedding.FromConfigFile("httpclient.yaml",
        embedding.WithSAMAddr("192.168.1.10:7656"),   // non-default SAM bridge
        embedding.WithMetricsAddr(":9090"),            // expose /metrics, /healthz
    )
    if err != nil {
        log.Fatal(err)
    }
    log.Fatal(t.Run())
}
```

### With an outproxy for clearnet access

Add the outproxy to the config file:

```yaml
tunnels:
  http-proxy:
    name: "http-proxy"
    type: "httpclient"
    port: 4444
    interface: "127.0.0.1"
    options:
      outproxy: "exit.stormycloud.i2p"
```

> **Security note:** The outproxy operator can see your clearnet traffic destination. They cannot see your IP address because you are connecting via I2P, but choose a trustworthy outproxy.

---

## 2. SOCKS5 Proxy Client

A SOCKS5 client proxy works with any application that supports SOCKS5, including SSH, curl, and most browsers.

### Minimal — from a config file

```go
package main

import (
    "log"

    "github.com/go-i2p/go-i2ptunnel/lib/embedding"
)

func main() {
    t, err := embedding.FromConfigFile("socksclient.yaml")
    if err != nil {
        log.Fatal(err)
    }
    log.Fatal(t.Run())
}
```

`socksclient.yaml` (see [examples/socksclient.yaml](../examples/socksclient.yaml)):

```yaml
tunnels:
  socks-proxy:
    name: "socks-proxy"
    type: "socksclient"
    port: 4447
    interface: "127.0.0.1"
```

Configure your application's SOCKS5 proxy to `127.0.0.1:4447`.

### With rate limiting

```yaml
tunnels:
  socks-proxy:
    name: "socks-proxy"
    type: "socksclient"
    port: 4447
    interface: "127.0.0.1"
    options:
      maxconns: 50
      ratelimit: 20.0
```

### Example: SSH over I2P via the SOCKS5 proxy

Once the proxy is running:

```bash
ssh -o ProxyCommand="nc -x 127.0.0.1:4447 %h %p" user@example.b32.i2p
```

---

## 3. Programmatic Construction with Wrap

Use `Wrap` when you build the tunnel implementation directly in code rather than loading it from a YAML file. This is useful when your application already has configuration management and you do not want a separate config file.

```go
package main

import (
    "log"

    i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
    httpclient "github.com/go-i2p/go-i2ptunnel/lib/http/client"
    "github.com/go-i2p/go-i2ptunnel/lib/embedding"
)

func main() {
    const samAddr = "127.0.0.1:7656"

    cfg := i2pconv.TunnelConfig{
        Name:      "http-proxy",
        Type:      "httpclient",
        Interface: "127.0.0.1",
        Port:      4444,
    }

    rawTunnel, err := httpclient.NewHTTPClient(cfg, samAddr)
    if err != nil {
        log.Fatal(err)
    }

    t, err := embedding.Wrap(rawTunnel,
        embedding.WithSAMAddr(samAddr),
        embedding.WithMetricsAddr(":9090"),
    )
    if err != nil {
        log.Fatal(err)
    }

    log.Fatal(t.Run())
}
```

The same pattern works for the SOCKS5 client:

```go
package main

import (
    "log"

    i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
    socks "github.com/go-i2p/go-i2ptunnel/lib/socks/client"
    "github.com/go-i2p/go-i2ptunnel/lib/embedding"
)

func main() {
    const samAddr = "127.0.0.1:7656"

    cfg := i2pconv.TunnelConfig{
        Name:      "socks-proxy",
        Type:      "socksclient",
        Interface: "127.0.0.1",
        Port:      4447,
        Tunnel: map[string]interface{}{
            "maxconns":  100,
            "ratelimit": 50.0,
        },
    }

    rawTunnel, err := socks.NewSocksClient(cfg, samAddr)
    if err != nil {
        log.Fatal(err)
    }

    t, err := embedding.Wrap(rawTunnel)
    if err != nil {
        log.Fatal(err)
    }

    log.Fatal(t.Run())
}
```

> **Note:** `Wrap`-based tunnels do not support hot reload via `SIGHUP` because no config file path is known. `SIGHUP` will stop the tunnel but not restart it. Use `FromConfigFile` if you need hot reload.

---

## 4. Encrypted LeaseSet (WithLeaseSetKey)

Encrypted LeaseSets add an extra layer of privacy by hiding your tunnel's destination from the I2P network database. The key is a Base64-encoded private key passed at startup.

```go
package main

import (
    "log"
    "os"

    "github.com/go-i2p/go-i2ptunnel/lib/embedding"
)

func main() {
    // Load the private key from an environment variable or secrets manager.
    leaseSetKey := os.Getenv("I2P_LEASESET_PRIVKEY")
    if leaseSetKey == "" {
        log.Fatal("I2P_LEASESET_PRIVKEY is required")
    }

    t, err := embedding.FromConfigFile("httpclient.yaml",
        embedding.WithLeaseSetKey(leaseSetKey),
    )
    if err != nil {
        log.Fatal(err)
    }
    log.Fatal(t.Run())
}
```

Corresponding config with encrypted LeaseSet options:

```yaml
tunnels:
  http-proxy:
    name: "http-proxy"
    type: "httpclient"
    port: 4444
    interface: "127.0.0.1"
    i2cp:
      leaseSetType: "3"
      leaseSetEncType: "4,0"
```

`WithLeaseSetKey` injects the `i2cp.leaseSetPrivKey` option before the tunnel is started. The key is never written to disk by the embedding package.

---

## 5. Prometheus Metrics (WithMetricsAddr)

`WithMetricsAddr` starts an HTTP server in the background exposing three endpoints:

| Endpoint | Description |
|---|---|
| `/metrics` | Prometheus text format |
| `/healthz` | `200 OK` when tunnel is healthy |
| `/api/status` | JSON tunnel status |

```go
package main

import (
    "log"

    "github.com/go-i2p/go-i2ptunnel/lib/embedding"
)

func main() {
    t, err := embedding.FromConfigFile("socksclient.yaml",
        embedding.WithMetricsAddr(":9090"),
    )
    if err != nil {
        log.Fatal(err)
    }
    log.Fatal(t.Run())
}
```

Available Prometheus metrics include connection counts, byte counters, error counts, rate-limit hits, and uptime. Scrape with Prometheus or check health in a container orchestrator:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: "i2p-socks-proxy"
    static_configs:
      - targets: ["localhost:9090"]
```

```bash
# Kubernetes liveness probe equivalent
curl -f http://localhost:9090/healthz
```

---

## 6. Manual Lifecycle — Start / Stop without Run

Use `Start` and `Stop` directly when you need to integrate the tunnel into an existing lifecycle (e.g., a daemon that manages its own signal handling or a test harness).

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/go-i2p/go-i2ptunnel/lib/embedding"
)

func main() {
    t, err := embedding.FromConfigFile("httpclient.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // Start runs the tunnel in a background goroutine from the caller's
    // perspective; Start itself blocks until the tunnel exits, so launch it
    // in a goroutine.
    errCh := make(chan error, 1)
    go func() { errCh <- t.Start() }()

    addr, _ := t.LocalAddress()
    log.Printf("HTTP proxy listening on %s", addr)

    // Wait for a signal or a tunnel error.
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

    select {
    case sig := <-sigCh:
        log.Printf("received %s, stopping", sig)
        if err := t.Stop(); err != nil {
            log.Printf("stop error: %v", err)
        }
    case err := <-errCh:
        log.Fatalf("tunnel error: %v", err)
    }
}
```

> Prefer `t.Run()` for the common case — it implements the same pattern shown above, plus SIGHUP reload, with less boilerplate.

---

## 7. Running Multiple Tunnels

Run an HTTP proxy and a SOCKS5 proxy side by side — each in its own goroutine with its own error channel.

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/go-i2p/go-i2ptunnel/lib/embedding"
)

func main() {
    http, err := embedding.FromConfigFile("httpclient.yaml",
        embedding.WithMetricsAddr(":9091"),
    )
    if err != nil {
        log.Fatalf("http tunnel: %v", err)
    }

    socks, err := embedding.FromConfigFile("socksclient.yaml",
        embedding.WithMetricsAddr(":9092"),
    )
    if err != nil {
        log.Fatalf("socks tunnel: %v", err)
    }

    httpErr := make(chan error, 1)
    socksErr := make(chan error, 1)

    go func() { httpErr <- http.Run() }()
    go func() { socksErr <- socks.Run() }()

    // Exit when either tunnel exits or returns an error.
    select {
    case err := <-httpErr:
        if err != nil {
            log.Printf("http tunnel exited with error: %v", err)
        }
        socks.Stop()
    case err := <-socksErr:
        if err != nil {
            log.Printf("socks tunnel exited with error: %v", err)
        }
        http.Stop()
    }
}
```

Each call to `Run()` registers its own signal listeners, so both tunnels respond to SIGINT/SIGTERM independently.

---

## 8. Hot Reload with SIGHUP

When a tunnel is created via `FromConfigFile`, sending `SIGHUP` to the process triggers a zero-downtime reload:

1. The running tunnel is stopped.
2. `LoadConfig` re-reads the YAML/TOML file in place.
3. If `LoadConfig` fails, the tunnel is recreated from the file via `loader.Load`.
4. The tunnel restarts with the new configuration.

```bash
# Edit the config while the proxy is running, then reload without restarting
vim httpclient.yaml
kill -HUP $(pidof my-i2p-app)
```

`Run()` continues to handle further signals after a successful reload. If the reload fails (e.g., the new config is invalid), the error is logged and the previous tunnel instance keeps running.

To reload programmatically:

```go
if err := t.Reload(); err != nil {
    log.Printf("reload failed: %v", err)
} else {
    // Restart the tunnel after Reload stops it
    go func() { log.Println(t.Start()) }()
}
```

> **Note:** `Wrap`-based tunnels (built without a config file path) treat `SIGHUP` as a stop with no restart. If hot reload is required, use `FromConfigFile`.

---

## Summary

| Use case | Constructor | Signal handling |
|---|---|---|
| Config file, blocking | `FromConfigFile` + `Run()` | SIGINT/SIGTERM stop, SIGHUP reloads |
| Config file, manual | `FromConfigFile` + `Start()`/`Stop()` | Caller manages signals |
| Programmatic, blocking | `Wrap` + `Run()` | SIGINT/SIGTERM stop, SIGHUP stops only |
| Programmatic, manual | `Wrap` + `Start()`/`Stop()` | Caller manages signals |

For the full API reference see the [Go package documentation](https://pkg.go.dev/github.com/go-i2p/go-i2ptunnel/lib/embedding) or the inline godoc in [lib/embedding/embedding.go](../lib/embedding/embedding.go).
