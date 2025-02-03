package i2ptunnel

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

type I2PTunnel interface {
	// Start the tunnel
	Start() error
	// Stop the tunnel
	Stop() error
	// Get the tunnel's name
	Name() string
	// Get the tunnel's type
	Type() string
	// Get the tunnel's I2P address
	Address() string
	// Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5 and HTTP
	Target() string
	// Get the tunnel's options
	Options() map[string]string
	// Get the tunnel's status
	Status() I2PTunnelStatus
	// Get the tunnel's error message
	Error() error
	// Get the tunnel's local host:port
	LocalAddress() (string, string, error)
}
