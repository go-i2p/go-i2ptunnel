// HTTP bidirectional (P2P) tunnel CLI.
// Starts a combined HTTP client proxy and HTTP server on a single I2P identity.
// Inbound I2P connections are forwarded to a local HTTP service (target),
// while a local HTTP proxy port allows outbound requests through I2P.
//
// Usage:
//
//	go-i2ptunnel-httpbidirectional -config tunnel.yaml [-sam 127.0.0.1:7656]
package main

import "github.com/go-i2p/go-i2ptunnel/cmd/shared"

func main() {
	shared.Run("HTTP bidirectional")
}
