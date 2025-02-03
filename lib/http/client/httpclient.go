package httpclient

/**
 *
**/

import i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"

var implementHTTPClient i2ptunnel.I2PTunnel = &HTTPClient{}

type HTTPClient struct {
}

// Address implements i2ptunnel.I2PTunnel.
func (h *HTTPClient) Address() string {
	panic("unimplemented")
}

// Error implements i2ptunnel.I2PTunnel.
func (h *HTTPClient) Error() error {
	panic("unimplemented")
}

// LocalAddress implements i2ptunnel.I2PTunnel.
func (h *HTTPClient) LocalAddress() (string, string, error) {
	panic("unimplemented")
}

// Name implements i2ptunnel.I2PTunnel.
func (h *HTTPClient) Name() string {
	panic("unimplemented")
}

// Options implements i2ptunnel.I2PTunnel.
func (h *HTTPClient) Options() map[string]string {
	panic("unimplemented")
}

// Start implements i2ptunnel.I2PTunnel.
func (h *HTTPClient) Start() error {
	panic("unimplemented")
}

// Status implements i2ptunnel.I2PTunnel.
func (h *HTTPClient) Status() i2ptunnel.I2PTunnelStatus {
	panic("unimplemented")
}

// Stop implements i2ptunnel.I2PTunnel.
func (h *HTTPClient) Stop() error {
	panic("unimplemented")
}

// Target implements i2ptunnel.I2PTunnel.
func (h *HTTPClient) Target() string {
	panic("unimplemented")
}

// Type implements i2ptunnel.I2PTunnel.
func (h *HTTPClient) Type() string {
	panic("unimplemented")
}
