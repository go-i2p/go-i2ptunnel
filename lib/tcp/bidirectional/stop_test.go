package tcpbidirectional

import (
	"sync"
	"testing"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	limitedlistener "github.com/go-i2p/go-limit"
)

// TestStopIdempotent verifies that calling Stop() multiple times does not panic.
func TestStopIdempotent(t *testing.T) {
	tunnel := &TCPBidirectional{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		done:            make(chan struct{}),
	}

	if err := tunnel.Stop(); err != nil {
		t.Fatalf("First Stop() returned error: %v", err)
	}
	if tunnel.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected status stopped after first Stop(), got %v", tunnel.Status())
	}

	if err := tunnel.Stop(); err != nil {
		t.Fatalf("Second Stop() returned error: %v", err)
	}
	if tunnel.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected status stopped after second Stop(), got %v", tunnel.Status())
	}
}

// TestDoneChannelSignaling verifies the done channel is properly closed on Stop().
func TestDoneChannelSignaling(t *testing.T) {
	tunnel := &TCPBidirectional{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		done:            make(chan struct{}),
	}

	select {
	case <-tunnel.done:
		t.Fatal("done channel should not be closed before Stop()")
	default:
	}

	tunnel.Stop()

	select {
	case <-tunnel.done:
		// expected
	default:
		t.Fatal("done channel should be closed after Stop()")
	}
}

// TestTCPBidirectionalType verifies the tunnel reports its type correctly.
func TestTCPBidirectionalType(t *testing.T) {
	tunnel := &TCPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Type: "tcpbidirectional",
		},
		done: make(chan struct{}),
	}
	if tunnel.Type() != "tcpbidirectional" {
		t.Errorf("Expected type tcpbidirectional, got %s", tunnel.Type())
	}
}

// TestTCPBidirectionalName verifies the tunnel reports its name correctly.
func TestTCPBidirectionalName(t *testing.T) {
	tunnel := &TCPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Name: "test-tcp-bidi",
		},
		done: make(chan struct{}),
	}
	if tunnel.Name() != "test-tcp-bidi" {
		t.Errorf("Expected name test-tcp-bidi, got %s", tunnel.Name())
	}
}

// TestTCPBidirectionalID verifies clean ID generation from tunnel name.
func TestTCPBidirectionalID(t *testing.T) {
	tunnel := &TCPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Name: "my tcp bidi",
		},
		done: make(chan struct{}),
	}
	if tunnel.ID() != "my-tcp-bidi" {
		t.Errorf("Expected ID my-tcp-bidi, got %s", tunnel.ID())
	}
}

// TestTCPBidirectionalOptions verifies Options returns all expected keys.
func TestTCPBidirectionalOptions(t *testing.T) {
	tunnel := &TCPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Name:      "test-bidi",
			Type:      "tcpbidirectional",
			Interface: "127.0.0.1",
			Port:      4447,
		},
		LimitedConfig: limitedlistener.LimitedConfig{
			MaxConns:  500,
			RateLimit: 50,
		},
		done: make(chan struct{}),
	}
	opts := tunnel.Options()

	checks := map[string]string{
		"name":      "test-bidi",
		"type":      "tcpbidirectional",
		"interface": "127.0.0.1",
		"port":      "4447",
		"maxconns":  "500",
		"ratelimit": "50",
	}
	for key, want := range checks {
		if got, ok := opts[key]; !ok {
			t.Errorf("Options() missing key %q", key)
		} else if got != want {
			t.Errorf("Options()[%q] = %q, want %q", key, got, want)
		}
	}
}

// TestTCPBidirectionalSetOptions verifies option setting with validation.
func TestTCPBidirectionalSetOptions(t *testing.T) {
	tunnel := &TCPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Name:      "original",
			Interface: "127.0.0.1",
			Port:      4447,
		},
		LimitedConfig: limitedlistener.LimitedConfig{
			MaxConns:  1000,
			RateLimit: 100,
		},
		done: make(chan struct{}),
	}

	err := tunnel.SetOptions(map[string]string{
		"name":      "updated",
		"port":      "5555",
		"maxconns":  "200",
		"ratelimit": "25",
	})
	if err != nil {
		t.Fatalf("SetOptions() returned error: %v", err)
	}
	if tunnel.Name() != "updated" {
		t.Errorf("Expected name updated, got %s", tunnel.Name())
	}
	if tunnel.TunnelConfig.Port != 5555 {
		t.Errorf("Expected port 5555, got %d", tunnel.TunnelConfig.Port)
	}
}

