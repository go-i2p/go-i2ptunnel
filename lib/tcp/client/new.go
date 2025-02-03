package tcpclient

import (
	"strings"

	"github.com/go-i2p/i2pkeys"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	"github.com/go-i2p/onramp"
)

// NewTCPClient creates a new TCP Client tunnel with the given configuration
func NewTCPClient(config i2pconv.TunnelConfig, samAddr string) (*TCPClient, error) {
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
	addr, err := i2pkeys.Lookup(config.Target)
	if err != nil {
		return nil, err
	}
	return &TCPClient{
		TunnelConfig: config,
		Garlic:       garlic,
		I2PAddr:      addr,
	}, nil
}
