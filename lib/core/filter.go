package i2ptunnel

import (
	"net"

	filter "github.com/go-i2p/go-connfilter"
)

// TCPFilterConfig holds optional byte-level read/write filters for TCP connections.
// Both fields default to nil, which means no filtering (passthrough).
// Filters follow the contract of filter.FunctionConnFilter: each function receives
// a byte slice and returns a (possibly modified) byte slice or an error.
//
// Use ApplyTCPFilter in handleConnection to wrap a net.Conn when filters are set.
type TCPFilterConfig struct {
	// ReadFilter intercepts bytes arriving from the remote side before forwarding.
	// Returning a non-nil error closes the connection.
	ReadFilter func([]byte) ([]byte, error)
	// WriteFilter intercepts bytes sent to the remote side before transmission.
	// Returning a non-nil error closes the connection.
	WriteFilter func([]byte) ([]byte, error)
}

// IsEnabled reports whether any filter function is configured.
func (f *TCPFilterConfig) IsEnabled() bool {
	return f != nil && (f.ReadFilter != nil || f.WriteFilter != nil)
}

// ApplyTCPFilter wraps conn with the configured read/write filters and returns
// the wrapped connection. If no filters are configured (IsEnabled returns false),
// conn is returned unchanged without allocating a wrapper.
func ApplyTCPFilter(conn net.Conn, cfg *TCPFilterConfig) (net.Conn, error) {
	if !cfg.IsEnabled() {
		return conn, nil
	}
	return filter.NewFunctionConnFilter(conn, cfg.ReadFilter, cfg.WriteFilter)
}
