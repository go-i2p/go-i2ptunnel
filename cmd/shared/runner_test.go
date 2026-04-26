package shared

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// mockTunnel implements i2ptunnel.I2PTunnel for unit testing CLI helpers.
type mockTunnel struct {
	stopped      bool
	started      bool
	configLoaded bool

	stopErr       error
	startErr      error
	loadConfigErr error
	localAddrVal  string
	localAddrErr  error
	setOptsErr    error

	opts        map[string]string
	lastSetOpts map[string]string
}

func (m *mockTunnel) Start() error                      { m.started = true; return m.startErr }
func (m *mockTunnel) Stop() error                       { m.stopped = true; return m.stopErr }
func (m *mockTunnel) Name() string                      { return "mock-tunnel" }
func (m *mockTunnel) ID() string                        { return "mock-tunnel" }
func (m *mockTunnel) Type() string                      { return "mock" }
func (m *mockTunnel) Address() string                   { return "" }
func (m *mockTunnel) Target() string                    { return "" }
func (m *mockTunnel) Error() error                      { return nil }
func (m *mockTunnel) Status() i2ptunnel.I2PTunnelStatus { return i2ptunnel.I2PTunnelStatusStopped }

func (m *mockTunnel) Options() map[string]string {
	if m.opts != nil {
		return m.opts
	}
	return map[string]string{}
}
func (m *mockTunnel) SetOptions(opts map[string]string) error {
	m.lastSetOpts = opts
	return m.setOptsErr
}
func (m *mockTunnel) LoadConfig(path string) error {
	m.configLoaded = true
	return m.loadConfigErr
}
func (m *mockTunnel) LocalAddress() (string, error) {
	return m.localAddrVal, m.localAddrErr
}

// TestLocalAddrWithError tests localAddr when the tunnel returns an error.
func TestLocalAddrWithError(t *testing.T) {
	tunnel := &mockTunnel{localAddrErr: os.ErrNotExist}
	addr := localAddr(tunnel)
	if addr != "(unknown)" {
		t.Errorf("Expected '(unknown)', got '%s'", addr)
	}
}

// TestLocalAddrSuccess tests localAddr with a working tunnel.
func TestLocalAddrSuccess(t *testing.T) {
	tunnel := &mockTunnel{localAddrVal: "127.0.0.1:8080"}
	addr := localAddr(tunnel)
	if addr != "127.0.0.1:8080" {
		t.Errorf("Expected '127.0.0.1:8080', got '%s'", addr)
	}
}

// TestPromptLeaseSetCredentialNoAuth verifies that tunnels with no auth type
// (the common case) skip the prompt and return nil.
func TestPromptLeaseSetCredentialNoAuth(t *testing.T) {
	tunnel := &mockTunnel{
		opts: map[string]string{"i2cp.leaseSetAuthType": "0"},
	}
	var out bytes.Buffer
	err := promptLeaseSetCredential(tunnel, strings.NewReader(""), &out)
	if err != nil {
		t.Fatalf("expected nil for auth type 0, got: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for auth type 0, got: %q", out.String())
	}
	if tunnel.lastSetOpts != nil {
		t.Error("SetOptions should not have been called for auth type 0")
	}
}

// TestPromptLeaseSetCredentialMissingAuthType verifies that a tunnel with no
// i2cp.leaseSetAuthType option also skips the prompt.
func TestPromptLeaseSetCredentialMissingAuthType(t *testing.T) {
	tunnel := &mockTunnel{}
	err := promptLeaseSetCredential(tunnel, strings.NewReader(""), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("expected nil when leaseSetAuthType is absent, got: %v", err)
	}
}

// TestPromptLeaseSetCredentialDH verifies that auth type "1" (DH) prompts for
// the key and passes it to SetOptions.
func TestPromptLeaseSetCredentialDH(t *testing.T) {
	const fakeKey = "abc123fakeBase64Key=="
	tunnel := &mockTunnel{
		opts: map[string]string{"i2cp.leaseSetAuthType": "1"},
	}
	var out bytes.Buffer
	err := promptLeaseSetCredential(tunnel, strings.NewReader(fakeKey+"\n"), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tunnel.lastSetOpts == nil {
		t.Fatal("SetOptions was not called")
	}
	if got := tunnel.lastSetOpts["i2cp.leaseSetPrivKey"]; got != fakeKey {
		t.Errorf("SetOptions got key %q, want %q", got, fakeKey)
	}
	if !strings.Contains(out.String(), "DH") {
		t.Errorf("expected DH in prompt output, got: %q", out.String())
	}
}

// TestPromptLeaseSetCredentialPSK verifies that auth type "2" (PSK) prompts
// with the correct type name and passes the key to SetOptions.
func TestPromptLeaseSetCredentialPSK(t *testing.T) {
	const fakeKey = "pskKey999=="
	tunnel := &mockTunnel{
		opts: map[string]string{"i2cp.leaseSetAuthType": "2"},
	}
	var out bytes.Buffer
	err := promptLeaseSetCredential(tunnel, strings.NewReader(fakeKey+"\n"), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tunnel.lastSetOpts["i2cp.leaseSetPrivKey"]; got != fakeKey {
		t.Errorf("SetOptions got key %q, want %q", got, fakeKey)
	}
	if !strings.Contains(out.String(), "PSK") {
		t.Errorf("expected PSK in prompt output, got: %q", out.String())
	}
}

// TestPromptLeaseSetCredentialEmptyInput verifies that an empty key returns an error.
func TestPromptLeaseSetCredentialEmptyInput(t *testing.T) {
	tunnel := &mockTunnel{
		opts: map[string]string{"i2cp.leaseSetAuthType": "2"},
	}
	err := promptLeaseSetCredential(tunnel, strings.NewReader("\n"), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for empty key input")
	}
}

// TestPromptLeaseSetCredentialSetOptionsError verifies that a SetOptions error
// is propagated to the caller.
func TestPromptLeaseSetCredentialSetOptionsError(t *testing.T) {
	tunnel := &mockTunnel{
		opts:       map[string]string{"i2cp.leaseSetAuthType": "1"},
		setOptsErr: errors.New("connection refused"),
	}
	err := promptLeaseSetCredential(tunnel, strings.NewReader("somekey\n"), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error when SetOptions fails")
	}
}
