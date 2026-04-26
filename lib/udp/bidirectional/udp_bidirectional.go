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
	"strconv"
	"sync"
	"time"

	"github.com/go-i2p/go-forward/packet"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
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

// maxConsecutiveErrors is the number of consecutive DialUDP failures before
// the tunnel transitions to I2PTunnelStatusFailed.
const maxConsecutiveErrors = 10

// Start launches both the server-side I2P datagram listener and the client-side
// SOCKS5 proxy concurrently. It blocks until the tunnel is stopped.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (u *UDPBidirectional) Start() error {
	u.lifeMu.Lock()
	u.done = make(chan struct{})
	u.stopOnce = sync.Once{}
	done := u.done // capture local ref before unlock to avoid data race with restart
	u.setStatus(i2ptunnel.I2PTunnelStatusStarting)
	u.lifeMu.Unlock()

	i2pListener, err := u.Garlic.ListenPacket()
	if err != nil {
		return fmt.Errorf("failed to start I2P datagram listener: %w", err)
	}
	defer i2pListener.Close()
	defer u.Stop()

	socksErrCh, err := u.startSOCKS5Proxy()
	if err != nil {
		return err
	}

	u.setStatus(i2ptunnel.I2PTunnelStatusRunning)
	if u.Metrics != nil {
		u.Metrics.RecordStart()
	}

	raddr, err := net.ResolveUDPAddr("udp", u.Target())
	if err != nil {
		return fmt.Errorf("failed to resolve target UDP address: %w", err)
	}

	return u.runForwardLoop(i2pListener, raddr, done, socksErrCh)
}

// runForwardLoop runs the main dial-and-forward cycle for outbound I2P datagrams.
func (u *UDPBidirectional) runForwardLoop(i2pListener net.PacketConn, raddr *net.UDPAddr, done chan struct{}, socksErrCh chan error) error {
	backoff := udpconst.MinBackoff
	consecutiveErrors := 0
	for {
		select {
		case <-done:
			return nil
		case err := <-socksErrCh:
			if cont, fatal := u.handleSOCKSError(err, done, socksErrCh); !cont {
				return fatal
			}
		default:
			cont, fatal := u.runForwardCycle(i2pListener, raddr, done, &consecutiveErrors, &backoff)
			if !cont {
				return fatal
			}
		}
	}
}

// runForwardCycle dials the target and forwards packets for one session.
// Returns (true, nil) to continue the loop, (false, nil) for clean stop, (false, err) for fatal.
func (u *UDPBidirectional) runForwardCycle(i2pListener net.PacketConn, raddr *net.UDPAddr, done chan struct{}, consecutiveErrors *int, backoff *time.Duration) (cont bool, fatal error) {
	lCon, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return u.handleDialError(err, consecutiveErrors, backoff, done)
	}
	*backoff = udpconst.MinBackoff
	*consecutiveErrors = 0
	func() {
		defer lCon.Close()
		ctx := context.Background()
		fwdCfg := udpconst.NewDatagramForwardConfig()
		fwdCfg.ShutdownSignal = done
		packet.Forward(ctx, i2pListener, metrics.WrapPacketConn(lCon, u.Metrics), fwdCfg)
	}()
	select {
	case <-done:
		return false, nil
	case <-time.After(100 * time.Millisecond):
	}
	return true, nil
}

// startSOCKS5Proxy creates and starts the SOCKS5 proxy goroutine.
func (u *UDPBidirectional) startSOCKS5Proxy() (chan error, error) {
	socksAddr := net.JoinHostPort(u.TunnelConfig.Interface, strconv.Itoa(u.TunnelConfig.Port))
	socksServer, err := socks5.NewClassicServer(socksAddr, "", "", "", 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCKS5 server: %w", err)
	}
	u.socksServer = socksServer
	u.socksServer.Handle = &socksHandler{garlic: u.Garlic}
	socksErrCh := make(chan error, 1)
	go func() {
		socksErrCh <- u.socksServer.ListenAndServe(u.socksServer.Handle)
	}()
	return socksErrCh, nil
}

// handleSOCKSError logs transient SOCKS errors and restarts the listener.
// Returns (true, nil) to continue, (false, nil) to stop cleanly.
func (u *UDPBidirectional) handleSOCKSError(err error, done <-chan struct{}, socksErrCh chan error) (cont bool, fatal error) {
	if err != nil {
		u.recordError(fmt.Errorf("SOCKS5 server error (restarting): %w", err))
	}
	select {
	case <-done:
		return false, nil
	default:
	}
	go func() {
		socksErrCh <- u.socksServer.ListenAndServe(u.socksServer.Handle)
	}()
	return true, nil
}

// handleDialError processes a DialUDP failure and returns whether to continue.
func (u *UDPBidirectional) handleDialError(err error, consecutiveErrors *int, backoff *time.Duration, done <-chan struct{}) (cont bool, fatal error) {
	select {
	case <-done:
		return false, nil
	default:
	}
	*consecutiveErrors++
	u.recordError(fmt.Errorf("dial error (%d consecutive): %w", *consecutiveErrors, err))
	if *consecutiveErrors >= maxConsecutiveErrors {
		u.setStatus(i2ptunnel.I2PTunnelStatusFailed)
		return false, fmt.Errorf("tunnel failed after %d consecutive dial errors", *consecutiveErrors)
	}
	time.Sleep(*backoff)
	*backoff = udpconst.NextBackoff(*backoff)
	return true, nil
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
// Note: the "max-conns" and "rate-limit" keys are included for config
// round-trip compatibility only — they are not enforced at runtime because
// UDP tunnels operate on net.PacketConn (datagrams), not net.Listener.
func (u *UDPBidirectional) Options() map[string]string {
	options := i2ptunnel.BuildCommonOptions(u.TunnelConfig)
	i2ptunnel.AddRateLimitOptions(options, u.LimitedConfig.MaxConns, u.LimitedConfig.RateLimit)
	if u.Addr != nil {
		options["target"] = u.Addr.String()
	}
	return options
}

// SetOptions applies configuration options from a string map with validation.
// Note: the "max-conns" and "rate-limit" keys are stored for config round-trip
// compatibility but are not enforced — see Options() for details.
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
	if err := i2ptunnel.CheckTunnelStopped(u.Status()); err != nil {
		return err
	}
	newConfig, err := i2ptunnel.ParseConfigFile(path)
	if err != nil {
		return err
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
