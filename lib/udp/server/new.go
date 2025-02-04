package udpserver

import (
	"net"
	"strconv"
	"strings"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/onramp"
)

// NewUDPServer creates a new UDP Server tunnel with the given configuration
func NewUDPServer(config i2pconv.TunnelConfig, samAddr string) (*UDPServer, error) {
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
	addr, err := net.ResolveUDPAddr("UDP", localAddr)
	if err != nil {
		return nil, err
	}
	return &UDPServer{
		TunnelConfig:    config,
		Garlic:          garlic,
		Addr:            addr,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		done:            make(chan struct{}),
	}, nil
}
