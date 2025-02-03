HTTP Tunnels
============
The go-i2ptunnel HTTP suite provides bidirectional HTTP tunneling capabilities between the I2P network and local services. It consists of two main components:

1. An HTTP client proxy for connecting local applications to I2P web services
2. An HTTP server proxy for exposing local web services to I2P clients

The tunnels handle protocol negotiation, header management, and encrypted routing through the I2P network while maintaining compatibility with standard HTTP implementations.

HTTP Client
-----------

The HTTP Client implements a proxy server that enables HTTP/S traffic between local applications and I2P network services. It acts as an intermediary, handling all standard HTTP methods and CONNECT requests.

```
[Browser/App] <-> [I2P HTTP Client] <-> [I2P Network] <-> [I2P Services]
    :8118           (HTTP Proxy)         Encrypted        Web Servers
                    |
               - Protocol handling
               - Header management  
               - Connection routing
```

Key features:
- Supports HTTP, HTTPS and CONNECT methods
- Proxies requests between local clients and I2P services
- Manages HTTP headers and connection states
- Handles protocol negotiation and routing

HTTP Server
-----------

The HTTP Server implements a reverse proxy that forwards traffic between local services and I2P clients. It acts as an intermediary, providing access control and traffic management.

```
[Local Service] <-> [I2P HTTP Server] <-> [I2P Network] <-> [I2P Clients]
    :8080            (Reverse Proxy)        Encrypted        Browser/App
                     |
                - Header filtering
                - Rate limiting
                - Access control
```

Key features:
- Forwards requests between local services and I2P network
- Filters and modifies HTTP headers
- Rate limits incoming requests
- Provides access control for I2P clients