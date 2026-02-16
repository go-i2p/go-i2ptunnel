// HTTP client proxy tunnel CLI.
// Starts an HTTP proxy listening on a local port, forwarding requests through I2P.
//
// Usage:
//
//	go-i2ptunnel-httpclient -config tunnel.yaml [-sam 127.0.0.1:7656]
package main

import "github.com/go-i2p/go-i2ptunnel/cmd/shared"

func main() {
	shared.Run("HTTP client")
}
