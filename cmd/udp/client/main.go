// UDP client tunnel CLI.
// Starts a local UDP listener that forwards datagrams to a specific I2P destination.
//
// Usage:
//
//	go-i2ptunnel-udpclient -config tunnel.yaml [-sam 127.0.0.1:7656]
package main

import "github.com/go-i2p/go-i2ptunnel/cmd/shared"

func main() {
	shared.Run("UDP client")
}
