package embedding

// Option configures a Tunnel at construction time.
type Option func(*tunnelConfig)

type tunnelConfig struct {
	samAddr     string
	metricsAddr string
	leaseSetKey string
}

func defaults() *tunnelConfig {
	return &tunnelConfig{
		samAddr: "127.0.0.1:7656",
	}
}

// WithSAMAddr sets the SAM bridge address used when loading or reloading a
// tunnel from a config file. Defaults to "127.0.0.1:7656".
func WithSAMAddr(addr string) Option {
	return func(c *tunnelConfig) {
		c.samAddr = addr
	}
}

// WithMetricsAddr enables a Prometheus/health/status HTTP server on addr
// (e.g. ":9090"). When empty (the default) no metrics server is started.
func WithMetricsAddr(addr string) Option {
	return func(c *tunnelConfig) {
		c.metricsAddr = addr
	}
}

// WithLeaseSetKey injects an i2cp.leaseSetPrivKey for encrypted LeaseSet
// tunnels. Pass the Base64-encoded private key. This replaces the CLI stdin
// prompt when embedding in an application.
func WithLeaseSetKey(key string) Option {
	return func(c *tunnelConfig) {
		c.leaseSetKey = key
	}
}
