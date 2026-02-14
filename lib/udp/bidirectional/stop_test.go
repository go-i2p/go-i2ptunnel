package udpbidirectional

import (
"testing"

i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// TestStopIdempotent verifies that calling Stop() multiple times does not panic.
func TestStopIdempotent(t *testing.T) {
tunnel := &UDPBidirectional{
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
tunnel := &UDPBidirectional{
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

// TestUDPBidirectionalType verifies the tunnel reports its type correctly.
func TestUDPBidirectionalType(t *testing.T) {
tunnel := &UDPBidirectional{
TunnelConfig: i2pconv.TunnelConfig{
Type: "udpbidirectional",
},
done: make(chan struct{}),
}
if tunnel.Type() != "udpbidirectional" {
t.Errorf("Expected type udpbidirectional, got %s", tunnel.Type())
}
}

// TestUDPBidirectionalName verifies the tunnel reports its name correctly.
func TestUDPBidirectionalName(t *testing.T) {
tunnel := &UDPBidirectional{
TunnelConfig: i2pconv.TunnelConfig{
Name: "test-udp-bidi",
},
done: make(chan struct{}),
}
if tunnel.Name() != "test-udp-bidi" {
t.Errorf("Expected name test-udp-bidi, got %s", tunnel.Name())
}
}

// TestUDPBidirectionalID verifies clean ID generation from tunnel name.
func TestUDPBidirectionalID(t *testing.T) {
tunnel := &UDPBidirectional{
TunnelConfig: i2pconv.TunnelConfig{
Name: "my udp bidi",
},
done: make(chan struct{}),
}
if tunnel.ID() != "my-udp-bidi" {
t.Errorf("Expected ID my-udp-bidi, got %s", tunnel.ID())
}
}

// TestUDPBidirectionalOptions verifies Options returns all expected keys.
func TestUDPBidirectionalOptions(t *testing.T) {
tunnel := &UDPBidirectional{
TunnelConfig: i2pconv.TunnelConfig{
Name:      "test-bidi",
Type:      "udpbidirectional",
Interface: "127.0.0.1",
Port:      4448,
},
done: make(chan struct{}),
}
opts := tunnel.Options()

checks := map[string]string{
"name":      "test-bidi",
"type":      "udpbidirectional",
"interface": "127.0.0.1",
"port":      "4448",
}
for key, want := range checks {
if got, ok := opts[key]; !ok {
t.Errorf("Options() missing key %q", key)
} else if got != want {
t.Errorf("Options()[%q] = %q, want %q", key, got, want)
}
}
}

// TestUDPBidirectionalSetOptions verifies option setting with validation.
func TestUDPBidirectionalSetOptions(t *testing.T) {
tunnel := &UDPBidirectional{
TunnelConfig: i2pconv.TunnelConfig{
Name:      "original",
Interface: "127.0.0.1",
Port:      4448,
},
done: make(chan struct{}),
}

err := tunnel.SetOptions(map[string]string{
"name": "updated",
"port": "5555",
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

// TestUDPBidirectionalSetOptionsValidation verifies option validation catches errors.
func TestUDPBidirectionalSetOptionsValidation(t *testing.T) {
tunnel := &UDPBidirectional{
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
}

// TestUDPBidirectionalLocalAddress verifies LocalAddress returns the SOCKS proxy address.
func TestUDPBidirectionalLocalAddress(t *testing.T) {
tunnel := &UDPBidirectional{
TunnelConfig: i2pconv.TunnelConfig{
Interface: "127.0.0.1",
Port:      4448,
},
done: make(chan struct{}),
}
addr, err := tunnel.LocalAddress()
if err != nil {
t.Fatalf("LocalAddress() returned error: %v", err)
}
if addr != "127.0.0.1:4448" {
t.Errorf("Expected 127.0.0.1:4448, got %s", addr)
}
}

// TestUDPBidirectionalAddressEmpty verifies Address returns empty when no Garlic is set.
func TestUDPBidirectionalAddressEmpty(t *testing.T) {
tunnel := &UDPBidirectional{
done: make(chan struct{}),
}
if tunnel.Address() != "" {
t.Errorf("Expected empty address when Garlic is nil, got %s", tunnel.Address())
}
}

// TestUDPBidirectionalErrorNil verifies Error returns nil when no errors recorded.
func TestUDPBidirectionalErrorNil(t *testing.T) {
tunnel := &UDPBidirectional{
done: make(chan struct{}),
}
if tunnel.Error() != nil {
t.Errorf("Expected nil error, got %v", tunnel.Error())
}
}

// TestUDPBidirectionalLoadConfigWhileRunning verifies LoadConfig fails when running.
func TestUDPBidirectionalLoadConfigWhileRunning(t *testing.T) {
tunnel := &UDPBidirectional{
I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
done:            make(chan struct{}),
}
err := tunnel.LoadConfig("/nonexistent")
if err == nil {
t.Error("LoadConfig while running should return error")
}
}

// TestUDPBidirectionalImplementsInterface confirms the type satisfies I2PTunnel.
func TestUDPBidirectionalImplementsInterface(t *testing.T) {
var _ i2ptunnel.I2PTunnel = &UDPBidirectional{}
}
