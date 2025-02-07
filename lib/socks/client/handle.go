package socks

import (
	"net"

	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/packet"
	"github.com/go-i2p/go-forward/stream"
	"github.com/txthinking/socks5"
)

var socksHandler socks5.Handler = &SOCKS{}

// TCPHandle implements socks5.Handler.
func (s *SOCKS) TCPHandle(_ *socks5.Server, conn *net.TCPConn, req *socks5.Request) error {
	// Connect to destination through I2P
	i2pConn, err := s.Garlic.Dial("tcp", req.Address())
	if err != nil {
		return err
	}
	defer i2pConn.Close()

	// Forward data between TCP and I2P connections
	err = stream.Forward(nil, conn, i2pConn, config.DefaultConfig())
	return err
}

// UDPHandle implements socks5.Handler.
func (s *SOCKS) UDPHandle(_ *socks5.Server, addr *net.UDPAddr, d *socks5.Datagram) error {
	// Connect to destination through I2P
	i2pConn, err := s.Garlic.DialRemote("udp", d.Address())
	if err != nil {
		return err
	}
	defer i2pConn.Close()

	// Create UDP connection back to client
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Forward packets between UDP and I2P connections
	err = packet.Forward(nil, conn, i2pConn, config.DefaultConfig())
	return err
}
