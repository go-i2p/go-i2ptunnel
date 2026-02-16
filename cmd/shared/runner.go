// Package shared provides a common tunnel runner for all CLI entry points.
// It handles configuration loading, signal-based graceful shutdown (SIGINT/SIGTERM),
// and hot configuration reload via SIGHUP.
//
// Why: All 9 tunnel CLIs need identical lifecycle management. A shared runner
// eliminates duplication and ensures consistent behavior across tunnel types.
//
// Design: Uses the existing loader.Load() function to create tunnels from config
// files. Signal handling uses os/signal with a dedicated goroutine. SIGHUP triggers
// stop -> LoadConfig -> start cycle for zero-downtime config updates.
package shared

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/loader"
)

// Run is the main entry point for all tunnel CLI tools. It parses flags,
// loads the tunnel configuration, starts the tunnel, and handles signals.
//
// tunnelType is a human-readable label for log messages (e.g., "HTTP client").
// The actual tunnel type is determined by the config file contents.
func Run(tunnelType string) {
	configPath := flag.String("config", "", "Path to tunnel configuration file (required)")
	samAddr := flag.String("sam", "127.0.0.1:7656", "SAM bridge address (host:port)")
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintf(os.Stderr, "Error: -config flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -config <path> [-sam <host:port>]\n", os.Args[0])
		os.Exit(1)
	}

	tunnel, err := loader.Load(*configPath, *samAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load %s tunnel config: %v\n", tunnelType, err)
		os.Exit(1)
	}

	fmt.Printf("Starting %s tunnel %q on %s\n", tunnelType, tunnel.Name(), localAddr(tunnel))

	if err := startAndWait(tunnel, tunnelType, *configPath, *samAddr); err != nil {
		fmt.Fprintf(os.Stderr, "Tunnel error: %v\n", err)
		os.Exit(1)
	}
}

// startAndWait starts the tunnel and blocks until a shutdown signal is received.
// SIGHUP triggers a config reload cycle. SIGINT/SIGTERM trigger graceful shutdown.
func startAndWait(tunnel i2ptunnel.I2PTunnel, tunnelType, configPath, samAddr string) error {
	// Start tunnel in a goroutine since Start() may block (e.g., SOCKS ListenAndServe)
	errCh := make(chan error, 1)
	go func() {
		errCh <- tunnel.Start()
	}()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		select {
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGHUP:
				fmt.Printf("Received SIGHUP, reloading %s tunnel config...\n", tunnelType)
				reloaded, err := reload(tunnel, tunnelType, configPath, samAddr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Config reload failed: %v (keeping current config)\n", err)
					continue
				}
				// Replace tunnel reference and restart the error listener
				tunnel = reloaded
				go func() {
					errCh <- tunnel.Start()
				}()
				fmt.Printf("Tunnel %q reloaded successfully\n", tunnel.Name())

			case syscall.SIGINT, syscall.SIGTERM:
				fmt.Printf("\nReceived %s, shutting down %s tunnel %q...\n", sig, tunnelType, tunnel.Name())
				if err := tunnel.Stop(); err != nil {
					return fmt.Errorf("error during shutdown: %w", err)
				}
				fmt.Println("Tunnel stopped cleanly")
				return nil
			}

		case err := <-errCh:
			// Start() returned — either it finished or errored
			if err != nil {
				return fmt.Errorf("%s tunnel error: %w", tunnelType, err)
			}
			// Some tunnels return nil from Start() after setup (non-blocking).
			// Keep waiting for signals in that case.
		}
	}
}

// reload performs a stop -> LoadConfig -> start cycle for hot config reload.
// If LoadConfig fails, it attempts to reload from the original config file
// by creating a fresh tunnel instance.
func reload(tunnel i2ptunnel.I2PTunnel, tunnelType, configPath, samAddr string) (i2ptunnel.I2PTunnel, error) {
	// Stop the current tunnel
	if err := tunnel.Stop(); err != nil {
		return nil, fmt.Errorf("failed to stop tunnel for reload: %w", err)
	}

	// Try in-place config reload first
	if err := tunnel.LoadConfig(configPath); err != nil {
		// Fall back to creating a fresh tunnel from the config file
		fmt.Fprintf(os.Stderr, "In-place reload failed (%v), creating fresh tunnel...\n", err)
		fresh, loadErr := loader.Load(configPath, samAddr)
		if loadErr != nil {
			return nil, fmt.Errorf("failed to load new config: %w", loadErr)
		}
		return fresh, nil
	}

	return tunnel, nil
}

// localAddr safely gets the tunnel's local address for display.
func localAddr(tunnel i2ptunnel.I2PTunnel) string {
	addr, err := tunnel.LocalAddress()
	if err != nil {
		return "(unknown)"
	}
	return addr
}
