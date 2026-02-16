// TCP server tunnel CLI.
// Starts an I2P listener that forwards incoming connections to a local TCP service.
//
// Usage:
//
//	go-i2ptunnel-tcpserver -config tunnel.yaml [-sam 127.0.0.1:7656]
package main

import "github.com/go-i2p/go-i2ptunnel/cmd/shared"

func main() {
	shared.Run("TCP server")
}
