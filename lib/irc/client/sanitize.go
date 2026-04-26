package ircclient

import (
	"fmt"
	"net"
	"strings"

	ircinspector "github.com/go-i2p/go-connfilter/irc"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
)

// DefaultIRCClientConfig returns an ircinspector.Config pre-configured with
// privacy-protecting defaults for IRC client tunnels. It blocks DCC commands
// that could expose real IP addresses.
func DefaultIRCClientConfig() ircinspector.Config {
	return ircinspector.Config{
		OnMessage: func(msg *ircinspector.Message) error {
			// Block DCC commands - direct client connections expose real IP
			if strings.EqualFold(msg.Command, "DCC") {
				return fmt.Errorf("DCC commands are not allowed over I2P")
			}

			// Check for DCC in CTCP messages (inside PRIVMSG trailing).
			// Call filterDCCRequest first to surface a precise error when the
			// DCC parameters target a private IP or use a forbidden port.
			if strings.EqualFold(msg.Command, "PRIVMSG") && strings.Contains(msg.Trailing, "\x01DCC") {
				if idx := strings.Index(msg.Trailing, "\x01DCC "); idx >= 0 {
					if err := filterDCCRequest(msg.Trailing[idx+1:]); err != nil {
						return err
					}
				}
				return fmt.Errorf("DCC CTCP commands are not allowed over I2P")
			}

			return nil
		},
	}
}

// ApplyIRCClientFilterRules adds DCC parameter validation and hostname-masking
// filter rules to an existing IRC inspector. Call this after
// ircinspector.New() to add per-command callbacks that block dangerous DCC
// parameters and replace real hostnames with the I2P address.
// m may be nil; when non-nil, RecordFilterBlock is incremented on each DCC block.
func ApplyIRCClientFilterRules(inspector *ircinspector.Inspector, i2pHost string, m *metrics.TunnelMetrics) {
	addDCCFilter(inspector, m)
	addPingFilter(inspector, i2pHost)
	addUserhostFilter(inspector, i2pHost)
}

// addDCCFilter validates DCC parameters and blocks all DCC commands.
func addDCCFilter(inspector *ircinspector.Inspector, m *metrics.TunnelMetrics) {
	inspector.AddFilter(ircinspector.Filter{
		Command: "DCC",
		Callback: func(msg *ircinspector.Message) error {
			body := "DCC " + strings.Join(msg.Params, " ")
			if err := filterDCCRequest(body); err != nil {
				if m != nil {
					m.RecordFilterBlock()
				}
				return err
			}
			if m != nil {
				m.RecordFilterBlock()
			}
			return fmt.Errorf("DCC commands are not permitted over I2P")
		},
	})
}

// addPingFilter replaces real hostnames with the I2P address in PING responses.
func addPingFilter(inspector *ircinspector.Inspector, i2pHost string) {
	inspector.AddFilter(ircinspector.Filter{
		Command: "PING",
		Callback: func(msg *ircinspector.Message) error {
			if i2pHost != "" && len(msg.Params) > 0 {
				msg.Params[0] = i2pHost
			}
			return nil
		},
	})
}

// addUserhostFilter masks user@host to prevent identity leakage.
func addUserhostFilter(inspector *ircinspector.Inspector, i2pHost string) {
	inspector.AddFilter(ircinspector.Filter{
		Command: "USERHOST",
		Callback: func(msg *ircinspector.Message) error {
			if i2pHost != "" && msg.Trailing != "" {
				parts := strings.Split(msg.Trailing, "@")
				if len(parts) > 1 {
					msg.Trailing = parts[0] + "@" + i2pHost
				}
			}
			return nil
		},
	})
}

// ApplyIRCClientFilters wraps a listener with IRC client-side filtering.
// Blocks dangerous commands like DCC and filters potentially leaky content.
func ApplyIRCClientFilters(listener net.Listener, i2pHost string) net.Listener {
	inspector := ircinspector.New(listener, DefaultIRCClientConfig())
	ApplyIRCClientFilterRules(inspector, i2pHost, nil)
	return inspector
}
