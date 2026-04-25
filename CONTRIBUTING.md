Contributing
============

Project Structure
-----------------

```md
# Libraries:

lib/http/client # high level http.Client implementation, filtering middleware, HTTP outproxy client, encrypted leaseSet middleware
lib/http/server # high level http.Server implementation
lib/http/server/listener # high level net.Listener implementation, filtering middleware
lib/irc/client # high level net.Conn implementation, filtering middleware, ??encrypted leaseSet middleware??
lib/irc/server # high level net.Listener implementation, filtering middleware
lib/socks/client # high level net.Conn implementation, filtering middleware, SOCKS outproxy client, ??encrypted leaseSet middleware??
lib/standard/client # high level net.Dialer implementation, filtering middleware
lib/standard/server # high level net.Listener implementation, filtering middleware
lib/udp/client # high level net.Conn implementation, filtering middleware
lib/udp/server # high level net.Listener implementation, filtering middleware

# Commands

cmd/http/client # starts an HTTP proxy listening on a port, forwarding requests to lib/http/client's http.Client
cmd/http/server # starts a client connected to a local port, forwarding requests to and lib/http/service/listener's custom net.Listener
cmd/irc/client # starts a proxy listening on a port, forwarding requests to lib/irc/client's custom net.Conn implementation
cmd/irc/server # starts a client connected to a port, forwarding requests to lib/irc/server's custom net.Listener
cmd/socks/client # starts a SOCKS5 proxy lisening on a port forwarding requests to lib/irc/server's custom net.Dialer
cmd/standard/client # starts a proxy listening on a port forwarding requests to lib/standard/client's custom net.Conn
cmd/standard/server # start a client connected to a local port forwarding requests to lib/standard/server's custom net.Listener
cmd/udp/client # start a "quasi-server" which forwards datagrams to and from lib/udp/client's custom net.Conn
cmd/udp/server # start a "quasi-client" which forwards datagrams to and from lib/udp/server's custom net.Listener

# VPN/Network Interface Support
# For TUN device and WireGuard-based VPN tunneling, see: github.com/go-i2p/wireguard
```

Design Decisions
----------------

### IRC Filtering — intentional divergence from Java I2P

The Java I2P IRC tunnel (`I2PTunnelIRCClient.java` / `I2PTunnelIRCServer.java`)
supports configurable command allow-lists and per-message CTCP flood protection.
This implementation deliberately uses a simpler, fixed **deny-list** approach:

- **DCC blocking** (client + server): DCC SEND / DCC CHAT target real IP
  addresses and cannot work over I2P. All DCC commands and CTCP DCC messages are
  unconditionally rejected. IP parameters are also validated to catch private-IP
  leaks before rejection.
- **Administrative command blocking** (server): ADMIN, OPER, DIE, RESTART,
  REHASH, and KILL are refused when received via the I2P-facing listener to
  prevent remote abuse of server administrative interfaces.
- **Hostname masking** (client + server): PING, USERHOST, JOIN, and WHOIS
  responses that carry `user@host` strings have the host component replaced with
  the I2P destination address.
- **Configurable command allow-lists** — **intentionally omitted**: Java's
  allow-list mechanism requires per-deployment configuration that is out-of-scope
  for a generic tunnel library. Operators can extend filtering by subclassing the
  `ircinspector.Config` callbacks.
- **CTCP flood protection** — **intentionally omitted**: Per-message rate
  limiting at the command level duplicates the connection-level rate limiting
  provided by `go-limit/limitedlistener`. Adding a second rate-limit mechanism
  for individual IRC messages would add complexity without clear benefit for the
  majority of deployments.

### HTTP Filtering — intentional divergence from Java I2P

The Java I2P HTTP client tunnel (`I2PTunnelHTTPClient.java`) supports outproxy
host lists and per-destination block lists in addition to header sanitization.

- **Identity-leaking header stripping** (client + server): `X-Forwarded-For`,
  `X-Real-IP`, `Via`, `Forwarded`, and related headers are removed from all
  requests. The server side also strips `Referer` for cross-origin requests.
- **Server fingerprinting header removal** (server): `Server`, `X-Powered-By`,
  `X-AspNet-Version`, and similar headers are stripped from responses.
- **Privacy response headers** (server): `X-Frame-Options: SAMEORIGIN` and
  `X-Content-Type-Options: nosniff` are added when not already present.
- **User-Agent normalization** (client): The `User-Agent` header is replaced
  with a fixed generic string when absent to reduce fingerprinting surface.
- **Outproxy host lists** — **intentionally omitted**: Routing `.clearnet`
  requests through a configurable list of clearnet outproxies requires proxy
  chaining logic that is beyond the scope of this library's tunnel abstraction.
  Applications that need outproxy routing should chain proxies at the application
  layer.
- **Per-destination block lists** — **intentionally omitted**: Blocking specific
  I2P destinations requires DNS-level resolution at the filter layer. This
  feature is out-of-scope; operators needing destination filtering should use
  firewall rules or a higher-level policy engine.

### Rate-Limiting for UDP and SOCKS5

`go-limit/limitedlistener` requires a `net.Listener` (stream-oriented accept
loop). UDP tunnels use `net.PacketConn` (datagram-oriented) and the SOCKS5
`txthinking/socks5` server manages its own internal listener. Both carry a
`LimitedConfig` field for configuration round-trips, but stream-based rate
limiting via `limitedlistener` is not wired for these tunnel types. IRC client
tunnels, which use a plain `net.Listen` accept loop, do apply
`limitedlistener.NewLimitedListener` in `Start()`.