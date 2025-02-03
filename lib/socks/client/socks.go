package socks

/**
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
**/

import (
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/onramp"
)

var implementSOCKS i2ptunnel.I2PTunnel = &SOCKS{}

type SOCKS struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// Channel for shutdown signaling
	done chan struct{}
}

// Get the tunnel's I2P address
func (s *SOCKS) Address() string {
	panic("unimplemented")
}

// Get the tunnel's error message
func (s *SOCKS) Error() error {
	panic("unimplemented")
}

// Get the tunnel's local host:port
func (s *SOCKS) LocalAddress() (string, string, error) {
	panic("unimplemented")
}

// Get the tunnel's name
func (s *SOCKS) Name() string {
	panic("unimplemented")
}

// Get the tunnel's options
func (s *SOCKS) Options() map[string]string {
	panic("unimplemented")
}

// Start the tunnel
func (s *SOCKS) Start() error {
	panic("unimplemented")
}

// Get the tunnel's status
func (s *SOCKS) Status() i2ptunnel.I2PTunnelStatus {
	panic("unimplemented")
}

// Stop the tunnel
func (s *SOCKS) Stop() error {
	panic("unimplemented")
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (s *SOCKS) Target() string {
	panic("unimplemented")
}

// Get the tunnel's type
func (s *SOCKS) Type() string {
	panic("unimplemented")
}
