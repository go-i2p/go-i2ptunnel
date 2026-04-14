// Package udpbidirectional implements a UDP bidirectional tunnel that
// combines a UDP server tunnel with a SOCKS5 proxy on the same I2P keys,
// enabling both inbound and outbound I2P datagram exchange.
package udpbidirectional

// UDP Bidirectional Tunnel
//
// A UDP bidirectional tunnel combines:
// 1. A UDP server tunnel (forwarding incoming I2P datagrams to a local service)
// 2. A SOCKS5 proxy (allowing local apps to reach arbitrary I2P destinations)
//
// Both sides share the same I2P identity (keys). This is a non-standard mode
// described in the README as using onramp "hybrid2" mode.

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/packet"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
	udpconst "github.com/go-i2p/go-i2ptunnel/lib/udp/const"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/onramp"
	"github.com/txthinking/socks5"
)

var implementUDPBidirectional i2ptunnel.I2PTunnel = &UDPBidirectional{}

// UDPBidirectional combines a UDP server tunnel with a SOCKS5 proxy client
// on the same I2P keys, enabling both inbound and outbound I2P connections.
type UDPBidirectional struct {
	// I2P connection (shared for both server and client sides)
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The local UDP service address (forward target for inbound I2P datagrams)
	net.Addr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// The rate-limiting configuration.
	// Note: UDP uses net.PacketConn (datagrams), not net.Listener; MaxConns/RateLimit
	// are persisted here for configuration round-trips.
	limitedlistener.LimitedConfig
	// SOCKS5 server instance for outbound connections
	socksServer *socks5.Server
	// Channel for shutdown signaling
	done chan struct{}
	// Ensures Stop() is only executed once to prevent double-close panic
	stopOnce sync.Once
	// Mutex protecting lifecycle fields (done, stopOnce) during Start/Stop transitions.
	// Prevents the race where Start() resets stopOnce while Stop() is calling stopOnce.Do().
	lifeMu sync.Mutex
	// Mutex protecting the I2PTunnelStatus field from concurrent read/write access
	statusMu sync.RWMutex
	// ErrorTracker provides bounded error history.
	i2ptunnel.ErrorTracker
	// Metrics tracks live operational data for this tunnel.
	// Set by the webui controller after construction. May be nil.
	Metrics *metrics.TunnelMetrics
}

// SetTunnelMetrics injects a live metrics tracker. Implements metrics.MetricsBearer.
func (u *UDPBidirectional) SetTunnelMetrics(m *metrics.TunnelMetrics) {
	u.Metrics = m
}

func (u *UDPBidirectional) recordError(err error) {
	u.ErrorTracker.Record(u, err)
	if u.Metrics != nil {
		u.Metrics.RecordError()
	}
}

func (u *UDPBidirectional) setStatus(s i2ptunnel.I2PTunnelStatus) {
	u.statusMu.Lock()
	u.I2PTunnelStatus = s
	u.statusMu.Unlock()
}

// Address returns the tunnel's I2P address.
func (u *UDPBidirectional) Address() string {
	if u.Garlic != nil && u.Garlic.ServiceKeys != nil {
		return u.Garlic.ServiceKeys.Addr().Base32()
	}
	return ""
}

// Error returns the most recent error, or nil.
func (u *UDPBidirectional) Error() error {
	return u.ErrorTracker.Last()
}

// LocalAddress returns the SOCKS5 proxy listen address.
func (u *UDPBidirectional) LocalAddress() (string, error) {
	addr := net.JoinHostPort(u.TunnelConfig.Interface, strconv.Itoa(u.TunnelConfig.Port))
	return addr, nil
}

// Name returns the tunnel's configured name.
func (u *UDPBidirectional) Name() string {
	return u.TunnelConfig.Name
}

