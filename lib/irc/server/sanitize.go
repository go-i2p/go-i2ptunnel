package ircserver

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	ircinspector "github.com/go-i2p/go-connfilter/irc"
)

// Pattern to match user@host format
var userHostPattern = regexp.MustCompile(`@[^\s]+`)

// ApplyIRCServerFilters wraps a listener with IRC server-side filtering
// Blocks administrative commands and masks host information
func ApplyIRCServerFilters(listener net.Listener, i2pHost string) net.Listener {
	config := ircinspector.Config{
		OnMessage: func(msg *ircinspector.Message) error {
			command := strings.ToUpper(msg.Command)

			// Block dangerous administrative commands
			switch command {
			case "ADMIN", "OPER", "DIE", "RESTART", "REHASH", "KILL":
				return fmt.Errorf("administrative command %s not allowed over I2P", command)
			}

			// Block DCC commands - direct connections expose real IPs
			if command == "DCC" {
				return fmt.Errorf("DCC commands are not allowed over I2P")
			}

			// Block DCC in CTCP messages
			if command == "PRIVMSG" && strings.Contains(strings.ToUpper(msg.Trailing), "DCC") {
				return fmt.Errorf("DCC CTCP commands are not allowed over I2P")
			}

			return nil
		},
		OnNumeric: func(numeric int, msg *ircinspector.Message) error {
			// Mask WHOIS responses (numeric 311) to hide real hostnames
			if numeric == 311 && i2pHost != "" {
				// WHOIS user response format: 311 nick user host * :realname
				if len(msg.Params) >= 3 {
					// Replace real host with I2P host
					msg.Params[2] = i2pHost
				}
			}
			return nil
		},
	}

	inspector := ircinspector.New(listener, config)

	// Filter JOIN messages to mask hostnames
	inspector.AddFilter(ircinspector.Filter{
		Command: "JOIN",
		Callback: func(msg *ircinspector.Message) error {
			// Mask user@host in prefix
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
			// Mask any @host patterns in the response
			if i2pHost != "" && msg.Trailing != "" {
				msg.Trailing = userHostPattern.ReplaceAllString(msg.Trailing, "@"+i2pHost)
			}
			return nil
		},
	})

	return inspector
}
