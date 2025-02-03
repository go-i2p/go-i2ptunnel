SOCKS5 Tunnels
=============

# SOCKS5 I2P Proxy

This project implements a SOCKS5 proxy server that enables secure and anonymous communication through the I2P network. By connecting to this proxy, applications can transparently route their network traffic via I2P's encrypted tunnels.

Key capabilities:
- Routes TCP and UDP traffic through I2P
- Implements standard SOCKS5 protocol
- Provides transparent proxy functionality
- Enables applications to use I2P without modification

## Traffic Flow

The I2P SOCKS5 proxy acts as a bridge between regular applications and the I2P network. Here's how traffic flows through the system:

```
graph LR
    A[Client App] -->|SOCKS5| B[SOCKS5 Proxy]
    B -->|I2P Protocol| C[I2P Network]
    C -->|I2P Protocol| D[Destination]
```

When a client connects to this SOCKS5 proxy:
1. The application sends SOCKS5 connection requests
2. The proxy converts these requests to I2P protocol
3. Traffic is routed anonymously through the I2P network
4. The destination receives the packets via I2P

## Features

- Full SOCKS5 protocol support (RFC 1928)
- Both TCP and UDP forwarding
- Anonymous routing through I2P
- Standard SOCKS5 authentication methods