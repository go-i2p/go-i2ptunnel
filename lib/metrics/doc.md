# Monitoring and Metrics

The `lib/metrics` package provides observability for I2P tunnel deployments through three HTTP endpoints.

## Endpoints

### `GET /healthz` — Health Check

Returns a JSON response indicating the service is running, with a summary of tunnel counts.

```json
{
  "status": "ok",
  "tunnels_total": 3,
  "tunnels_running": 2
}
```

Always returns HTTP 200 if the web UI is responsive.

### `GET /api/status` — Detailed Status

Returns JSON with full tunnel state and metrics for each registered tunnel.

```json
{
  "tunnels": [
    {
      "name": "my-http-proxy",
      "id": "my-http-proxy",
      "type": "httpclient",
      "status": "running",
      "address": "abcd1234.b32.i2p",
      "local_address": "127.0.0.1:4444",
      "metrics": {
        "connections_accepted": 150,
        "connections_failed": 3,
        "active_connections": 5,
        "bytes_in": 1048576,
        "bytes_out": 524288,
        "error_count": 2,
        "rate_limit_hits": 0,
        "filter_blocks": 12,
        "uptime_seconds": 3600.5
      }
    }
  ]
}
```

### `GET /metrics` — Prometheus Format

Exposes all metrics in [Prometheus text exposition format](https://prometheus.io/docs/instrumenting/exposition_formats/), suitable for scraping by Prometheus or compatible monitoring systems.

```text
# HELP i2ptunnel_connections_accepted_total Total accepted connections
# TYPE i2ptunnel_connections_accepted_total counter
i2ptunnel_connections_accepted_total{name="my-proxy",id="my-proxy",type="httpclient"} 150
# HELP i2ptunnel_active_connections Currently active connections
# TYPE i2ptunnel_active_connections gauge
i2ptunnel_active_connections{name="my-proxy",id="my-proxy",type="httpclient"} 5
# HELP i2ptunnel_uptime_seconds Tunnel uptime in seconds
# TYPE i2ptunnel_uptime_seconds gauge
i2ptunnel_uptime_seconds{name="my-proxy",id="my-proxy",type="httpclient"} 3600.500
```

**Metrics exposed:**

| Metric | Type | Description |
|--------|------|-------------|
| `i2ptunnel_connections_accepted_total` | counter | Total accepted connections |
| `i2ptunnel_connections_failed_total` | counter | Total failed connection attempts |
| `i2ptunnel_bytes_received_total` | counter | Total bytes received |
| `i2ptunnel_bytes_sent_total` | counter | Total bytes sent |
| `i2ptunnel_errors_total` | counter | Total errors encountered |
| `i2ptunnel_rate_limit_hits_total` | counter | Total rate limit rejections |
| `i2ptunnel_filter_blocks_total` | counter | Total filter blocks |
| `i2ptunnel_active_connections` | gauge | Currently active connections |
| `i2ptunnel_uptime_seconds` | gauge | Tunnel uptime in seconds |

All metrics include `name`, `id`, and `type` labels to identify the tunnel.

## Prometheus Scrape Configuration

```yaml
scrape_configs:
  - job_name: 'i2ptunnel'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:8089']
```

## Library Usage

Tunnels can record metrics directly using the `TunnelMetrics` API:

```go
import "github.com/go-i2p/go-i2ptunnel/lib/metrics"

// Register a tunnel in the metrics registry.
m := metrics.DefaultRegistry.Register("my-tunnel", "my-tunnel-id", "tcpclient")

// Record events during tunnel operation.
m.RecordStart()
m.RecordConnection()
m.RecordBytesIn(1024)
m.RecordBytesOut(512)
m.RecordDisconnection()
m.RecordStop()
```

All recording methods are safe for concurrent use from multiple goroutines.
