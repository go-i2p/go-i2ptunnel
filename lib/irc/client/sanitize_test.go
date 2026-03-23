package ircclient

import (
	"strings"
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

// IP addresses as 32-bit big-endian integers (DCC protocol format):
//
//	192.168.1.1 → 3232235777  (private, RFC 1918)
//	10.0.0.1    → 167772161   (private, RFC 1918)
//	127.0.0.1   → 2130706433  (loopback)
//	172.16.0.1  → 2886729729  (private, RFC 1918)
//	1.2.3.4     → 16909060    (public)
//	8.8.8.8     → 134744072   (public)
func TestFilterDCCRequest(t *testing.T) {
	t.Run("blocked DCC SEND to private IP (192.168.x.x)", func(t *testing.T) {
		err := filterDCCRequest("DCC SEND file.txt 3232235777 5000 1024")
		if err == nil {
			t.Fatal("expected error for private IP DCC SEND")
		}
		if !strings.Contains(err.Error(), "private IP") {
			t.Errorf("expected 'private IP' in error, got: %v", err)
		}
	})

	t.Run("blocked DCC SEND to private IP (10.x.x.x)", func(t *testing.T) {
		err := filterDCCRequest("DCC SEND file.txt 167772161 5000 1024")
		if err == nil {
			t.Fatal("expected error for private IP DCC SEND")
		}
		if !strings.Contains(err.Error(), "private IP") {
			t.Errorf("expected 'private IP' in error, got: %v", err)
		}
	})

	t.Run("blocked DCC SEND to loopback", func(t *testing.T) {
		err := filterDCCRequest("DCC SEND file.txt 2130706433 5000 1024")
		if err == nil {
			t.Fatal("expected error for loopback IP DCC SEND")
		}
	})

	t.Run("blocked DCC SEND to private IP (172.16.x.x)", func(t *testing.T) {
		err := filterDCCRequest("DCC SEND file.txt 2886729729 5000 1024")
		if err == nil {
			t.Fatal("expected error for 172.16.x.x DCC SEND")
		}
	})

	t.Run("blocked DCC with invalid port (too low)", func(t *testing.T) {
		// 1.2.3.4 = 16909060, port 80 < 1024
		err := filterDCCRequest("DCC SEND file.txt 16909060 80 1024")
		if err == nil {
			t.Fatal("expected error for port < 1024")
		}
		if !strings.Contains(err.Error(), "port") {
			t.Errorf("expected 'port' in error, got: %v", err)
		}
	})

	t.Run("blocked DCC with invalid port (too high)", func(t *testing.T) {
		err := filterDCCRequest("DCC SEND file.txt 16909060 70000 1024")
		if err == nil {
			t.Fatal("expected error for port > 65535")
		}
	})

	t.Run("allowed DCC with valid public parameters", func(t *testing.T) {
		// 1.2.3.4 = 16909060, port 5000 — params are valid; caller still blocks DCC
		err := filterDCCRequest("DCC SEND file.txt 16909060 5000 102400")
		if err != nil {
			t.Errorf("filterDCCRequest returned unexpected error for public IP / valid port: %v", err)
		}
	})

	t.Run("blocked DCC CHAT to private IP", func(t *testing.T) {
		// 10.0.0.1 = 167772161
		err := filterDCCRequest("DCC CHAT chat 167772161 5000")
		if err == nil {
			t.Fatal("expected error for private IP DCC CHAT")
		}
		if !strings.Contains(err.Error(), "private IP") {
			t.Errorf("expected 'private IP' in error, got: %v", err)
		}
	})

	t.Run("blocked DCC CHAT with invalid port", func(t *testing.T) {
		// 8.8.8.8 = 134744072, port 22 < 1024
		err := filterDCCRequest("DCC CHAT chat 134744072 22")
		if err == nil {
			t.Fatal("expected error for invalid port DCC CHAT")
		}
	})

	t.Run("strips trailing CTCP delimiter", func(t *testing.T) {
		// Valid params with trailing \x01 — should still parse correctly
		err := filterDCCRequest("DCC SEND file.txt 16909060 5000 1024\x01")
		if err != nil {
			t.Errorf("unexpected error with trailing control char: %v", err)
		}
	})

	t.Run("unrecognized DCC subcommand returns nil", func(t *testing.T) {
		// RESUME, ACCEPT, etc. are not parsed; filterDCCRequest returns nil
		err := filterDCCRequest("DCC RESUME file.txt 5000 0")
		if err != nil {
			t.Errorf("unexpected error for unrecognized DCC subcommand: %v", err)
		}
	})
}

func TestDefaultIRCClientConfigDCCWithIntegerIP(t *testing.T) {
	config := DefaultIRCClientConfig()

	t.Run("blocks DCC SEND to private IP (integer format)", func(t *testing.T) {
		msg := &ircinspector.Message{
			Command:  "PRIVMSG",
			Params:   []string{"#channel"},
			Trailing: "\x01DCC SEND file.txt 3232235777 5000 1024\x01",
		}
		err := config.OnMessage(msg)
		if err == nil {
			t.Fatal("expected error for private-IP DCC SEND in PRIVMSG")
		}
		if !strings.Contains(err.Error(), "private IP") {
			t.Errorf("expected 'private IP' in error, got: %v", err)
		}
	})

	t.Run("blocks DCC SEND with invalid port (integer format)", func(t *testing.T) {
		msg := &ircinspector.Message{
			Command:  "PRIVMSG",
			Params:   []string{"#channel"},
			Trailing: "\x01DCC SEND file.txt 16909060 80 1024\x01",
		}
		err := config.OnMessage(msg)
		if err == nil {
			t.Fatal("expected error for invalid-port DCC SEND in PRIVMSG")
		}
	})

	t.Run("blocks DCC SEND with valid public params (generic DCC block)", func(t *testing.T) {
		// filterDCCRequest returns nil (params OK), but DCC is still blocked generically
		msg := &ircinspector.Message{
			Command:  "PRIVMSG",
			Params:   []string{"#channel"},
			Trailing: "\x01DCC SEND file.txt 16909060 5000 1024\x01",
		}
		err := config.OnMessage(msg)
		if err == nil {
			t.Fatal("expected generic DCC block error")
		}
	})
}
