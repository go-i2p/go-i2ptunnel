# Deployment Guide

Production deployment guide for go-i2ptunnel on Linux systems.

## Prerequisites

- **I2P router** running with SAM bridge enabled (default: `127.0.0.1:7656`)
- **Go 1.24+** for building from source
- **Linux** with systemd (Debian, Ubuntu, Fedora, Arch, etc.)

## Building

```bash
git clone https://github.com/go-i2p/go-i2ptunnel.git
cd go-i2ptunnel
go build ./cmd/...
```

Install binaries to a system path:

```bash
sudo install -m 755 cmd/http/client/client /usr/local/bin/i2ptunnel-http-client
sudo install -m 755 cmd/http/server/server /usr/local/bin/i2ptunnel-http-server
sudo install -m 755 cmd/tcp/client/client /usr/local/bin/i2ptunnel-tcp-client
sudo install -m 755 cmd/tcp/server/server /usr/local/bin/i2ptunnel-tcp-server
sudo install -m 755 cmd/irc/client/client /usr/local/bin/i2ptunnel-irc-client
sudo install -m 755 cmd/irc/server/server /usr/local/bin/i2ptunnel-irc-server
sudo install -m 755 cmd/socks/client/client /usr/local/bin/i2ptunnel-socks-client
sudo install -m 755 cmd/udp/client/client /usr/local/bin/i2ptunnel-udp-client
sudo install -m 755 cmd/udp/server/server /usr/local/bin/i2ptunnel-udp-server
sudo install -m 755 cmd/web/web /usr/local/bin/i2ptunnel-webui
```

Or build specific binaries:

```bash
go build -o i2ptunnel-http-proxy ./cmd/http/client
go build -o i2ptunnel-webui ./cmd/web
```

## Directory Layout

Recommended production directory structure:

```
/etc/i2ptunnel/                    # Configuration files
├── http-proxy.yaml                # HTTP proxy config
├── web-server.yaml                # Web server config
├── irc-client.yaml                # IRC client config
└── ...

/var/lib/i2ptunnel/                # Runtime data (keys, state)

/var/log/i2ptunnel/                # Log files (if using file logging)

/usr/local/bin/                    # Binaries
├── i2ptunnel-http-client
├── i2ptunnel-http-server
├── i2ptunnel-tcp-client
├── i2ptunnel-tcp-server
├── i2ptunnel-webui
└── ...
```

Create the directories:

```bash
sudo mkdir -p /etc/i2ptunnel /var/lib/i2ptunnel /var/log/i2ptunnel
sudo chown i2ptunnel:i2ptunnel /etc/i2ptunnel /var/lib/i2ptunnel /var/log/i2ptunnel
```

## System User

Create a dedicated system user:

```bash
sudo useradd -r -s /usr/sbin/nologin -d /var/lib/i2ptunnel i2ptunnel
```

## Systemd Service Units

### Single Tunnel Service

Create `/etc/systemd/system/i2ptunnel@.service` — a template unit that runs
any tunnel type:

```ini
[Unit]
Description=I2P Tunnel (%i)
Documentation=https://github.com/go-i2p/go-i2ptunnel
After=network.target i2p.service
Wants=i2p.service

[Service]
Type=simple
User=i2ptunnel
Group=i2ptunnel
ExecStart=/usr/local/bin/i2ptunnel-%i -config /etc/i2ptunnel/%i.yaml
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=10

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/var/lib/i2ptunnel
PrivateTmp=yes
ProtectKernelTunables=yes
ProtectControlGroups=yes

# Environment
Environment=DEBUG_I2P=warn
WorkingDirectory=/var/lib/i2ptunnel

[Install]
WantedBy=multi-user.target
```

Usage:

```bash
# Start an HTTP proxy tunnel
sudo systemctl start i2ptunnel@http-client

# Enable auto-start on boot
sudo systemctl enable i2ptunnel@http-client

# Reload configuration without restart (SIGHUP)
sudo systemctl reload i2ptunnel@http-client

# View logs
sudo journalctl -u i2ptunnel@http-client -f
```

The template matches the binary name to the config file:
- `i2ptunnel@http-client` runs `/usr/local/bin/i2ptunnel-http-client` with
  `/etc/i2ptunnel/http-client.yaml`

### Web UI Service

Create `/etc/systemd/system/i2ptunnel-webui.service`:

```ini
[Unit]
Description=I2P Tunnel Web Management UI
Documentation=https://github.com/go-i2p/go-i2ptunnel
After=network.target i2p.service
Wants=i2p.service

[Service]
Type=simple
User=i2ptunnel
Group=i2ptunnel
ExecStart=/usr/local/bin/i2ptunnel-webui -config /etc/i2ptunnel -host 127.0.0.1 -port 8089
Restart=on-failure
RestartSec=10

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/var/lib/i2ptunnel /etc/i2ptunnel
PrivateTmp=yes
ProtectKernelTunables=yes
ProtectControlGroups=yes

Environment=DEBUG_I2P=warn
WorkingDirectory=/var/lib/i2ptunnel

[Install]
WantedBy=multi-user.target
```

Usage:

```bash
sudo systemctl start i2ptunnel-webui
sudo systemctl enable i2ptunnel-webui
```

Access the UI at `http://127.0.0.1:8089`.

> **Security note**: The web UI has no authentication. Bind only to
> `127.0.0.1` and use SSH tunneling or a reverse proxy with authentication
> for remote access.

## Configuration Management

### Creating Tunnel Configs

Place YAML config files in `/etc/i2ptunnel/`:

