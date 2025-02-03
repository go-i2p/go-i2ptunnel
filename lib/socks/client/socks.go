package socks

/**
 *
**/

import i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"

var implementSOCKS i2ptunnel.I2PTunnel = &SOCKS{}

type SOCKS struct {
}

// Address implements i2ptunnel.I2PTunnel.
func (s *SOCKS) Address() string {
	panic("unimplemented")
}

// Error implements i2ptunnel.I2PTunnel.
func (s *SOCKS) Error() error {
	panic("unimplemented")
}

// LocalAddress implements i2ptunnel.I2PTunnel.
func (s *SOCKS) LocalAddress() (string, string, error) {
	panic("unimplemented")
}

// Name implements i2ptunnel.I2PTunnel.
func (s *SOCKS) Name() string {
	panic("unimplemented")
}

// Options implements i2ptunnel.I2PTunnel.
func (s *SOCKS) Options() map[string]string {
	panic("unimplemented")
}

// Start implements i2ptunnel.I2PTunnel.
func (s *SOCKS) Start() error {
	panic("unimplemented")
}

// Status implements i2ptunnel.I2PTunnel.
func (s *SOCKS) Status() i2ptunnel.I2PTunnelStatus {
	panic("unimplemented")
}

// Stop implements i2ptunnel.I2PTunnel.
func (s *SOCKS) Stop() error {
	panic("unimplemented")
}

// Target implements i2ptunnel.I2PTunnel.
func (s *SOCKS) Target() string {
	panic("unimplemented")
}

// Type implements i2ptunnel.I2PTunnel.
func (s *SOCKS) Type() string {
	panic("unimplemented")
}
