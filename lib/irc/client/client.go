// Package ircclient implements an IRC client tunnel that proxies local IRC
// connections over I2P with DCC filtering and CTCP sanitization.
package ircclient

/**
IRC Client
----------

The IRC Client implements a SOCKS-compatible proxy that enables local IRC clients to connect to services on the I2P network. It provides:

- Transparent proxying between local IRC clients and I2P servers
- Command filtering for enhanced security
- Connection management and automatic reconnection
- Bandwidth usage monitoring
**/

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	ircinspector "github.com/go-i2p/go-connfilter/irc"
	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/stream"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/i2pkeys"
	"github.com/go-i2p/onramp"
)

var implementIRCClient i2ptunnel.I2PTunnel = &IRCClient{}

// IRCClient is an IRC client tunnel that listens locally and forwards connections to
// an IRC server reachable over I2P. It applies ircinspector filtering (DCC blocking,
// admin-command blocking, hostname masking) to protect local IRC clients.
type IRCClient struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// TunnelBase provides Name, ID, Type, Status, Error, SetTunnelMetrics, SetStatus, RecordError, and the common Options/SetOptions keys.
	i2ptunnel.TunnelBase
	// The remote I2P destination target
	*i2pkeys.I2PAddr
	// The IRC filtering configuration
	ircinspector.Config
	// Channel for shutdown signaling
	done chan struct{}
	// Ensures Stop() is only executed once to prevent double-close panic
	stopOnce sync.Once
	// Listener reference for clean shutdown — closing unblocks Accept()
	listener net.Listener
	// Mutex protecting lifecycle fields (done, stopOnce, listener) during Start/Stop transitions.
	// Prevents the race where Start() resets stopOnce while Stop() is calling stopOnce.Do().
	lifeMu sync.Mutex
}

// Get the tunnel's I2P address
func (i *IRCClient) Address() string {
	// For IRC client, return the service address if available
	if i.Garlic != nil && i.Garlic.ServiceKeys != nil {
		return i.Garlic.ServiceKeys.Addr().Base32()
	}
	return ""
}

// Get the tunnel's local host:port
func (i *IRCClient) LocalAddress() (string, error) {
	addr := net.JoinHostPort(i.TunnelConfig.Interface, strconv.Itoa(i.TunnelConfig.Port))
	return addr, nil
}

// maxConsecutiveErrors is the number of consecutive Accept() failures before
// the tunnel transitions to I2PTunnelStatusFailed.
const maxConsecutiveErrors = 10

// Start the tunnel.
// Each accepted local connection gets its own I2P stream to the target destination.
// Connections are handled concurrently in separate goroutines.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (i *IRCClient) Start() error {
	i.lifeMu.Lock()
	i.done = make(chan struct{})
	i.stopOnce = sync.Once{}
	i.SetStatus(i2ptunnel.I2PTunnelStatusStarting)
	listener, err := net.Listen("tcp", net.JoinHostPort(i.Interface, strconv.Itoa(i.Port)))
	if err != nil {
		i.lifeMu.Unlock()
		return err
	}
	i.listener = listener
	i.lifeMu.Unlock()
	defer listener.Close()
	defer i.Stop()
	limitedL := limitedlistener.NewLimitedListener(listener, limitedlistener.WithMaxConnections(i.LimitedConfig.MaxConns), limitedlistener.WithRateLimit(i.LimitedConfig.RateLimit))
	filteredListener := ircinspector.New(limitedL, i.Config)
	ApplyIRCClientFilterRules(filteredListener, i.Address(), i.Metrics)
	defer filteredListener.Close()
	i.SetStatus(i2ptunnel.I2PTunnelStatusRunning)
	if i.Metrics != nil {
		i.Metrics.RecordStart()
	}
	consecutiveErrors := 0
	for {
		select {
		case <-i.done:
			return nil
		default:
			con, err := filteredListener.Accept()
			if err != nil {
				if (err == limitedlistener.ErrMaxConnsReached || err == limitedlistener.ErrRateLimitExceeded) && i.Metrics != nil {
					i.Metrics.RecordRateLimitHit()
				}
				select {
				case <-i.done:
					return nil
				default:
				}
				if err != limitedlistener.ErrMaxConnsReached && err != limitedlistener.ErrRateLimitExceeded {
					consecutiveErrors++
					i.RecordError(fmt.Errorf("accept error (%d consecutive): %w", consecutiveErrors, err))
					if consecutiveErrors >= maxConsecutiveErrors {
						i.SetStatus(i2ptunnel.I2PTunnelStatusFailed)
						return fmt.Errorf("listener failed after %d consecutive accept errors", consecutiveErrors)
					}
				}
				time.Sleep(50 * time.Millisecond)
				continue
			}
			consecutiveErrors = 0
			go i.handleConnection(con)
		}
	}
}

