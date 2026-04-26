// Package shared provides a common tunnel runner for all CLI entry points.
// It handles flag parsing, LeaseSet credential prompting, and delegates
// lifecycle management (start, stop, signal handling, hot reload) to the
// lib/embedding package.
package shared

import (
	"flag"
	"fmt"
	"os"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/embedding"
)

// Run is the main entry point for all tunnel CLI tools. It parses flags,
// loads the tunnel configuration, prompts for LeaseSet credentials when
// required, and blocks until shutdown.
//
// tunnelType is a human-readable label for log messages (e.g., "HTTP client").
// The actual tunnel type is determined by the config file contents.
func Run(tunnelType string) {
	configPath := flag.String("config", "", "Path to tunnel configuration file (required)")
	samAddr := flag.String("sam", "127.0.0.1:7656", "SAM bridge address (host:port)")
	metricsAddr := flag.String("metrics-addr", "", "Address to serve Prometheus metrics (e.g. :9090); disabled if empty")
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintf(os.Stderr, "Error: -config flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -config <path> [-sam <host:port>] [-metrics-addr <host:port>]\n", os.Args[0])
		os.Exit(1)
	}

	opts := []embedding.Option{
		embedding.WithSAMAddr(*samAddr),
	}
	if *metricsAddr != "" {
		opts = append(opts, embedding.WithMetricsAddr(*metricsAddr))
	}

	t, err := embedding.FromConfigFile(*configPath, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Setup error: %v\n", err)
		os.Exit(1)
	}

	// CLI-specific: prompt for encrypted LeaseSet credentials from stdin.
	if err := promptLeaseSetCredential(t.Tunnel(), os.Stdin, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "LeaseSet credential error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Starting %s tunnel %q on %s\n", tunnelType, t.Name(), localAddr(t.Tunnel()))

	if err := t.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Tunnel error: %v\n", err)
		os.Exit(1)
	}
}

// localAddr safely gets the tunnel's local address for CLI display.
func localAddr(tunnel i2ptunnel.I2PTunnel) string {
	addr, err := tunnel.LocalAddress()
	if err != nil {
		return "(unknown)"
	}
	return addr
}
