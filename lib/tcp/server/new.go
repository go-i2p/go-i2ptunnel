package tcpserver

import (
	"strings"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
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
	return &TCPServer{
		TunnelConfig: config,
		Garlic:       garlic,
	}, nil
}
