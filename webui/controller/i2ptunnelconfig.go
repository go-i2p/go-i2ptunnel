package controller

import (
	"net/http"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

/**
Config is an I2P Tunnel configuration GUI for a single I2P Tunnel.
It manages a single i2ptunnel.I2PTunnel, and can accept any implementation of that interface.
It has no specific behaviors for any tunnel type.
It presents a simple categorial list of I2PTunnel options.
It uses ../templates/i2ptunnelconfig.html as an HTML template
*/

type Config struct {
	i2ptunnel.I2PTunnel
}

func (c *Config) ServeHTTP(w http.ResponseWriter, r *http.Request) {

}

func NewConfig(yamlFile string) (*Config, error) {
	c := &Config{}
	err := c.LoadConfig(yamlFile)
	if err != nil {
		return nil, err
	}
	return c, nil
}