// Start launches both the server-side I2P datagram listener and the client-side
// SOCKS5 proxy concurrently. It blocks until the tunnel is stopped.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (u *UDPBidirectional) Start() error {
	u.lifeMu.Lock()
	u.done = make(chan struct{})
	u.stopOnce = sync.Once{}
	u.setStatus(i2ptunnel.I2PTunnelStatusStarting)
	u.lifeMu.Unlock()

	// Start the server side: listen for I2P datagrams
	i2pListener, err := u.Garlic.ListenPacket()
	if err != nil {
		return fmt.Errorf("failed to start I2P datagram listener: %w", err)
	}
	defer i2pListener.Close()
	defer u.Stop()

	// Create SOCKS5 proxy for outbound connections
	socksAddr := net.JoinHostPort(u.TunnelConfig.Interface, strconv.Itoa(u.TunnelConfig.Port))
	socksServer, err := socks5.NewClassicServer(socksAddr, "", "", "", 0, 0)
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 server: %w", err)
	}
	u.socksServer = socksServer
	u.socksServer.Handle = &socksHandler{garlic: u.Garlic}

	// Start SOCKS5 proxy in a background goroutine
	socksErrCh := make(chan error, 1)
	go func() {
		socksErrCh <- u.socksServer.ListenAndServe(u.socksServer.Handle)
	}()

	u.setStatus(i2ptunnel.I2PTunnelStatusRunning)
	if u.Metrics != nil {
		u.Metrics.RecordStart()
	}

	// Resolve target address once before entering the loop
	raddr, err := net.ResolveUDPAddr("udp", u.Target())
	if err != nil {
		return fmt.Errorf("failed to resolve target UDP address: %w", err)
	}

	// Server-side datagram forwarding loop
	backoff := udpconst.MinBackoff
	for {
		select {
		case <-u.done:
			return nil
		case err := <-socksErrCh:
			// Log transient SOCKS errors and restart the listener instead
			// of tearing down the entire bidirectional tunnel.
			if err != nil {
				u.recordError(fmt.Errorf("SOCKS5 server error (restarting): %w", err))
			}
			select {
			case <-u.done:
				return nil
			default:
			}
			go func() {
				socksErrCh <- u.socksServer.ListenAndServe(u.socksServer.Handle)
			}()
		default:
			lCon, err := net.DialUDP("udp", nil, raddr)
			if err != nil {
				select {
				case <-u.done:
					return nil
				default:
				}
				time.Sleep(backoff)
				backoff = udpconst.NextBackoff(backoff)
				continue
			}
			backoff = udpconst.MinBackoff // reset on success
			func() {
				defer lCon.Close()
				ctx := context.Background()
				packet.Forward(ctx, i2pListener, lCon, config.DefaultConfig())
			}()
			// Brief pause between forwarding attempts to prevent rapid socket
			// churn when packet.Forward returns quickly (e.g., on error or timeout).
			select {
			case <-u.done:
				return nil
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
}

// Status returns the current tunnel status.
func (u *UDPBidirectional) Status() i2ptunnel.I2PTunnelStatus {
	u.statusMu.RLock()
	defer u.statusMu.RUnlock()
	return u.I2PTunnelStatus
}

// Stop gracefully shuts down both the server and SOCKS5 proxy sides.
// Safe to call multiple times.
// Closes the Garlic (I2P SAM session) to release network resources.
func (u *UDPBidirectional) Stop() error {
	u.lifeMu.Lock()
	defer u.lifeMu.Unlock()
	u.stopOnce.Do(func() {
		close(u.done)
		if u.socksServer != nil {
			u.socksServer.Shutdown()
		}
		if u.Garlic != nil {
			u.Garlic.Close()
		}
		u.setStatus(i2ptunnel.I2PTunnelStatusStopped)
		if u.Metrics != nil {
			u.Metrics.RecordStop()
		}
	})
	return nil
}

// Target returns the local service address for inbound I2P forwarding.
func (u *UDPBidirectional) Target() string {
	return u.Addr.String()
}

// Type returns the tunnel type identifier.
func (u *UDPBidirectional) Type() string {
	return u.TunnelConfig.Type
}

// ID returns a clean identifier derived from the tunnel name.
func (u *UDPBidirectional) ID() string {
	return i2ptunnel.Clean(u.Name())
}

// Options returns the tunnel's configuration as a string map.
func (u *UDPBidirectional) Options() map[string]string {
	options := i2ptunnel.BuildCommonOptions(u.TunnelConfig)
	i2ptunnel.AddRateLimitOptions(options, u.LimitedConfig.MaxConns, u.LimitedConfig.RateLimit)
	if u.Addr != nil {
		options["target"] = u.Addr.String()
	}
	return options
}

// SetOptions applies configuration options from a string map with validation.
func (u *UDPBidirectional) SetOptions(opts map[string]string) error {
	if err := i2ptunnel.ApplyCommonOptions(opts, &u.TunnelConfig); err != nil {
		return err
	}
	if err := i2ptunnel.ApplyRateLimitOptions(opts, &u.LimitedConfig.MaxConns, &u.LimitedConfig.RateLimit); err != nil {
		return err
	}
	if target, ok := opts["target"]; ok {
		if err := validate.NetworkAddress(target); err != nil {
			return err
		}
		addr, err := net.ResolveUDPAddr("udp", target)
		if err != nil {
			return fmt.Errorf("invalid target address %q: %w", target, err)
		}
		u.Addr = addr
	}
	return nil
}

// LoadConfig loads tunnel configuration from a file. The tunnel must be stopped first.
func (u *UDPBidirectional) LoadConfig(path string) error {
	status := u.Status()
	if status == i2ptunnel.I2PTunnelStatusRunning ||
		status == i2ptunnel.I2PTunnelStatusStarting {
		return fmt.Errorf("cannot load config while tunnel is %s - stop tunnel first", status)
	}

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

	if newConfig.Type != "udpbidirectional" {
		return fmt.Errorf("config file contains %s tunnel, expected udpbidirectional", newConfig.Type)
	}

	targetAddr, err := net.ResolveUDPAddr("udp", newConfig.Target)
	if err != nil {
		return fmt.Errorf("invalid target address in config: %w", err)
	}

	u.TunnelConfig = *newConfig
	u.Addr = targetAddr
	return nil
}
