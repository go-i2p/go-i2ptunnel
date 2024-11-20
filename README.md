I2PTunnel for Go
================

Implementation of I2PTunnel for go which includes equivalents of all I2PTunnel functionality.
Implements middleware in libraries to handle filtering, rate-limiting, and encrypted leaseSets.

I2PTunnel Services
------------------

 - [] Standard Server
 - [] HTTP Server
 - [] IRC Server
 - [] UDP Server(Not in I2PTunnel Java)

I2PTunnel Client
----------------

 - [] Standard Server
 - [] HTTP Proxy Client
 - [] SOCKS Proxy Client
 - [] SOCKS IRC Client
 - [] UDP Client(Not in I2PTunnel Java)
 - [] TUN Device(Not in I2PTunnel Java, may require root)

Omitted:
--------

 - [] SOCKS4* Client

### Structure

```md
# Libraries:

lib/http/client # high level http.Client implementation, filtering middleware, encrypted leaseSet middleware
lib/http/server # high level http.Server implementation
lib/http/server/listener # high level net.Listener implementation, filtering middleware
lib/irc/client # high level net.Conn implementation, filtering middleware, ??encrypted leaseSet middleware??
lib/irc/socksclient # high level net.Dialer implementation, filtering middleware, ??encrypted leaseSet middleware??
lib/irc/server # high level net.Listener implementation, filtering middleware
lib/socks/client # high level net.Conn implementation, filtering middleware, ??encrypted leaseSet middleware??
lib/standard/client # high level net.Dialer implementation, filtering middleware
lib/standard/server # high level net.Listener implementation, filtering middleware
lib/udp/client # high level net.Conn implementation, filtering middleware
lib/udp/server # high level net.Listener implementation, filtering middleware
lib/tun/client # high level songgao.Water implementation using SOCKS5 backend, configuration tooling

# Commands

cmd/http/client # starts an HTTP proxy listening on a port, forwarding requests to lib/http/client's http.Client
cmd/http/server # starts a client connected to a local port, forwarding requests to and lib/http/service/listener's custom net.Listener
cmd/irc/client # starts a proxy listening on a port, forwarding requests to lib/irc/client's custom net.Conn implementation
cmd/irc/socksclient # starts a SOCKS proxy listening on a port forwarding requests to lib/irc/socksclient's custom net.Dialer
cmd/irc/server # starts a client connected to a port, forwarding requests to lib/irc/server's custom net.Listener
cmd/socks/client # starts a SOCKS5 proxy lisening on a port forwarding requests to lib/irc/server's custom net.Dialer
cmd/standard/client # starts a proxy listening on a port forwarding requests to lib/standard/client's custom net.Conn
cmd/standard/server # start a client connected to a local port forwarding requests to lib/standard/server's custom net.Listener
cmd/udp/client # start a "quasi-server" which forwards datagrams to and from lib/udp/client's custom net.Conn
cmd/udp/server # start a "quasi-client" which forwards datagrams to and from lib/udp/server's custom net.Listener
```

### Usage

```Go
TODO: example usage from shell
```


```Go
TODO: example usage from Go
```