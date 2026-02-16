// SOCKS5 client proxy tunnel CLI.
// Starts a SOCKS5 proxy listening on a local port, routing TCP and UDP traffic
// through the I2P network.
//
// Usage:
//
//	go-i2ptunnel-socksclient -config tunnel.yaml [-sam 127.0.0.1:7656]
package main

import "github.com/go-i2p/go-i2ptunnel/cmd/shared"

func main() {
	shared.Run("SOCKS client")
}