```bash
# HTTP proxy for browsing
cat > /etc/i2ptunnel/http-client.yaml << 'EOF'
tunnels:
  http-proxy:
    name: "http-proxy"
    type: "httpclient"
    port: 4444
    interface: "127.0.0.1"
EOF

# Web server
cat > /etc/i2ptunnel/http-server.yaml << 'EOF'
tunnels:
  my-site:
    name: "my-site"
    type: "httpserver"
    target: "localhost:8080"
    interface: "127.0.0.1"
    i2cp:
      leaseSetType: "3"
      leaseSetEncType: "4,0"
EOF
```

### Hot Reload

All tunnel binaries support `SIGHUP` for configuration reload without
downtime:

```bash
# Via systemd
sudo systemctl reload i2ptunnel@http-server

# Manually
kill -HUP $(pidof i2ptunnel-http-server)
```

The reload cycle:
1. Stop the running tunnel
2. Reload configuration from the file
3. Start the tunnel with new settings

If reload fails, the tunnel falls back to creating a fresh instance from the
config file.

## Logging

### Log Levels

Control logging via the `DEBUG_I2P` environment variable:

| Value | Output |
|---|---|
| `debug` | Verbose debug information |
| `warn` | Warnings and errors |
| `error` | Errors only |
| *(empty/unset)* | No logging (production default) |

### Systemd Journal

With systemd, logs go to the journal automatically:

```bash
# Follow logs for a specific tunnel
journalctl -u i2ptunnel@http-client -f

# Show last 100 lines
journalctl -u i2ptunnel@http-client -n 100

# Show logs since boot
journalctl -u i2ptunnel@http-client -b

# Filter by priority
journalctl -u i2ptunnel@http-client -p warning
```

### File Logging

To log to files, redirect output in the systemd unit:

```ini
[Service]
StandardOutput=append:/var/log/i2ptunnel/http-client.log
StandardError=append:/var/log/i2ptunnel/http-client-error.log
```

Add logrotate configuration at `/etc/logrotate.d/i2ptunnel`:

```
/var/log/i2ptunnel/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0640 i2ptunnel i2ptunnel
}
```

## Security Recommendations

### Network Binding

Always bind tunnel listeners to `127.0.0.1` unless you have a specific
reason to expose them on other interfaces:

```yaml
interface: "127.0.0.1"
```

### Encrypted LeaseSets

For server tunnels hosting sensitive services, enable encrypted LeaseSets:

```yaml
i2cp:
  leaseSetType: "3"
  leaseSetEncType: "4,0"
  leaseSetAuthType: "2"
  leaseSetPrivKey: "your-base64-key"
```

This hides your LeaseSet from unauthorized peers, adding a layer of
anonymity protection.

### Rate Limiting

Protect server tunnels from abuse with connection limits:

```yaml
maxconns: 100
ratelimit: 20.0
```

### File Permissions

```bash
# Config files readable only by service user
chmod 600 /etc/i2ptunnel/*.yaml
chown i2ptunnel:i2ptunnel /etc/i2ptunnel/*.yaml

# Binaries owned by root, not writable by service user
chown root:root /usr/local/bin/i2ptunnel-*
chmod 755 /usr/local/bin/i2ptunnel-*
```

### Firewall

No inbound ports need to be opened for I2P tunnels. All I2P traffic flows
through the I2P router's existing connections. Only local listening ports
need to be accessible from localhost.

## Troubleshooting

### Tunnel Won't Start

1. **Check I2P router is running** with SAM enabled:
   ```bash
   nc -z 127.0.0.1 7656 && echo "SAM is up" || echo "SAM is down"
   ```

2. **Enable debug logging**:
   ```bash
   DEBUG_I2P=debug i2ptunnel-http-client -config /etc/i2ptunnel/http-client.yaml
   ```

3. **Validate the config file**:
   - Ensure `name` and `type` are present
   - Ensure `target` is valid for client/server tunnels
   - Ensure `port` is in range and not already in use

### Port Already in Use

```bash
# Find what's using the port
ss -tlnp | grep :4444
```

Change the `port` in your config file or stop the conflicting service.

### SAM Connection Refused

The I2P router must have SAM enabled:
- **Java I2P**: Enable SAM in Router Console → I2P Internals → Clients
- **i2pd**: Set `sam.enabled=true` in `i2pd.conf`

Default SAM port is 7656. If your router uses a different port, pass
`-sam host:port` to the tunnel binary.

### Configuration Reload Fails

If `SIGHUP` reload fails, the tunnel continues with the previous
configuration. Check the logs for error details:

```bash
journalctl -u i2ptunnel@http-server -n 20
```

Common causes: invalid YAML syntax, wrong tunnel type in config, invalid
port or address.

### High Memory Usage

Each tunnel creates a SAM session with I2P. Running many tunnels increases
memory use. Consider:
- Using bidirectional tunnels to share keys between server and client
- Running only the tunnels you need
- Setting reasonable `maxconns` limits

## Multi-Tunnel Deployment

For running many tunnels, the web UI is recommended. Place all configs in a
single directory:

```
/etc/i2ptunnel/
├── http-proxy.yaml
├── web-server.yaml
├── irc-client.yaml
└── socks-proxy.yaml
```

Start the web UI pointing to this directory:

```bash
sudo systemctl start i2ptunnel-webui
```

The web UI will load all configs and let you start/stop/configure each
tunnel individually from `http://127.0.0.1:8089`.

## Next Steps

- **[Quick Start Guide](QUICKSTART.md)** — Get running quickly
- **[Configuration Reference](CONFIGURATION.md)** — All options for every tunnel type
