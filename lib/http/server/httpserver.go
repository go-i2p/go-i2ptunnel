package httpserver

/**
 *
**/

import i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"

var implementHTTPServer i2ptunnel.I2PTunnel = &HTTPServer{}

type HTTPServer struct {
}

// Address implements i2ptunnel.I2PTunnel.
func (h *HTTPServer) Address() string {
	panic("unimplemented")
}

// Error implements i2ptunnel.I2PTunnel.
func (h *HTTPServer) Error() error {
	panic("unimplemented")
}

// LocalAddress implements i2ptunnel.I2PTunnel.
func (h *HTTPServer) LocalAddress() (string, string, error) {
	panic("unimplemented")
}

// Name implements i2ptunnel.I2PTunnel.
func (h *HTTPServer) Name() string {
	panic("unimplemented")
}

// Options implements i2ptunnel.I2PTunnel.
func (h *HTTPServer) Options() map[string]string {
	panic("unimplemented")
}

// Start implements i2ptunnel.I2PTunnel.
func (h *HTTPServer) Start() error {
	panic("unimplemented")
}

// Status implements i2ptunnel.I2PTunnel.
func (h *HTTPServer) Status() i2ptunnel.I2PTunnelStatus {
	panic("unimplemented")
}

// Stop implements i2ptunnel.I2PTunnel.
func (h *HTTPServer) Stop() error {
	panic("unimplemented")
}

// Target implements i2ptunnel.I2PTunnel.
func (h *HTTPServer) Target() string {
	panic("unimplemented")
}

// Type implements i2ptunnel.I2PTunnel.
func (h *HTTPServer) Type() string {
	panic("unimplemented")
}
