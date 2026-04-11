package loader

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func writeYAML(t *testing.T, tunnelName, tunnelType, target string, port int) string {
	t.Helper()
	content := fmt.Sprintf("tunnels:\n  %s:\n    name: %q\n    type: %q\n    interface: \"127.0.0.1\"\n", tunnelName, tunnelName, tunnelType)
	if target != "" {
		content += fmt.Sprintf("    target: %q\n", target)
	}
	if port > 0 {
		content += fmt.Sprintf("    port: %d\n", port)
	}
	f, err := os.CreateTemp(t.TempDir(), "*.yaml")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	f.Close()
	return f.Name()
}

func isUnknownTypeError(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "unknown tunnel type:")
}

func TestLoadDispatch(t *testing.T) {
	tests := []struct {
		tunnelType string
		target     string
		port       int
	}{
		{"tcpclient", "example.b32.i2p", 4488},
		{"ircclient", "example.b32.i2p", 6668},
		{"udpclient", "example.b32.i2p", 7701},
		{"tcpserver", "127.0.0.1:8080", 0},
		{"httpserver", "127.0.0.1:8080", 0},
		{"ircserver", "127.0.0.1:6667", 0},
		{"udpserver", "127.0.0.1:7700", 0},
		{"httpclient", "", 4444},
		{"socks", "", 4447},
		{"socksclient", "", 4447},
		{"tcpbidirectional", "127.0.0.1:8080", 4447},
		{"udpbidirectional", "127.0.0.1:7700", 4447},
		{"httpbidirectional", "127.0.0.1:8080", 4444},
	}

	for _, tt := range tests {
		t.Run(tt.tunnelType, func(t *testing.T) {
			path := writeYAML(t, tt.tunnelType, tt.tunnelType, tt.target, tt.port)
			_, err := Load(path, "localhost:1")
			if isUnknownTypeError(err) {
				t.Errorf("tunnel type %q was not dispatched: %v", tt.tunnelType, err)
			}
			// Any other error (SAM connection refused, etc.) is expected in a test environment.
		})
	}
}

func TestLoadUnknownType(t *testing.T) {
	path := writeYAML(t, "tunnel", "unknowntype", "", 4400)
	_, err := Load(path, "localhost:1")
	if !isUnknownTypeError(err) {
		t.Errorf("expected unknown tunnel type error, got: %v", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/tunnel.yaml", "localhost:1")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "*.yaml")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	f.WriteString(":::not valid yaml:::")
	f.Close()
	_, err = Load(f.Name(), "localhost:1")
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestValidateHost(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{nil, DefaultSAMAddress},
		{[]string{}, DefaultSAMAddress},
		{[]string{"127.0.0.1:7656"}, "127.0.0.1:7656"},
		{[]string{"localhost:7656"}, "localhost:7656"},
		{[]string{"not-valid"}, DefaultSAMAddress},
		{[]string{"127.0.0.1", "7656"}, "127.0.0.1:7656"},
		{[]string{"127.0.0.1", "notaport"}, DefaultSAMAddress},
		{[]string{"a", "b", "c"}, DefaultSAMAddress},
	}
	for _, tt := range tests {
		got := validateHost(tt.input...)
		if got != tt.expected {
			t.Errorf("validateHost(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