// handleConnection forwards a single local connection over its own I2P stream.
// Both connections are closed when forwarding completes.
func (i *IRCClient) handleConnection(con net.Conn) {
	if i.Metrics != nil {
		i.Metrics.RecordConnection()
	}
	wrapped := metrics.WrapConn(con, i.Metrics)
	defer wrapped.Close()
	i2pConn, err := i.Garlic.Dial("tcp", i.Target())
	if err != nil {
		i.RecordError(err)
		if i.Metrics != nil {
			i.Metrics.RecordConnectionFailed()
		}
		return
	}
	defer i2pConn.Close()
	ctx := context.Background()
	stream.Forward(ctx, wrapped, i2pConn, config.DefaultConfig())
}

// Stop the tunnel. Safe to call multiple times.
// Closes the Garlic (I2P SAM session) to release network resources.
func (i *IRCClient) Stop() error {
	i.lifeMu.Lock()
	defer i.lifeMu.Unlock()
	i.stopOnce.Do(func() {
		close(i.done)
		if i.listener != nil {
			i.listener.Close()
		}
		if i.Garlic != nil {
			i.Garlic.Close()
		}
		i.SetStatus(i2ptunnel.I2PTunnelStatusStopped)
		if i.Metrics != nil {
			i.Metrics.RecordStop()
		}
	})
	return nil
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (i *IRCClient) Target() string {
	return i.I2PAddr.Base32()
}

// Get the tunnel's options
func (i *IRCClient) Options() map[string]string {
	options := i.TunnelBase.Options()
	if i.I2PAddr != nil {
		options["target"] = i.I2PAddr.Base32()
	}
	return options
}

// Set the tunnel's options
func (i *IRCClient) SetOptions(opts map[string]string) error {
	if err := i.TunnelBase.SetOptions(opts); err != nil {
		return err
	}
	if target, ok := opts["target"]; ok {
		if err := validate.I2PAddress(target); err != nil {
			return err
		}
		addr, err := i2pkeys.Lookup(target)
		if err != nil {
			return fmt.Errorf("invalid target address: %w", err)
		}
		i.I2PAddr = addr
	}
	return nil
}

// LoadConfig loads tunnel configuration from a file and updates the tunnel settings.
// The tunnel must be stopped before calling LoadConfig to prevent inconsistent state.
// Supported formats: .properties, .ini, .yaml/.yml
func (i *IRCClient) LoadConfig(path string) error {
	// Prevent config changes while tunnel is running to avoid race conditions
	status := i.Status()
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
	if newConfig.Type != "ircclient" {
		return fmt.Errorf("config file contains %s tunnel, expected ircclient", newConfig.Type)
	}

	// Validate target address before applying changes
	addr, err := i2pkeys.Lookup(newConfig.Target)
	if err != nil {
		return fmt.Errorf("invalid target address in config: %w", err)
	}

	// Update mutable configuration fields
	i.TunnelConfig = *newConfig
	i.I2PAddr = addr

	return nil
}
