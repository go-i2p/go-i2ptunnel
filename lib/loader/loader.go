package loader

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	httpclient "github.com/go-i2p/go-i2ptunnel/lib/http/client"
	httpserver "github.com/go-i2p/go-i2ptunnel/lib/http/server"
	ircclient "github.com/go-i2p/go-i2ptunnel/lib/irc/client"
	ircserver "github.com/go-i2p/go-i2ptunnel/lib/irc/server"
	socks "github.com/go-i2p/go-i2ptunnel/lib/socks/client"
	tcpclient "github.com/go-i2p/go-i2ptunnel/lib/tcp/client"
	tcpserver "github.com/go-i2p/go-i2ptunnel/lib/tcp/server"
	udpclient "github.com/go-i2p/go-i2ptunnel/lib/udp/client"
	udpserver "github.com/go-i2p/go-i2ptunnel/lib/udp/server"
)

var DEFAULT_SAM_ADDRESS = "localhost:7656"

func validateHost(samAddr ...string) string {
	samhost := DEFAULT_SAM_ADDRESS
	switch len(samAddr) {
	case 0:
		samhost = DEFAULT_SAM_ADDRESS
	case 1:
		host, port, err := net.SplitHostPort(samAddr[0])
		if err != nil {
			log.Println("Invalid SAM port number, trying the default", err)
			return DEFAULT_SAM_ADDRESS
		}
		samhost = net.JoinHostPort(host, port)
	case 2:
		host := samAddr[0]
		port, err := strconv.Atoi(samAddr[1])
		if err != nil {
			log.Println("Invalid SAM port number, trying the default", err)
			return DEFAULT_SAM_ADDRESS
		}
		samhost = net.JoinHostPort(host, strconv.Itoa(port))
	default:
		return DEFAULT_SAM_ADDRESS
	}
	return samhost
}

// using go-i2ptunnel-config's Converter, load the config file and return an I2PTunnel matching the configured tunnel type
func Load(path string, samAddr ...string) (i2ptunnel.I2PTunnel, error) {
	samhost := validateHost(samAddr...)

	conv := i2pconv.Converter{}
	format, err := conv.DetectFormat(path)
	if err != nil {
		return nil, err
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	tunnel, err := conv.ParseInput(bytes, format)
	if err != nil {
		return nil, err
	}
	switch tunnel.Type {
	case "tcpclient":
		return tcpclient.NewTCPClient(*tunnel, samhost)
	case "tcpserver":
		return tcpserver.NewTCPServer(*tunnel, samhost)
	case "udpclient":
		return udpclient.NewUDPClient(*tunnel, samhost)
	case "udpserver":
		return udpserver.NewUDPServer(*tunnel, samhost)
	case "socks":
		return socks.NewSocksClient(*tunnel, samhost)
	case "httpclient":
		return httpclient.NewHTTPClient(*tunnel, samhost)
	case "httpserver":
		return httpserver.NewHTTPServer(*tunnel, samhost)
	case "ircclient":
		return ircclient.NewIRCClient(*tunnel, samhost)
	case "ircserver":
		return ircserver.NewIRCServer(*tunnel, samhost)
	default:
		return nil, fmt.Errorf("unknown tunnel type: %s", tunnel.Type)
	}
}
