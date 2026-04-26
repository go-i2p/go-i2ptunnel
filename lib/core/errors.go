// Package i2ptunnel provides core types, interfaces, and shared helpers for
// building I2P tunnel services. It defines the I2PTunnel interface that all
// tunnel implementations satisfy, along with configuration helpers, error
// tracking, and option-parsing utilities.
package i2ptunnel

import "fmt"

// I2PTunnelError is an error type that wraps a tunnel-specific error message,
// including the tunnel's name for context when logging or propagating errors.
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
// underlying error message. tun must implement Name() string; passing a full
// I2PTunnel value (or a *TunnelBase) both satisfy this constraint.
func NewError(tun interface{ Name() string }, err error) I2PTunnelError {
	details := fmt.Sprintf("Name:%s\n\tError:%s\n", tun.Name(), err)
	return I2PTunnelError{
		errorString: details,
	}
}
