package ircclient

import (
	"testing"

	ircinspector "github.com/go-i2p/go-connfilter/irc"
)

// TestDefaultIRCClientConfig verifies the default config blocks DCC commands.
// Why: DCC commands can leak real IP addresses, defeating I2P anonymity.
func TestDefaultIRCClientConfig(t *testing.T) {
	config := DefaultIRCClientConfig()

	if config.OnMessage == nil {
		t.Fatal("DefaultIRCClientConfig().OnMessage should not be nil")
	}

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

	t.Run("blocks DCC case-insensitive", func(t *testing.T) {
		msg := &ircinspector.Message{
			Command: "dcc",
			Params:  []string{"CHAT"},
		}
		err := config.OnMessage(msg)
		if err == nil {
			t.Error("DCC command (lowercase) should be blocked")
		}
	})

	t.Run("blocks DCC in CTCP PRIVMSG", func(t *testing.T) {
		msg := &ircinspector.Message{
			Command:  "PRIVMSG",
			Params:   []string{"#channel"},
			Trailing: "\x01DCC SEND file.txt 192.168.1.1 5000 1024\x01",
		}
		err := config.OnMessage(msg)
		if err == nil {
			t.Error("DCC CTCP in PRIVMSG should be blocked")
		}
	})

	t.Run("allows normal PRIVMSG", func(t *testing.T) {
		msg := &ircinspector.Message{
			Command:  "PRIVMSG",
			Params:   []string{"#channel"},
			Trailing: "Hello, world!",
		}
		err := config.OnMessage(msg)
		if err != nil {
			t.Errorf("Normal PRIVMSG should be allowed, got: %v", err)
		}
	})

	t.Run("allows normal commands", func(t *testing.T) {
		commands := []string{"JOIN", "PART", "NICK", "QUIT", "PING", "PONG"}
		for _, cmd := range commands {
			msg := &ircinspector.Message{
				Command: cmd,
				Params:  []string{"test"},
			}
			err := config.OnMessage(msg)
			if err != nil {
				t.Errorf("Command %s should be allowed, got: %v", cmd, err)
			}
		}
	})
}
