package tcpserver

import (
	"net"
	"strconv"
	"strings"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	limitedlistener "github.com/go-i2p/go-limit"
	"github.com/go-i2p/onramp"
)

// NewTCPServer creates a new TCP Server tunnel with the given configuration
func NewTCPServer(config i2pconv.TunnelConfig, samAddr string) (*TCPServer, error) {
	keys, options, err := config.SAMTunnel()
	if err != nil {
		return nil, err
	}
	name := strings.ReplaceAll(config.Name, " ", "_")
	garlic, err := onramp.NewGarlic(name, samAddr, options)
	if err != nil {
		return nil, err
	}
	garlic.ServiceKeys = keys
	localPort := strconv.Itoa(config.Port)
	localAddr := net.JoinHostPort(config.Interface, localPort)
	addr, err := net.ResolveTCPAddr("tcp", localAddr)
	if err != nil {
		return nil, err
	}
	return &TCPServer{
		TunnelConfig:    config,
		Garlic:          garlic,
		Addr:            addr,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		LimitedConfig: limitedlistener.LimitedConfig{
			MaxConns:  1000,
			RateLimit: 100,
		},
		done: make(chan struct{}),
	}, nil
}
