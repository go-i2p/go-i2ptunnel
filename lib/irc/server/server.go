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
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/onramp"
)

var implementIRCServer i2ptunnel.I2PTunnel = &IRCServer{}

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

// Start the tunnel.
// Each incoming I2P connection is forwarded to the local IRC service in a separate goroutine.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (i *IRCServer) Start() error {
	i.lifeMu.Lock()
	i.done = make(chan struct{})
	i.stopOnce = sync.Once{}
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
	ircInspectorListener := ircinspector.New(limitedI2PListener, i.Config)
	ApplyIRCServerFilterRules(ircInspectorListener, i.Address())
	for {
		select {
		case <-i.done:
			return nil
		default:
			con, err := ircInspectorListener.Accept()
			if err != nil {
				select {
				case <-i.done:
					return nil
				default:
				}
				time.Sleep(50 * time.Millisecond)
				continue
			}
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
	// Return basic configuration options as a map
	options := make(map[string]string)
	options["name"] = i.TunnelConfig.Name
	options["type"] = i.TunnelConfig.Type
	options["interface"] = i.TunnelConfig.Interface
	options["port"] = strconv.Itoa(i.TunnelConfig.Port)
	options["maxconns"] = strconv.Itoa(i.LimitedConfig.MaxConns)
	options["ratelimit"] = strconv.FormatFloat(i.LimitedConfig.RateLimit, 'f', -1, 64)
	if i.Addr != nil {
		options["target"] = i.Addr.String()
	}
	i2ptunnel.MergeI2CPOptions(i.TunnelConfig.I2CP, options)
	return options
}

// Set the tunnel's options
func (i *IRCServer) SetOptions(opts map[string]string) error {
	// Apply configuration options from the map with validation
	if name, ok := opts["name"]; ok {
		if err := validate.RequiredString("name", name); err != nil {
			return err
		}
		i.TunnelConfig.Name = name
	}
	if iface, ok := opts["interface"]; ok {
		if err := validate.Interface(iface); err != nil {
			return err
		}
		i.TunnelConfig.Interface = iface
	}
	if portStr, ok := opts["port"]; ok {
		port, err := validate.PortString(portStr)
		if err != nil {
			return err
		}
		i.TunnelConfig.Port = port
	}
	if maxconnsStr, ok := opts["maxconns"]; ok {
		maxconns, err := strconv.Atoi(maxconnsStr)
		if err != nil {
			return fmt.Errorf("invalid maxconns value: %s", maxconnsStr)
		}
		if err := validate.MaxConnections(maxconns); err != nil {
			return err
		}
		i.LimitedConfig.MaxConns = maxconns
	}
	if ratelimitStr, ok := opts["ratelimit"]; ok {
		ratelimit, err := validate.RateLimitString(ratelimitStr)
		if err != nil {
			return err
		}
		i.LimitedConfig.RateLimit = ratelimit
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
	// Apply I2CP options (encrypted LeaseSet, authentication, etc.)
	if i2cpOpts := i2ptunnel.ExtractI2CPOptions(opts); i2cpOpts != nil {
		if i.TunnelConfig.I2CP == nil {
			i.TunnelConfig.I2CP = make(map[string]interface{})
		}
		for k, v := range i2cpOpts {
			i.TunnelConfig.I2CP[k] = v
		}
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
