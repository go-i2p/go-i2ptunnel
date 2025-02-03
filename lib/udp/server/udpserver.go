package udpserver

/**
 *
**/

import i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"

var implementUDPServer i2ptunnel.I2PTunnel = &UDPServer{}

type UDPServer struct {
}

// Address implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Address() string {
	panic("unimplemented")
}

// Error implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Error() error {
	panic("unimplemented")
}

// LocalAddress implements i2ptunnel.I2PTunnel.
func (u *UDPServer) LocalAddress() (string, string, error) {
	panic("unimplemented")
}

// Name implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Name() string {
	panic("unimplemented")
}

// Options implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Options() map[string]string {
	panic("unimplemented")
}

// Start implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Start() error {
	panic("unimplemented")
}

// Status implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Status() i2ptunnel.I2PTunnelStatus {
	panic("unimplemented")
}

// Stop implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Stop() error {
	panic("unimplemented")
}

// Target implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Target() string {
	panic("unimplemented")
}

// Type implements i2ptunnel.I2PTunnel.
func (u *UDPServer) Type() string {
	panic("unimplemented")
}
