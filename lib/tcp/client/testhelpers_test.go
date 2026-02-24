package tcpclient

// testhelpers_test.go provides small utilities shared across the tcpclient test files.

import (
	"net"
	"testing"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
)

const testTarget = "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p"
const testSAMAddr = "localhost:7656"

// minimalConfig returns a stopped TunnelConfig suitable for unit tests that need a
// real TCPClient but do not exercise the SAM connection.
func minimalConfig(name string) i2pconv.TunnelConfig {
	return i2pconv.TunnelConfig{
		Name:      name,
		Type:      "tcpclient",
		Interface: "127.0.0.1",
		Port:      0, // kernel picks a free port when used
		Target:    testTarget,
	}
}

// localPipe creates an in-memory net.Conn pair using net.Pipe().
// Both ends are returned; callers are responsible for closing them.
// The test is marked fatal if the pipe cannot be created.
func localPipe(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	a, b := net.Pipe()
	return a, b
}
