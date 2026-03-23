package controller

import (
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// mockTunnel is a minimal I2PTunnel implementation for testing.
// All method return values are configurable via fields.
type mockTunnel struct {
	name    string
	id      string
	kind    string
	status  i2ptunnel.I2PTunnelStatus
	options map[string]string
}

func (m *mockTunnel) Start() error                      { return nil }
func (m *mockTunnel) Stop() error                       { return nil }
func (m *mockTunnel) Name() string                      { return m.name }
func (m *mockTunnel) ID() string                        { return m.id }
func (m *mockTunnel) Type() string                      { return m.kind }
func (m *mockTunnel) Address() string                   { return "" }
func (m *mockTunnel) Target() string                    { return "" }
func (m *mockTunnel) LocalAddress() (string, error)     { return "", nil }
func (m *mockTunnel) Error() error                      { return nil }
func (m *mockTunnel) LoadConfig(path string) error      { return nil }
func (m *mockTunnel) Status() i2ptunnel.I2PTunnelStatus { return m.status }
func (m *mockTunnel) Options() map[string]string        { return m.options }
func (m *mockTunnel) SetOptions(opts map[string]string) error {
	for k, v := range opts {
		m.options[k] = v
	}
	return nil
}
