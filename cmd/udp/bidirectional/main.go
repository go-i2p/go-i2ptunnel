// UDP bidirectional (P2P) tunnel CLI.
// Starts a combined UDP client and UDP server on a single I2P identity.
// Inbound I2P datagrams are forwarded to a local UDP service (target),
// while a local SOCKS5 proxy port allows outbound datagrams through I2P.
//
// Usage:
//
//	go-i2ptunnel-udpbidirectional -config tunnel.yaml [-sam 127.0.0.1:7656]
package main

import "github.com/go-i2p/go-i2ptunnel/cmd/shared"

func main() {
	shared.Run("UDP bidirectional")
}
