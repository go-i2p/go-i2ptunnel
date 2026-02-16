// TCP client tunnel CLI.
// Starts a local TCP listener that forwards connections to a specific I2P destination.
//
// Usage:
//
//	go-i2ptunnel-tcpclient -config tunnel.yaml [-sam 127.0.0.1:7656]
package main

import "github.com/go-i2p/go-i2ptunnel/cmd/shared"

func main() {
	shared.Run("TCP client")
}
