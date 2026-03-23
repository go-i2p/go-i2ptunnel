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
	"os"
	"strconv"
	"sync"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
	limitedlistener "github.com/go-i2p/go-limit"
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
	// The rate-limiting configuration.
	// Note: socks5.Server manages its own listener; MaxConns/RateLimit are
	// persisted here for configuration round-trips and future wiring.
	limitedlistener.LimitedConfig
	// SOCKS5 server instance
	*socks5.Server
	// Channel for shutdown signaling
	done chan struct{}
	// Ensures Stop() is only executed once to prevent double-close panic
	stopOnce sync.Once
	// Mutex for server operations
	mu sync.Mutex
	// Mutex protecting the I2PTunnelStatus field from concurrent read/write access
	statusMu sync.RWMutex
	// ErrorTracker provides bounded error history.
	i2ptunnel.ErrorTracker
	// Metrics tracks live operational data for this tunnel.
	// Set by the webui controller after construction. May be nil.
	Metrics *metrics.TunnelMetrics
	// Context for cleanup
	ctx    context.Context
	cancel context.CancelFunc
}

// SetTunnelMetrics injects a live metrics tracker. Implements metrics.MetricsBearer.
func (s *SOCKS) SetTunnelMetrics(m *metrics.TunnelMetrics) {
	s.Metrics = m
}

func (s *SOCKS) recordError(err error) {
	s.ErrorTracker.Record(s, err)
	if s.Metrics != nil {
		s.Metrics.RecordError()
	}
}

func (s *SOCKS) setStatus(s2 i2ptunnel.I2PTunnelStatus) {
	s.statusMu.Lock()
	s.I2PTunnelStatus = s2
	s.statusMu.Unlock()
}

// Get the tunnel's I2P address
func (s *SOCKS) Address() string {
	// For SOCKS proxy, return the service address if available
	if s.Garlic != nil && s.Garlic.ServiceKeys != nil {
		return s.Garlic.ServiceKeys.Addr().Base32()
	}
	return ""
}

// Get the tunnel's error message
func (s *SOCKS) Error() error {
	return s.ErrorTracker.Last()
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
	s.stopOnce = sync.Once{}

	// Create SOCKS5 server
	addr := net.JoinHostPort(s.TunnelConfig.Interface, strconv.Itoa(s.TunnelConfig.Port))
	server, err := socks5.NewClassicServer(addr, "", "", "", 0, 0)
	if err != nil {
		s.recordError(err)
		return fmt.Errorf("failed to create SOCKS5 server: %w", err)
	}

	s.Server = server
	s.setStatus(i2ptunnel.I2PTunnelStatusStarting)
	s.Server.Handle = s
	s.setStatus(i2ptunnel.I2PTunnelStatusRunning)
	if s.Metrics != nil {
		s.Metrics.RecordStart()
	}

	return s.Server.ListenAndServe(s)
}

// Get the tunnel's status
func (s *SOCKS) Status() i2ptunnel.I2PTunnelStatus {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	return s.I2PTunnelStatus
}

