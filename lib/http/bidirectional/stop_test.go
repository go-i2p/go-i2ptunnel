package httpbidirectional

import (
	"testing"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	limitedlistener "github.com/go-i2p/go-limit"
)

// TestStopIdempotent verifies that calling Stop() multiple times does not panic.
func TestStopIdempotent(t *testing.T) {
	tunnel := &HTTPBidirectional{
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
}

// TestDoneChannelSignaling verifies the done channel is properly closed on Stop().
func TestDoneChannelSignaling(t *testing.T) {
	tunnel := &HTTPBidirectional{
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

// TestHTTPBidirectionalType verifies the tunnel reports its type correctly.
func TestHTTPBidirectionalType(t *testing.T) {
	tunnel := &HTTPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Type: "httpbidirectional",
		},
		done: make(chan struct{}),
	}
	if tunnel.Type() != "httpbidirectional" {
		t.Errorf("Expected type httpbidirectional, got %s", tunnel.Type())
	}
}

// TestHTTPBidirectionalName verifies the tunnel reports its name correctly.
func TestHTTPBidirectionalName(t *testing.T) {
	tunnel := &HTTPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Name: "test-http-bidi",
		},
		done: make(chan struct{}),
	}
	if tunnel.Name() != "test-http-bidi" {
		t.Errorf("Expected name test-http-bidi, got %s", tunnel.Name())
	}
}

// TestHTTPBidirectionalID verifies clean ID generation from tunnel name.
func TestHTTPBidirectionalID(t *testing.T) {
	tunnel := &HTTPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Name: "my http bidi",
		},
		done: make(chan struct{}),
	}
	if tunnel.ID() != "my-http-bidi" {
		t.Errorf("Expected ID my-http-bidi, got %s", tunnel.ID())
	}
}

// TestHTTPBidirectionalOptions verifies Options returns all expected keys.
func TestHTTPBidirectionalOptions(t *testing.T) {
	tunnel := &HTTPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Name:      "test-bidi",
			Type:      "httpbidirectional",
			Interface: "127.0.0.1",
			Port:      4449,
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
		"type":      "httpbidirectional",
		"interface": "127.0.0.1",
		"port":      "4449",
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

// TestHTTPBidirectionalSetOptions verifies option setting with validation.
func TestHTTPBidirectionalSetOptions(t *testing.T) {
	tunnel := &HTTPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Name:      "original",
			Interface: "127.0.0.1",
			Port:      4449,
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

// TestHTTPBidirectionalSetOptionsValidation verifies option validation catches errors.
func TestHTTPBidirectionalSetOptionsValidation(t *testing.T) {
	tunnel := &HTTPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Name: "test",
		},
		done: make(chan struct{}),
	}

	err := tunnel.SetOptions(map[string]string{"name": ""})
	if err == nil {
		t.Error("SetOptions with empty name should return error")
	}

	err = tunnel.SetOptions(map[string]string{"port": "notanumber"})
	if err == nil {
		t.Error("SetOptions with invalid port should return error")
	}

	err = tunnel.SetOptions(map[string]string{"interface": "not-an-ip"})
	if err == nil {
		t.Error("SetOptions with invalid interface should return error")
	}
}

// TestHTTPBidirectionalLocalAddress verifies LocalAddress returns the HTTP proxy address.
func TestHTTPBidirectionalLocalAddress(t *testing.T) {
	tunnel := &HTTPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Interface: "127.0.0.1",
			Port:      4449,
		},
		done: make(chan struct{}),
	}
	addr, err := tunnel.LocalAddress()
	if err != nil {
		t.Fatalf("LocalAddress() returned error: %v", err)
	}
	if addr != "127.0.0.1:4449" {
		t.Errorf("Expected 127.0.0.1:4449, got %s", addr)
	}
}

// TestHTTPBidirectionalAddressEmpty verifies Address returns empty when no Garlic is set.
func TestHTTPBidirectionalAddressEmpty(t *testing.T) {
	tunnel := &HTTPBidirectional{
		done: make(chan struct{}),
	}
	if tunnel.Address() != "" {
		t.Errorf("Expected empty address when Garlic is nil, got %s", tunnel.Address())
	}
}

// TestHTTPBidirectionalErrorNil verifies Error returns nil when no errors recorded.
func TestHTTPBidirectionalErrorNil(t *testing.T) {
	tunnel := &HTTPBidirectional{
		done: make(chan struct{}),
	}
	if tunnel.Error() != nil {
		t.Errorf("Expected nil error, got %v", tunnel.Error())
	}
}

// TestHTTPBidirectionalLoadConfigWhileRunning verifies LoadConfig fails when running.
func TestHTTPBidirectionalLoadConfigWhileRunning(t *testing.T) {
	tunnel := &HTTPBidirectional{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		done:            make(chan struct{}),
	}
	err := tunnel.LoadConfig("/nonexistent")
	if err == nil {
		t.Error("LoadConfig while running should return error")
	}
}

// TestHTTPBidirectionalLoadConfigWhileStarting verifies LoadConfig fails when starting.
func TestHTTPBidirectionalLoadConfigWhileStarting(t *testing.T) {
	tunnel := &HTTPBidirectional{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStarting,
		done:            make(chan struct{}),
	}
	err := tunnel.LoadConfig("/nonexistent")
	if err == nil {
		t.Error("LoadConfig while starting should return error")
	}
}

// TestHTTPBidirectionalImplementsInterface confirms the type satisfies I2PTunnel.
func TestHTTPBidirectionalImplementsInterface(t *testing.T) {
	var _ i2ptunnel.I2PTunnel = &HTTPBidirectional{}
}
