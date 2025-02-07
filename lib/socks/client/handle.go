package socks

import (
	"context"
	"fmt"
	"net"

	"github.com/go-i2p/go-forward/packet"
	"github.com/go-i2p/go-forward/stream"
	udpconst "github.com/go-i2p/go-i2ptunnel/lib/udp/const"
	"github.com/go-i2p/i2pkeys"
	"github.com/txthinking/socks5"
)

var (
	socksHandler  socks5.Handler = &SOCKS{}
	forwardConfig                = udpconst.DatagramForwardConfig
)

// TCPHandle implements socks5.Handler.
func (s *SOCKS) TCPHandle(_ *socks5.Server, conn *net.TCPConn, req *socks5.Request) error {
	// Connect to destination through I2P
	i2pConn, err := s.Garlic.Dial("tcp", req.Address())
	if err != nil {
		return err
	}
	defer i2pConn.Close()

	ctx := context.Background()
	err = stream.Forward(ctx, conn, i2pConn, forwardConfig)
	return err
}

// UDPHandle implements socks5.Handler.
func (s *SOCKS) UDPHandle(_ *socks5.Server, addr *net.UDPAddr, data *socks5.Datagram) error {
	// Connect to destination through I2P
	i2pConn, err := s.Garlic.DialRemote("udp", data.Address())
	if err != nil {
		return err
	}
	defer i2pConn.Close()

	remoteAddr, err := i2pkeys.Lookup(data.Address())
	if err != nil {
		return fmt.Errorf("failed to lookup remote address: %w", err)
	}

	// Send initial datagram
	if _, err := i2pConn.WriteTo(data.Data, remoteAddr); err != nil {
		return fmt.Errorf("failed to write initial datagram: %w", err)
	}

	// Set up UDP connection for replies
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Forward subsequent packets
	ctx := context.Background()
	return packet.Forward(ctx, conn, i2pConn, forwardConfig)
}
