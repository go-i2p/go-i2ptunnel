// Package embedding provides a high-level API for embedding I2P tunnels inside
// Go applications. It wraps the lower-level lib/core, lib/loader, and
// lib/metrics packages behind a single Tunnel type that supports:
//
//   - Construction from a YAML/TOML config file (FromConfigFile) or from any
//     existing I2PTunnel implementation (Wrap).
//   - Functional options (WithSAMAddr, WithMetricsAddr, WithLeaseSetKey) to
//     configure behaviour without sub-typing.
//   - Simple lifecycle: Start / Stop / Reload / Run.
//   - Optional Prometheus metrics, health, and status HTTP endpoints.
//   - Signal-aware Run that handles SIGINT/SIGTERM (shutdown) and SIGHUP
//     (zero-downtime config reload) so the caller only needs to call Run().
//
// Typical usage — config file:
//
//	t, err := embedding.FromConfigFile("tunnel.yaml",
//	    embedding.WithSAMAddr("127.0.0.1:7656"),
//	    embedding.WithMetricsAddr(":9090"),
//	)
//	if err != nil { log.Fatal(err) }
//	log.Fatal(t.Run())
//
// Typical usage — existing tunnel:
//
//	myTunnel := tcpserver.NewTCPServer(cfg, samAddr)
//	t, err := embedding.Wrap(myTunnel)
//	if err != nil { log.Fatal(err) }
//	if err := t.Start(); err != nil { log.Fatal(err) }
//	defer t.Stop()
package embedding

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/loader"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
)

// Tunnel wraps an I2PTunnel with lifecycle management, optional Prometheus
// metrics, and signal-based graceful shutdown / hot reload. It is the primary
// embedding surface for application developers.
type Tunnel struct {
	tunnel     i2ptunnel.I2PTunnel
	configPath string // path used for Reload(); empty when built via Wrap
	samAddr    string
	registry   *metrics.Registry
}

// FromConfigFile loads a tunnel from a YAML/TOML config file and applies
// the given options. It is the primary constructor for embedded tunnels.
func FromConfigFile(path string, opts ...Option) (*Tunnel, error) {
	cfg := defaults()
	for _, o := range opts {
		o(cfg)
	}

	tunnel, err := loader.Load(path, cfg.samAddr)
	if err != nil {
		return nil, fmt.Errorf("embedding: failed to load tunnel from %q: %w", path, err)
	}

	return build(tunnel, path, cfg)
}

// Wrap creates a Tunnel from an already-constructed I2PTunnel. Use this when
// you build the tunnel directly rather than from a config file. Reload will
// be a no-op for tunnels constructed this way (no config path is known).
func Wrap(tunnel i2ptunnel.I2PTunnel, opts ...Option) (*Tunnel, error) {
	cfg := defaults()
	for _, o := range opts {
		o(cfg)
	}
	return build(tunnel, "", cfg)
}

// build is the shared construction helper used by both FromConfigFile and Wrap.
func build(tunnel i2ptunnel.I2PTunnel, configPath string, cfg *tunnelConfig) (*Tunnel, error) {
	if cfg.leaseSetKey != "" {
		if err := tunnel.SetOptions(map[string]string{"i2cp.leaseSetPrivKey": cfg.leaseSetKey}); err != nil {
			return nil, fmt.Errorf("embedding: failed to set leaseSetPrivKey: %w", err)
		}
	}

	registry := metrics.NewRegistry()
	m := registry.Register(tunnel.Name(), tunnel.ID(), tunnel.Type())
	if bearer, ok := tunnel.(metrics.MetricsBearer); ok {
		bearer.SetTunnelMetrics(m)
	}

	t := &Tunnel{
		tunnel:     tunnel,
		configPath: configPath,
		samAddr:    cfg.samAddr,
		registry:   registry,
	}

	if cfg.metricsAddr != "" {
		t.startMetricsServer(cfg.metricsAddr)
	}

	return t, nil
}

// Tunnel returns the underlying I2PTunnel. Use this to access
// tunnel-type-specific methods not exposed by the embedding API,
// or to pass the tunnel to functions that accept I2PTunnel.
func (t *Tunnel) Tunnel() i2ptunnel.I2PTunnel { return t.tunnel }