// Stop the tunnel. Safe to call multiple times.
// Closes the Garlic (I2P SAM session) to release network resources.
func (s *SOCKS) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Server != nil {
		s.setStatus(i2ptunnel.I2PTunnelStatusStopping)
		s.stopOnce.Do(func() {
			close(s.done)
		})
		if s.cancel != nil {
			s.cancel()
		}
		if err := s.Server.Shutdown(); err != nil {
			s.recordError(err)
			return err
		}
		if s.Garlic != nil {
			s.Garlic.Close()
		}
		s.Server = nil
		s.setStatus(i2ptunnel.I2PTunnelStatusStopped)
		if s.Metrics != nil {
			s.Metrics.RecordStop()
		}
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

// Get the tunnel's ID
func (s *SOCKS) ID() string {
	return i2ptunnel.Clean(s.Name())
}

// Get the tunnel's options
func (s *SOCKS) Options() map[string]string {
	// Return basic configuration options as a map
	options := make(map[string]string)
	options["name"] = s.TunnelConfig.Name
	options["type"] = s.TunnelConfig.Type
	options["interface"] = s.TunnelConfig.Interface
	options["port"] = strconv.Itoa(s.TunnelConfig.Port)
	options["maxconns"] = strconv.Itoa(s.LimitedConfig.MaxConns)
	options["ratelimit"] = strconv.FormatFloat(s.LimitedConfig.RateLimit, 'f', -1, 64)
	i2ptunnel.MergeI2CPOptions(s.TunnelConfig.I2CP, options)
	return options
}

// Set the tunnel's options
func (s *SOCKS) SetOptions(opts map[string]string) error {
	// Apply configuration options from the map with validation
	if name, ok := opts["name"]; ok {
		if err := validate.RequiredString("name", name); err != nil {
			return err
		}
		s.TunnelConfig.Name = name
	}
	if iface, ok := opts["interface"]; ok {
		if err := validate.Interface(iface); err != nil {
			return err
		}
		s.TunnelConfig.Interface = iface
	}
	if portStr, ok := opts["port"]; ok {
		port, err := validate.PortString(portStr)
		if err != nil {
			return err
		}
		s.TunnelConfig.Port = port
	}
	if maxconnsStr, ok := opts["maxconns"]; ok {
		maxconns, err := strconv.Atoi(maxconnsStr)
		if err != nil {
			return fmt.Errorf("invalid maxconns value: %s", maxconnsStr)
		}
		if err := validate.MaxConnections(maxconns); err != nil {
			return err
		}
		s.LimitedConfig.MaxConns = maxconns
	}
	if ratelimitStr, ok := opts["ratelimit"]; ok {
		ratelimit, err := validate.RateLimitString(ratelimitStr)
		if err != nil {
			return err
		}
		s.LimitedConfig.RateLimit = ratelimit
	}
	// Apply I2CP options (encrypted LeaseSet, authentication, etc.)
	if i2cpOpts := i2ptunnel.ExtractI2CPOptions(opts); i2cpOpts != nil {
		if s.TunnelConfig.I2CP == nil {
			s.TunnelConfig.I2CP = make(map[string]interface{})
		}
		for k, v := range i2cpOpts {
			s.TunnelConfig.I2CP[k] = v
		}
	}
	return nil
}

// LoadConfig loads tunnel configuration from a file and updates the tunnel settings.
// The tunnel must be stopped before calling LoadConfig to prevent inconsistent state.
// Supported formats: .properties, .ini, .yaml/.yml
//
// Why: SOCKS proxies need dynamic configuration for production deployments.
// Design: Uses go-i2ptunnel-config library for parsing. Preserves SAM connection and I2P keys.
func (s *SOCKS) LoadConfig(path string) error {
	// Prevent config changes while tunnel is running to avoid race conditions
	status := s.Status()
	if status == i2ptunnel.I2PTunnelStatusRunning ||
		status == i2ptunnel.I2PTunnelStatusStarting {
		return fmt.Errorf("cannot load config while tunnel is %s - stop tunnel first", status)
	}

	// Parse config file using the converter library
	conv := i2pconv.Converter{}
	format, err := conv.DetectFormat(path)
	if err != nil {
		return fmt.Errorf("failed to detect config format: %w", err)
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	newConfig, err := conv.ParseInput(bytes, format)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Type safety: ensure loaded config matches expected tunnel type
	if newConfig.Type != "socks" && newConfig.Type != "socksclient" {
		return fmt.Errorf("config file contains %s tunnel, expected socks or socksclient", newConfig.Type)
	}

	// Update mutable configuration fields
	// The Garlic connection (SAM) is preserved to maintain tunnel identity and keys
	s.TunnelConfig = *newConfig

	return nil
}
