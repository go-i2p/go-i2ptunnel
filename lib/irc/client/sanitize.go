package ircclient

import (
	"fmt"
	"net"
	"strings"

	ircinspector "github.com/go-i2p/go-connfilter/irc"
)

// ApplyIRCClientFilters wraps a listener with IRC client-side filtering
// Blocks dangerous commands like DCC and filters potentially leaky content
func ApplyIRCClientFilters(listener net.Listener, i2pHost string) net.Listener {
	config := ircinspector.Config{
		OnMessage: func(msg *ircinspector.Message) error {
			// Block DCC commands - direct client connections expose real IP
			if strings.EqualFold(msg.Command, "DCC") {
				return fmt.Errorf("DCC commands are not allowed over I2P")
			}

			// Check for DCC in CTCP messages (inside PRIVMSG trailing)
			if strings.EqualFold(msg.Command, "PRIVMSG") && strings.Contains(msg.Trailing, "\x01DCC") {
				return fmt.Errorf("DCC CTCP commands are not allowed over I2P")
			}

			return nil
		},
	}

	inspector := ircinspector.New(listener, config)

	// Replace real hostnames with .i2p addresses in PING responses
	inspector.AddFilter(ircinspector.Filter{
		Command: "PING",
		Callback: func(msg *ircinspector.Message) error {
			if i2pHost != "" && len(msg.Params) > 0 {
				// Replace any real hostname with I2P hostname
				msg.Params[0] = i2pHost
			}
			return nil
		},
	})

	// Mask user@host information to prevent identity leakage
	inspector.AddFilter(ircinspector.Filter{
		Command: "USERHOST",
		Callback: func(msg *ircinspector.Message) error {
			// Mask the response to hide real user@host
			if i2pHost != "" && msg.Trailing != "" {
				// Replace user@realhost with user@i2phost
				parts := strings.Split(msg.Trailing, "@")
				if len(parts) > 1 {
					msg.Trailing = parts[0] + "@" + i2pHost
				}
			}
			return nil
		},
	})

	return inspector
}
