package i2ptunnel

import "strings"

// I2PTunnelStatus represents the operational state of an I2P tunnel.
// It is a string type so that status values are human-readable in logs and APIs.
type I2PTunnelStatus string

const (
	// Tunnel is running
	I2PTunnelStatusRunning I2PTunnelStatus = "running"
	// Tunnel is stopped
	I2PTunnelStatusStopped I2PTunnelStatus = "stopped"
	// Tunnel is starting
	I2PTunnelStatusStarting I2PTunnelStatus = "starting"
	// Tunnel is stopping
	I2PTunnelStatusStopping I2PTunnelStatus = "stopping"
	// Tunnel is failed
	I2PTunnelStatusFailed I2PTunnelStatus = "failed"
	// Tunnel is unknown
	I2PTunnelStatusUnknown I2PTunnelStatus = "unknown"
)

// I2PTunnel is the common interface implemented by all 12 tunnel types in this
// package (TCP, HTTP, IRC, UDP, SOCKS5 — each in client, server, and bidirectional
// variants). It provides a uniform lifecycle (Start/Stop), configuration
// (Options/SetOptions/LoadConfig), and observability (Status/Error/Address) API.
type I2PTunnel interface {
	// Start the tunnel
	Start() error
	// Stop the tunnel
	Stop() error
	// Get the tunnel's name
	Name() string
	// Get the tunnel's ID
	ID() string
	// Get the tunnel's type
	Type() string
	// Get the tunnel's I2P address
	Address() string
	// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
	Target() string
	// Get the tunnel's options
	Options() map[string]string
	// Set the tunnel's options
	SetOptions(map[string]string) error
	// Load the tunnel config
	LoadConfig(path string) error
	// Get the tunnel's status
	Status() I2PTunnelStatus
	// Get the tunnel's error message
	Error() error
	// Get the tunnel's local host:port
	LocalAddress() (string, error)
}

// Clean the name to form an ID
// change newlines to +
// change tabs to _
// change spaces to -
// erase foreslashes
func Clean(name string) string {
	// change newlines to +
	// change tabs to _
	// change spaces to -
	// erase foreslashes
	clean := strings.ReplaceAll(name, "\n", "+")
	clean = strings.ReplaceAll(clean, "\t", "_")
	clean = strings.ReplaceAll(clean, " ", "-")
	clean = strings.ReplaceAll(clean, "/", "")
	return clean
}
