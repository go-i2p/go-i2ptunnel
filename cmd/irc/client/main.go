// IRC client tunnel CLI.
// Starts a local proxy that forwards IRC connections through I2P with DCC blocking
// and privacy filtering.
//
// Usage:
//
//	go-i2ptunnel-ircclient -config tunnel.yaml [-sam 127.0.0.1:7656]
package main

import "github.com/go-i2p/go-i2ptunnel/cmd/shared"

func main() {
	shared.Run("IRC client")
}
