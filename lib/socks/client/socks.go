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
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/onramp"
	"github.com/txthinking/socks5"
)

var implementSOCKS i2ptunnel.I2PTunnel = &SOCKS{}

type SOCKS struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// SOCKS5 server instance
	*socks5.Server
	// Channel for shutdown signaling
	done chan struct{}
	// Mutex for server operations
	mu sync.Mutex
	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
	// Context for cleanup
	ctx    context.Context
	cancel context.CancelFunc
}

func (s *SOCKS) recordError(err error) {
	s.Errors = append(s.Errors, i2ptunnel.NewError(s, err))
}

// Get the tunnel's I2P address
func (s *SOCKS) Address() string {
	return s.Garlic.B32()
}

// Get the tunnel's error message
func (s *SOCKS) Error() error {
	if len(s.Errors) > 0 {
		return s.Errors[len(s.Errors)-1]
	}
	return nil
}

// Get the tunnel's local host:port
func (s *SOCKS) LocalAddress() (string, error) {
	addr := net.JoinHostPort(s.TunnelConfig.Interface, strconv.Itoa(s.TunnelConfig.Port))
	return addr, nil
}

// Get the tunnel's name
func (s *SOCKS) Name() string {
	return s.TunnelConfig.Name
}

// Get the tunnel's options
func (s *SOCKS) Options() map[string]string {
	return s.TunnelConfig.Options()
}

// Start the tunnel
func (s *SOCKS) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Server != nil {
		return nil // Already started
	}

	// Create context for managing goroutines
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.done = make(chan struct{})

	// Create SOCKS5 server
	addr := net.JoinHostPort(s.TunnelConfig.Interface, strconv.Itoa(s.TunnelConfig.Port))
	server, err := socks5.NewClassicServer(addr, "", "", "", 0, 0)
	if err != nil {
		s.recordError(err)
		return fmt.Errorf("failed to create SOCKS5 server: %w", err)
	}

	s.Server = server
	s.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStarting
	s.Server.Handle = s
	s.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning

	return s.Server.ListenAndServe(s)
}

// Get the tunnel's status
func (s *SOCKS) Status() i2ptunnel.I2PTunnelStatus {
	return s.I2PTunnelStatus
}

// Stop the tunnel
func (s *SOCKS) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Server != nil {
		s.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopping
		close(s.done)
		s.cancel()
		if err := s.Server.Shutdown(); err != nil {
			s.recordError(err)
			return err
		}
		s.Server = nil
		s.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped
	}
	return nil
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (s *SOCKS) Target() string {
	return ""
}

// Get the tunnel's type
func (s *SOCKS) Type() string {
	return s.TunnelConfig.Type
}
