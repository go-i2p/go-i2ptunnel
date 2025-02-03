package i2ptunnel

import "fmt"

type I2PTunnelError struct {
	errorString string
}

func (i I2PTunnelError) Error() string {
	return i.errorString
}

var exampleError error = I2PTunnelError{}

func NewError(tun I2PTunnel, err error) I2PTunnelError {
	details := fmt.Sprintf("Name:%s\n\tError:%s\n", tun.Name(), err)
	return I2PTunnelError{
		errorString: details,
	}
}
