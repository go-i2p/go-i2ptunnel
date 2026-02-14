package ircserver

import (
	"testing"

	ircinspector "github.com/go-i2p/go-connfilter/irc"
)

// TestDefaultIRCServerConfig verifies the default config blocks admin and DCC commands.
// Why: Administrative commands and DCC can compromise server security and client privacy.
func TestDefaultIRCServerConfig(t *testing.T) {
	config := DefaultIRCServerConfig()

	if config.OnMessage == nil {
		t.Fatal("DefaultIRCServerConfig().OnMessage should not be nil")
	}

	t.Run("blocks administrative commands", func(t *testing.T) {
		adminCmds := []string{"ADMIN", "OPER", "DIE", "RESTART", "REHASH", "KILL"}
		for _, cmd := range adminCmds {
			msg := &ircinspector.Message{
				Command: cmd,
				Params:  []string{"test"},
			}
			err := config.OnMessage(msg)
			if err == nil {
				t.Errorf("Administrative command %s should be blocked", cmd)
			}
		}
	})

	t.Run("blocks DCC command", func(t *testing.T) {
		msg := &ircinspector.Message{
			Command: "DCC",
			Params:  []string{"SEND", "file.txt"},
		}
		err := config.OnMessage(msg)
		if err == nil {
			t.Error("DCC command should be blocked")
		}
	})

	t.Run("blocks DCC in CTCP PRIVMSG", func(t *testing.T) {
		msg := &ircinspector.Message{
			Command:  "PRIVMSG",
			Params:   []string{"#channel"},
			Trailing: "Contains DCC SEND command",
		}
		err := config.OnMessage(msg)
		if err == nil {
			t.Error("DCC in PRIVMSG trailing should be blocked")
		}
	})

	t.Run("allows normal commands", func(t *testing.T) {
		commands := []string{"JOIN", "PART", "NICK", "QUIT", "PING", "PONG", "PRIVMSG"}
		for _, cmd := range commands {
			msg := &ircinspector.Message{
				Command:  cmd,
				Params:   []string{"test"},
				Trailing: "Hello, world!",
			}
			err := config.OnMessage(msg)
			if err != nil {
				t.Errorf("Command %s should be allowed, got: %v", cmd, err)
			}
		}
	})
}
