package socks

import (
	"context"
	"fmt"
	"net"

	"github.com/go-i2p/go-forward/packet"
	"github.com/go-i2p/go-forward/stream"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
	udpconst "github.com/go-i2p/go-i2ptunnel/lib/udp/const"
	"github.com/go-i2p/go-sam-go/datagram"
	"github.com/go-i2p/i2pkeys"
	"github.com/txthinking/socks5"
)

var socksHandler socks5.Handler = &SOCKS{}

// TCPHandle implements socks5.Handler.
func (s *SOCKS) TCPHandle(_ *socks5.Server, conn *net.TCPConn, req *socks5.Request) error {
	if err := s.admitConnection(); err != nil {
		return err
	}
	if s.connSem != nil {
		defer func() { <-s.connSem }()
	}
	if s.Metrics != nil {
		s.Metrics.RecordConnection()
		defer s.Metrics.RecordDisconnection()
	}
	i2pConn, err := s.Garlic.Dial("tcp", req.Address())
	if err != nil {
		if s.Metrics != nil {
			s.Metrics.RecordConnectionFailed()
		}
		return err
	}
	defer i2pConn.Close()
	ctx := context.Background()
	wrapped := metrics.WrapConn(conn, s.Metrics)
	return stream.Forward(ctx, wrapped, i2pConn, udpconst.NewDatagramForwardConfig())
}

// admitConnection enforces rate limiting and concurrency limits.
// For connSem, it only acquires the semaphore slot; the caller is responsible
// for releasing it (via defer <-s.connSem) at connection end.
func (s *SOCKS) admitConnection() error {
	if err := s.checkRateLimit(); err != nil {
		return err
	}
	return s.checkConcurrencyLimit()
}

// checkRateLimit rejects the connection if the rate limiter is saturated.
func (s *SOCKS) checkRateLimit() error {
	if s.rateLimiter != nil && !s.rateLimiter.Allow() {
		if s.Metrics != nil {
			s.Metrics.RecordRateLimitHit()
		}
		s.recordError(fmt.Errorf("connection rejected: rate limit exceeded"))
		return fmt.Errorf("rate limit exceeded")
	}
	return nil
}

// checkConcurrencyLimit rejects the connection if the semaphore is full.
func (s *SOCKS) checkConcurrencyLimit() error {
	if s.connSem != nil {
		select {
		case s.connSem <- struct{}{}:
		default:
			s.recordError(fmt.Errorf("connection rejected: at capacity (%d max concurrent)", cap(s.connSem)))
			return fmt.Errorf("connection limit reached")
		}
	}
	return nil
}

// UDPHandle implements socks5.Handler.
func (s *SOCKS) UDPHandle(_ *socks5.Server, addr *net.UDPAddr, data *socks5.Datagram) error {
	// Connect to destination through I2P
	i2pConn, err := s.Garlic.Dial("udp", data.Address())
	if err != nil {
		if s.Metrics != nil {
			s.Metrics.RecordConnectionFailed()
		}
		return err
	}
	defer i2pConn.Close()

	remoteAddr, err := i2pkeys.Lookup(data.Address())
	if err != nil {
		return fmt.Errorf("failed to lookup remote address: %w", err)
	}

	// Send initial datagram
	if _, err := i2pConn.(*datagram.DatagramSession).WriteTo(data.Data, remoteAddr); err != nil {
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
	return packet.Forward(ctx, conn, i2pConn.(*datagram.DatagramSession), udpconst.NewDatagramForwardConfig())
}
