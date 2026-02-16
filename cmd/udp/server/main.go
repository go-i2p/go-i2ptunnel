// UDP server tunnel CLI.
// Starts an I2P datagram listener that forwards incoming packets to a local UDP service.
//
// Usage:
//
//	go-i2ptunnel-udpserver -config tunnel.yaml [-sam 127.0.0.1:7656]
package main

import "github.com/go-i2p/go-i2ptunnel/cmd/shared"

func main() {
	shared.Run("UDP server")
}
