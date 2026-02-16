// HTTP server tunnel CLI.
// Starts an I2P listener that forwards incoming connections to a local HTTP server.
//
// Usage:
//
//	go-i2ptunnel-httpserver -config tunnel.yaml [-sam 127.0.0.1:7656]
package main

import "github.com/go-i2p/go-i2ptunnel/cmd/shared"

func main() {
	shared.Run("HTTP server")
}
