package controller

import (
	"net/http"
)

/*
*
Controller is a tunnel controller GUI for a single I2P Tunnel.
It manages a single i2ptunnel.I2PTunnel, and can accept any implementation of that interface.
It has no specific behaviors for any tunnel type.
It uses ../templates/i2ptunnelcontrol.html as an HTML template for the control page.
It uses ../templates/i2ptunnelminicontrol.html as an HTML template for the home page.
*/
type Controller struct {
	*Config
}

func (c *Controller) ServeHTTP(w http.ResponseWriter, r *http.Request) {

}

func (c *Controller) MiniServeHTTP(w http.ResponseWriter, r *http.Request) {

}

func NewController(yamlFile string) (*Controller, error) {
	cfg, err := NewConfig(yamlFile)
	if err != nil {
		return nil, err
	}
	c := &Controller{
		Config: cfg,
	}
	return c, err
}
