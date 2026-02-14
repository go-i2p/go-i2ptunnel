package udpbidirectional

import (
	"context"
	"fmt"
	"net"

	"github.com/go-i2p/go-forward/config"
	"github.com/go-i2p/go-forward/packet"
	"github.com/go-i2p/go-forward/stream"
	"github.com/go-i2p/go-sam-go/datagram"
	"github.com/go-i2p/i2pkeys"
	"github.com/go-i2p/onramp"
	"github.com/txthinking/socks5"
)

// socksHandler implements socks5.Handler using the shared Garlic instance.
type socksHandler struct {
	garlic *onramp.Garlic
}

var _ socks5.Handler = (*socksHandler)(nil)

// TCPHandle connects to the requested I2P destination via TCP and forwards data.
func (h *socksHandler) TCPHandle(_ *socks5.Server, conn *net.TCPConn, req *socks5.Request) error {
	i2pConn, err := h.garlic.Dial("tcp", req.Address())
	if err != nil {
		return err
	}
	defer i2pConn.Close()

	ctx := context.Background()
	return stream.Forward(ctx, conn, i2pConn, config.DefaultConfig())
}

// UDPHandle connects to the requested I2P destination via UDP and forwards datagrams.
func (h *socksHandler) UDPHandle(_ *socks5.Server, addr *net.UDPAddr, data *socks5.Datagram) error {
	i2pConn, err := h.garlic.Dial("udp", data.Address())
	if err != nil {
		return err
	}
	defer i2pConn.Close()

	remoteAddr, err := i2pkeys.Lookup(data.Address())
	if err != nil {
		return fmt.Errorf("failed to lookup remote address: %w", err)
	}

	if _, err := i2pConn.(*datagram.DatagramSession).WriteTo(data.Data, remoteAddr); err != nil {
		return fmt.Errorf("failed to write initial datagram: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx := context.Background()
	return packet.Forward(ctx, conn, i2pConn.(*datagram.DatagramSession), config.DefaultConfig())
}
