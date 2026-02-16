// IRC server tunnel CLI.
// Starts an I2P listener that forwards incoming connections to a local IRC server
// with admin command blocking and hostname masking.
//
// Usage:
//
//	go-i2ptunnel-ircserver -config tunnel.yaml [-sam 127.0.0.1:7656]
package main

import "github.com/go-i2p/go-i2ptunnel/cmd/shared"

func main() {
	shared.Run("IRC server")
}