// TestTCPBidirectionalSetOptionsValidation verifies option validation catches errors.
func TestTCPBidirectionalSetOptionsValidation(t *testing.T) {
	tunnel := &TCPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Name: "test",
		},
		done: make(chan struct{}),
	}

	// Empty name should fail
	err := tunnel.SetOptions(map[string]string{"name": ""})
	if err == nil {
		t.Error("SetOptions with empty name should return error")
	}

	// Invalid port should fail
	err = tunnel.SetOptions(map[string]string{"port": "notanumber"})
	if err == nil {
		t.Error("SetOptions with invalid port should return error")
	}

	// Invalid interface should fail
	err = tunnel.SetOptions(map[string]string{"interface": "not-an-ip"})
	if err == nil {
		t.Error("SetOptions with invalid interface should return error")
	}
}

// TestTCPBidirectionalLocalAddress verifies LocalAddress returns the SOCKS proxy address.
func TestTCPBidirectionalLocalAddress(t *testing.T) {
	tunnel := &TCPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Interface: "127.0.0.1",
			Port:      4447,
		},
		done: make(chan struct{}),
	}
	addr, err := tunnel.LocalAddress()
	if err != nil {
		t.Fatalf("LocalAddress() returned error: %v", err)
	}
	if addr != "127.0.0.1:4447" {
		t.Errorf("Expected 127.0.0.1:4447, got %s", addr)
	}
}

// TestTCPBidirectionalAddressEmpty verifies Address returns empty when no Garlic is set.
func TestTCPBidirectionalAddressEmpty(t *testing.T) {
	tunnel := &TCPBidirectional{
		done: make(chan struct{}),
	}
	if tunnel.Address() != "" {
		t.Errorf("Expected empty address when Garlic is nil, got %s", tunnel.Address())
	}
}

// TestTCPBidirectionalErrorNil verifies Error returns nil when no errors recorded.
func TestTCPBidirectionalErrorNil(t *testing.T) {
	tunnel := &TCPBidirectional{
		done: make(chan struct{}),
	}
	if tunnel.Error() != nil {
		t.Errorf("Expected nil error, got %v", tunnel.Error())
	}
}

// TestTCPBidirectionalLoadConfigWhileRunning verifies LoadConfig fails when running.
func TestTCPBidirectionalLoadConfigWhileRunning(t *testing.T) {
	tunnel := &TCPBidirectional{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		done:            make(chan struct{}),
	}
	err := tunnel.LoadConfig("/nonexistent")
	if err == nil {
		t.Error("LoadConfig while running should return error")
	}
}

// TestTCPBidirectionalLoadConfigWhileStarting verifies LoadConfig fails when starting.
func TestTCPBidirectionalLoadConfigWhileStarting(t *testing.T) {
	tunnel := &TCPBidirectional{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStarting,
		done:            make(chan struct{}),
	}
	err := tunnel.LoadConfig("/nonexistent")
	if err == nil {
		t.Error("LoadConfig while starting should return error")
	}
}

// TestTCPBidirectionalImplementsInterface confirms the type satisfies I2PTunnel.
func TestTCPBidirectionalImplementsInterface(t *testing.T) {
	var _ i2ptunnel.I2PTunnel = &TCPBidirectional{}
}

// TestRestartAfterStop verifies that after Stop(), the done channel and stopOnce
// can be reset (as Start() now does) so the tunnel is restartable.
func TestRestartAfterStop(t *testing.T) {
	tunnel := &TCPBidirectional{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		done:            make(chan struct{}),
	}

	tunnel.Stop()
	select {
	case <-tunnel.done:
	default:
		t.Fatal("done channel should be closed after first Stop()")
	}

	tunnel.done = make(chan struct{})
	tunnel.stopOnce = sync.Once{}

	select {
	case <-tunnel.done:
		t.Fatal("done channel should be open after reset")
	default:
	}

	tunnel.Stop()
	select {
	case <-tunnel.done:
	default:
		t.Fatal("done channel should be closed after second Stop()")
	}
	if tunnel.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected status stopped after restart cycle, got %v", tunnel.Status())
	}
}
