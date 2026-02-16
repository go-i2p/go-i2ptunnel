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
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/core/validate"
	"github.com/go-i2p/i2pkeys"
	"github.com/go-i2p/onramp"
)

var implementIRCClient i2ptunnel.I2PTunnel = &IRCClient{}

type IRCClient struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The remote I2P destination target
	*i2pkeys.I2PAddr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// The IRC filtering configuration
	ircinspector.Config
	// Channel for shutdown signaling
	done chan struct{}
	// Ensures Stop() is only executed once to prevent double-close panic
	stopOnce sync.Once
	// Mutex protecting the Errors slice from concurrent access
	errMu sync.Mutex

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}

func (t *IRCClient) recordError(err error) {
	t.errMu.Lock()
	t.Errors = append(t.Errors, i2ptunnel.NewError(t, err))
	t.errMu.Unlock()
}

// Get the tunnel's I2P address
func (i *IRCClient) Address() string {
	// For IRC client, return the service address if available
	if i.Garlic != nil && i.Garlic.ServiceKeys != nil {
		return i.Garlic.ServiceKeys.Addr().Base32()
	}
	return ""
}

// Get the tunnel's error message
func (i *IRCClient) Error() error {
	i.errMu.Lock()
	defer i.errMu.Unlock()
	if len(i.Errors) > 0 {
		return i.Errors[len(i.Errors)-1]
	}
	return nil
}

// Get the tunnel's local host:port
func (i *IRCClient) LocalAddress() (string, error) {
	addr := net.JoinHostPort(i.TunnelConfig.Interface, strconv.Itoa(i.TunnelConfig.Port))
	return addr, nil
}

// Get the tunnel's name
func (i *IRCClient) Name() string {
	return i.TunnelConfig.Name
}

// Start the tunnel.
// Each accepted local connection gets its own I2P stream to the target destination.
// Connections are handled concurrently in separate goroutines.
// Safe to call after Stop() — done channel and stopOnce are reset for restartability.
func (i *IRCClient) Start() error {
	i.done = make(chan struct{})
	i.stopOnce = sync.Once{}
	i.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStarting
	listener, err := net.Listen("tcp", net.JoinHostPort(i.Interface, strconv.Itoa(i.Port)))
	if err != nil {
		return err
	}
	defer listener.Close()
	defer i.Stop()
	filteredListener := ircinspector.New(listener, i.Config)
	ApplyIRCClientFilterRules(filteredListener, i.Address())
	defer filteredListener.Close()
	i.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusRunning
	for {
		select {
		case <-i.done:
			return nil
		default:
			con, err := filteredListener.Accept()
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

// handleConnection forwards a single local connection over its own I2P stream.
// Both connections are closed when forwarding completes.
func (i *IRCClient) handleConnection(con net.Conn) {
	defer con.Close()
	i2pConn, err := i.Garlic.Dial("tcp", i.Target())
	if err != nil {
		i.recordError(err)
		return
	}
	defer i2pConn.Close()
	ctx := context.Background()
	stream.Forward(ctx, con, i2pConn, config.DefaultConfig())
}

// Get the tunnel's status
func (i *IRCClient) Status() i2ptunnel.I2PTunnelStatus {
	return i.I2PTunnelStatus
}

// Stop the tunnel. Safe to call multiple times.
// Closes the Garlic (I2P SAM session) to release network resources.
func (i *IRCClient) Stop() error {
	i.stopOnce.Do(func() {
		close(i.done)
		if i.Garlic != nil {
			i.Garlic.Close()
		}
	})
	i.I2PTunnelStatus = i2ptunnel.I2PTunnelStatusStopped
	return nil
}

// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
func (i *IRCClient) Target() string {
	return i.I2PAddr.Base32()
}

// Get the tunnel's type
func (i *IRCClient) Type() string {
	return i.TunnelConfig.Type
}

// Get the tunnel's ID
func (i *IRCClient) ID() string {
	return i2ptunnel.Clean(i.Name())
}

// Get the tunnel's options
func (i *IRCClient) Options() map[string]string {
	// Return basic configuration options as a map
	options := make(map[string]string)
	options["name"] = i.TunnelConfig.Name
	options["type"] = i.TunnelConfig.Type
	options["interface"] = i.TunnelConfig.Interface
	options["port"] = strconv.Itoa(i.TunnelConfig.Port)
	if i.I2PAddr != nil {
		options["target"] = i.I2PAddr.Base32()
	}
	return options
}

// Set the tunnel's options
func (i *IRCClient) SetOptions(opts map[string]string) error {
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
	if i.I2PTunnelStatus == i2ptunnel.I2PTunnelStatusRunning ||
		i.I2PTunnelStatus == i2ptunnel.I2PTunnelStatusStarting {
		return fmt.Errorf("cannot load config while tunnel is %s - stop tunnel first", i.I2PTunnelStatus)
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
