package udpclient

/**
 *
**/

import i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"

var implementUDPClient i2ptunnel.I2PTunnel = &UDPClient{}

type UDPClient struct {
}

// Address implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Address() string {
	panic("unimplemented")
}

// Error implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Error() error {
	panic("unimplemented")
}

// LocalAddress implements i2ptunnel.I2PTunnel.
func (u *UDPClient) LocalAddress() (string, string, error) {
	panic("unimplemented")
}

// Name implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Name() string {
	panic("unimplemented")
}

// Options implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Options() map[string]string {
	panic("unimplemented")
}

// Start implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Start() error {
	panic("unimplemented")
}

// Status implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Status() i2ptunnel.I2PTunnelStatus {
	panic("unimplemented")
}

// Stop implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Stop() error {
	panic("unimplemented")
}

// Target implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Target() string {
	panic("unimplemented")
}

// Type implements i2ptunnel.I2PTunnel.
func (u *UDPClient) Type() string {
	panic("unimplemented")
}
