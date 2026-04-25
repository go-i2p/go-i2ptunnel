package ircserver

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	ircinspector "github.com/go-i2p/go-connfilter/irc"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
)

// Pattern to match user@host format
var userHostPattern = regexp.MustCompile(`@[^\s]+`)

// DefaultIRCServerConfig returns an ircinspector.Config pre-configured with
// privacy-protecting defaults for IRC server tunnels. It blocks DCC commands
// and dangerous administrative commands.
func DefaultIRCServerConfig() ircinspector.Config {
	return DefaultIRCServerConfigWithMetrics(nil)
}

// DefaultIRCServerConfigWithMetrics returns an ircinspector.Config that blocks
// DCC commands and dangerous administrative commands, and increments
// RecordFilterBlock on m for each blocked message. m may be nil.
func DefaultIRCServerConfigWithMetrics(m *metrics.TunnelMetrics) ircinspector.Config {
	return ircinspector.Config{
		OnMessage: func(msg *ircinspector.Message) error {
			command := strings.ToUpper(msg.Command)

			// Block dangerous administrative commands
			switch command {
			case "ADMIN", "OPER", "DIE", "RESTART", "REHASH", "KILL":
				if m != nil {
					m.RecordFilterBlock()
				}
				return fmt.Errorf("administrative command %s not allowed over I2P", command)
			}

			// Block DCC commands
			if command == "DCC" {
				if m != nil {
					m.RecordFilterBlock()
				}
				return fmt.Errorf("DCC commands are not allowed over I2P")
			}

			// Block DCC in CTCP messages
			if command == "PRIVMSG" && strings.Contains(strings.ToUpper(msg.Trailing), "DCC") {
				if m != nil {
					m.RecordFilterBlock()
				}
				return fmt.Errorf("DCC CTCP commands are not allowed over I2P")
			}

			return nil
		},
		OnNumeric: func(numeric int, msg *ircinspector.Message) error {
			return nil
		},
	}
}

// ApplyIRCServerFilterRules adds hostname-masking filter rules to an existing
// IRC inspector. Call this after ircinspector.New() to add JOIN and WHOIS
// filters that replace real hostnames with the I2P address.
// m may be nil; it is reserved for future filter-block counting.
func ApplyIRCServerFilterRules(inspector *ircinspector.Inspector, i2pHost string, m *metrics.TunnelMetrics) {
	// Filter JOIN messages to mask hostnames
	inspector.AddFilter(ircinspector.Filter{
		Command: "JOIN",
		Callback: func(msg *ircinspector.Message) error {
			if i2pHost != "" && msg.Prefix != "" {
				msg.Prefix = userHostPattern.ReplaceAllString(msg.Prefix, "@"+i2pHost)
			}
			return nil
		},
	})

	// Filter WHOIS responses to hide real IP information
	inspector.AddFilter(ircinspector.Filter{
		Command: "WHOIS",
		Callback: func(msg *ircinspector.Message) error {
			if i2pHost != "" && msg.Trailing != "" {
				msg.Trailing = userHostPattern.ReplaceAllString(msg.Trailing, "@"+i2pHost)
			}
			return nil
		},
	})
}

// ApplyIRCServerFilters wraps a listener with IRC server-side filtering.
// Blocks administrative commands and masks host information.
func ApplyIRCServerFilters(listener net.Listener, i2pHost string) net.Listener {
	inspector := ircinspector.New(listener, DefaultIRCServerConfig())
	ApplyIRCServerFilterRules(inspector, i2pHost, nil)
	return inspector
}