// Name returns the tunnel's human-readable name.
func (t *Tunnel) Name() string { return t.tunnel.Name() }

// LocalAddress returns the tunnel's local listening address.
func (t *Tunnel) LocalAddress() (string, error) { return t.tunnel.LocalAddress() }

// Start starts the underlying tunnel. It blocks until the tunnel exits or
// encounters a fatal error. Call it in a goroutine for non-blocking use.
func (t *Tunnel) Start() error { return t.tunnel.Start() }

// Stop shuts down the tunnel and releases its resources.
func (t *Tunnel) Stop() error { return t.tunnel.Stop() }

// Reload performs a stop → LoadConfig → start cycle for zero-downtime config
// updates. If in-place LoadConfig fails, or no config path was recorded (tunnel
// was built via Wrap), it attempts to create a fresh tunnel from the config
// file. Returns an error if the tunnel cannot be restarted.
func (t *Tunnel) Reload() error {
	if err := t.tunnel.Stop(); err != nil {
		return fmt.Errorf("embedding: reload stop failed: %w", err)
	}

	if t.configPath == "" {
		// No config file path known; nothing to reload from.
		return nil
	}

	if err := t.tunnel.LoadConfig(t.configPath); err != nil {
		log.WithError(err).Warn("In-place reload failed, creating fresh tunnel from config file")
		fresh, loadErr := loader.Load(t.configPath, t.samAddr)
		if loadErr != nil {
			return fmt.Errorf("embedding: reload failed to load fresh config: %w", loadErr)
		}
		t.tunnel = fresh
	}

	return nil
}

// Run starts the tunnel and blocks until SIGINT or SIGTERM is received.
// SIGHUP triggers a zero-downtime Reload cycle. This method is intended as the
// last call in main() for embedded tunnel programs. It returns nil on a clean
// shutdown and an error if the tunnel itself fails.
func (t *Tunnel) Run() error {
	errCh := make(chan error, 1)
	go func() { errCh <- t.tunnel.Start() }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigCh)

	reloading := false
	for {
		select {
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGHUP:
				if reloading {
					log.Warn("Reload already in progress, ignoring SIGHUP")
					continue
				}
				reloading = true
				if err := t.Reload(); err != nil {
					log.WithError(err).Warn("Config reload failed, keeping current config")
					reloading = false
					continue
				}
				reloading = false
				go func() { errCh <- t.tunnel.Start() }()

			case syscall.SIGINT, syscall.SIGTERM:
				return t.tunnel.Stop()
			}

		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("embedding: tunnel error: %w", err)
			}
			// Start() returned nil: the tunnel exited cleanly (e.g. was stopped
			// by a previous Reload or Stop call). Continue the loop so that any
			// subsequent signals (e.g. SIGTERM after a SIGHUP reload) are handled.
		}
	}
}

// startMetricsServer starts Prometheus metrics, health, and status endpoints
// on addr in a background goroutine. Errors binding the listener are logged
// but do not stop the tunnel.
func (t *Tunnel) startMetricsServer(addr string) {
	tunnel := t.tunnel
	statusFunc := func() []metrics.TunnelStatus {
		localAddr, _ := tunnel.LocalAddress()
		errMsg := ""
		if err := tunnel.Error(); err != nil {
			errMsg = err.Error()
		}
		return []metrics.TunnelStatus{{
			Name:         tunnel.Name(),
			ID:           tunnel.ID(),
			Type:         tunnel.Type(),
			Status:       string(tunnel.Status()),
			Address:      tunnel.Address(),
			Target:       tunnel.Target(),
			LocalAddress: localAddr,
			Error:        errMsg,
		}}
	}

	handler := metrics.NewHandler(t.registry, statusFunc)
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", handler.HandleMetrics)
	mux.HandleFunc("/healthz", handler.HandleHealth)
	mux.HandleFunc("/api/status", handler.HandleStatus)

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil && err != http.ErrServerClosed {
			log.WithError(err).WithField("addr", addr).Warn("Metrics server error")
		}
	}()
}
