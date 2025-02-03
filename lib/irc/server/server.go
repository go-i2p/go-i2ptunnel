package ircserver

/**
 *
**/

import i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"

var implementIRCServer i2ptunnel.I2PTunnel = &IRCServer{}

type IRCServer struct {
}

// Address implements i2ptunnel.I2PTunnel.
func (i *IRCServer) Address() string {
	panic("unimplemented")
}

// Error implements i2ptunnel.I2PTunnel.
func (i *IRCServer) Error() error {
	panic("unimplemented")
}

// LocalAddress implements i2ptunnel.I2PTunnel.
func (i *IRCServer) LocalAddress() (string, string, error) {
	panic("unimplemented")
}

// Name implements i2ptunnel.I2PTunnel.
func (i *IRCServer) Name() string {
	panic("unimplemented")
}

// Options implements i2ptunnel.I2PTunnel.
func (i *IRCServer) Options() map[string]string {
	panic("unimplemented")
}

// Start implements i2ptunnel.I2PTunnel.
func (i *IRCServer) Start() error {
	panic("unimplemented")
}

// Status implements i2ptunnel.I2PTunnel.
func (i *IRCServer) Status() i2ptunnel.I2PTunnelStatus {
	panic("unimplemented")
}

// Stop implements i2ptunnel.I2PTunnel.
func (i *IRCServer) Stop() error {
	panic("unimplemented")
}

// Target implements i2ptunnel.I2PTunnel.
func (i *IRCServer) Target() string {
	panic("unimplemented")
}

// Type implements i2ptunnel.I2PTunnel.
func (i *IRCServer) Type() string {
	panic("unimplemented")
}
