// Package i2ptunnel provides core types, interfaces, and shared helpers for
// building I2P tunnel services. It defines the I2PTunnel interface that all
// tunnel implementations satisfy, along with configuration helpers, error
// tracking, and option-parsing utilities.
package i2ptunnel

import "fmt"

type I2PTunnelError struct {
	errorString string
}

// Error returns the formatted error string.
func (i I2PTunnelError) Error() string {
	return i.errorString
}

// Compile-time interface satisfaction check
var _ error = I2PTunnelError{}

// NewError creates an I2PTunnelError that includes the tunnel name and the
// underlying error message.
func NewError(tun I2PTunnel, err error) I2PTunnelError {
	details := fmt.Sprintf("Name:%s\n\tError:%s\n", tun.Name(), err)
	return I2PTunnelError{
		errorString: details,
	}
}
