package ircserver

/**
IRC Server
----------

The IRC Server implements a reverse proxy that enables IRC servers hosted on the local machine to be accessible from the I2P network. It provides:

- Secure traffic forwarding between local IRC services and I2P clients
- Access control and connection management
- Command filtering and security policies
- Bandwidth and resource monitoring
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
	"github.com/go-i2p/onramp"
)

var implementIRCServer i2ptunnel.I2PTunnel = &IRCServer{}

// IRCServer is an IRC server tunnel that accepts connections from the I2P network
// and forwards them to a local IRC daemon. It applies rate-limiting and ircinspector
// filtering to protect the local service from abuse.
type IRCServer struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The local IRC service address
	net.Addr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// The rate-limiting configuration
	limitedlistener.LimitedConfig
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
	// Mutex protecting the I2PTunnelStatus field from concurrent read/write access
	statusMu sync.RWMutex
	// ErrorTracker provides bounded error history.
	i2ptunnel.ErrorTracker
	// Metrics tracks live operational data for this tunnel.
	// Set by the webui controller after construction. May be nil.
	Metrics *metrics.TunnelMetrics
}

// SetTunnelMetrics injects a live metrics tracker. Implements metrics.MetricsBearer.
func (i *IRCServer) SetTunnelMetrics(m *metrics.TunnelMetrics) {
	i.Metrics = m
}

func (t *IRCServer) recordError(err error) {
	t.ErrorTracker.Record(t, err)
	if t.Metrics != nil {
		t.Metrics.RecordError()
	}
}

func (i *IRCServer) setStatus(s i2ptunnel.I2PTunnelStatus) {
	i.statusMu.Lock()
	i.I2PTunnelStatus = s
	i.statusMu.Unlock()
}

// Get the tunnel's I2P address
func (i *IRCServer) Address() string {
	// For IRC server, return the service address if available
	if i.Garlic != nil && i.Garlic.ServiceKeys != nil {
		return i.Garlic.ServiceKeys.Addr().Base32()
	}
	return ""
}

// Get the tunnel's error message
func (i *IRCServer) Error() error {
	return i.ErrorTracker.Last()
}

// Get the tunnel's local host:port
func (i *IRCServer) LocalAddress() (string, error) {
	addr := net.JoinHostPort(i.TunnelConfig.Interface, strconv.Itoa(i.TunnelConfig.Port))
	return addr, nil
}

// Get the tunnel's name
func (i *IRCServer) Name() string {
	return i.TunnelConfig.Name
}

// maxConsecutiveErrors is the number of consecutive Accept() failures before
// the tunnel transitions to I2PTunnelStatusFailed.
const maxConsecutiveErrors = 10

// Start the tunnel.
// Each incoming I2P connection is forwarded to the local IRC service in a separate goroutine.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (i *IRCServer) Start() error {
	i.lifeMu.Lock()
	i.done = make(chan struct{})
	i.stopOnce = sync.Once{}
	i.setStatus(i2ptunnel.I2PTunnelStatusStarting)
	i2pListener, err := i.Garlic.ListenStream()
	if err != nil {
		i.lifeMu.Unlock()
		return err
	}
	i.listener = i2pListener
	i.lifeMu.Unlock()
	defer i2pListener.Close()
	defer i.Stop()
	i.setStatus(i2ptunnel.I2PTunnelStatusRunning)
	if i.Metrics != nil {
		i.Metrics.RecordStart()
	}
	limitedI2PListener := limitedlistener.NewLimitedListener(i2pListener, limitedlistener.WithMaxConnections(i.LimitedConfig.MaxConns), limitedlistener.WithRateLimit(i.LimitedConfig.RateLimit))
	ircInspectorListener := ircinspector.New(limitedI2PListener, DefaultIRCServerConfigWithMetrics(i.Metrics))
	ApplyIRCServerFilterRules(ircInspectorListener, i.Address(), i.Metrics)
	consecutiveErrors := 0
	for {
		select {
		case <-i.done:
			return nil
		default:
			con, err := ircInspectorListener.Accept()
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
					i.recordError(fmt.Errorf("accept error (%d consecutive): %w", consecutiveErrors, err))
					if consecutiveErrors >= maxConsecutiveErrors {
						i.setStatus(i2ptunnel.I2PTunnelStatusFailed)
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

// handleConnection forwards a single I2P connection to the local IRC service.
// Both connections are closed when forwarding completes.
func (i *IRCServer) handleConnection(con net.Conn) {
	if i.Metrics != nil {
		i.Metrics.RecordConnection()
	}
	wrapped := metrics.WrapConn(con, i.Metrics)
	defer wrapped.Close()
	lCon, err := net.Dial("tcp", i.Target())
	if err != nil {
		i.recordError(err)
		if i.Metrics != nil {
			i.Metrics.RecordConnectionFailed()
		}
		return
	}
	defer lCon.Close()
	ctx := context.Background()
	stream.Forward(ctx, wrapped, lCon, config.DefaultConfig())
}

// Get the tunnel's status
func (i *IRCServer) Status() i2ptunnel.I2PTunnelStatus {
	i.statusMu.RLock()
	defer i.statusMu.RUnlock()
	return i.I2PTunnelStatus
}

// Stop the tunnel. Safe to call multiple times.
// Closes the Garlic (I2P SAM session) to release network resources.
func (i *IRCServer) Stop() error {
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
		i.setStatus(i2ptunnel.I2PTunnelStatusStopped)
		if i.Metrics != nil {
			i.Metrics.RecordStop()
		}
	})
	return nil
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (i *IRCServer) Target() string {
	return i.Addr.String()
}

// Get the tunnel's type
func (i *IRCServer) Type() string {
	return i.TunnelConfig.Type
}

// Get the tunnel's ID
func (i *IRCServer) ID() string {
	return i2ptunnel.Clean(i.Name())
}

// Get the tunnel's options
func (i *IRCServer) Options() map[string]string {
	options := i2ptunnel.BuildCommonOptions(i.TunnelConfig)
	i2ptunnel.AddRateLimitOptions(options, i.LimitedConfig.MaxConns, i.LimitedConfig.RateLimit)
	if i.Addr != nil {
		options["target"] = i.Addr.String()
	}
	return options
}

// Set the tunnel's options
func (i *IRCServer) SetOptions(opts map[string]string) error {
	if err := i2ptunnel.ApplyCommonOptions(opts, &i.TunnelConfig); err != nil {
		return err
	}
	if err := i2ptunnel.ApplyRateLimitOptions(opts, &i.LimitedConfig.MaxConns, &i.LimitedConfig.RateLimit); err != nil {
		return err
	}
	if target, ok := opts["target"]; ok {
		if err := validate.NetworkAddress(target); err != nil {
			return err
		}
		addr, err := net.ResolveTCPAddr("tcp", target)
		if err != nil {
			return fmt.Errorf("invalid target address %q: %w", target, err)
		}
		i.Addr = addr
	}
	return nil
}

// LoadConfig loads tunnel configuration from a file and updates the tunnel settings.
// The tunnel must be stopped before calling LoadConfig to prevent inconsistent state.
// Supported formats: .properties, .ini, .yaml/.yml
func (i *IRCServer) LoadConfig(path string) error {
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
	if newConfig.Type != "ircserver" {
		return fmt.Errorf("config file contains %s tunnel, expected ircserver", newConfig.Type)
	}

	// Validate target address (local service) before applying changes
	targetAddr, err := net.ResolveTCPAddr("tcp", newConfig.Target)
	if err != nil {
		return fmt.Errorf("invalid target address in config: %w", err)
	}

	// Update mutable configuration fields
	i.TunnelConfig = *newConfig
	i.Addr = targetAddr

	return nil
}
