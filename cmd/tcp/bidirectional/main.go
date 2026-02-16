// TCP bidirectional (P2P) tunnel CLI.
// Starts a combined TCP client and TCP server on a single I2P identity.
// Inbound I2P connections are forwarded to a local TCP service (target),
// while a local SOCKS5 proxy port allows outbound connections through I2P.
//
// Usage:
//
//	go-i2ptunnel-tcpbidirectional -config tunnel.yaml [-sam 127.0.0.1:7656]
package main

import "github.com/go-i2p/go-i2ptunnel/cmd/shared"

func main() {
	shared.Run("TCP bidirectional")
}
