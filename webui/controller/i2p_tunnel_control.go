package controller

import (
	"fmt"
	"net/http"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/webui/templates"
)

/*
*
Controller is a tunnel controller GUI for a single I2P Tunnel.
It manages a single i2ptunnel.I2PTunnel, and can accept any implementation of that interface.
It has no specific behaviors for any tunnel type.
It uses ../templates/i2ptunnelcontrol.html as an HTML template for the control page.
It uses ../templates/i2ptunnelminicontrol.html as an HTML template for the home page.
*/

// Controller is an HTTP handler that extends Config with Start/Stop/Restart actions
// for a single tunnel. It renders a full control page and a compact inline widget
// used on the home page.
type Controller struct {
	*Config
}

// ControlData holds data for rendering control templates
type ControlData struct {
	Name         string
	ID           string
	Type         string
	Status       string
	LocalAddress string
	Address      string
	Target       string
	Options      map[string]string
	Error        string
}

// ServeHTTP handles the full control page (GET displays info, POST handles actions)
func (c *Controller) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c.handleGetControl(w, r)
	case http.MethodPost:
		c.handlePostControl(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// MiniServeHTTP renders the mini control widget for the dashboard
func (c *Controller) MiniServeHTTP(w http.ResponseWriter, r *http.Request) {
	data := c.buildControlData()

	if err := templates.I2PTunnelMiniControlTemplate.Execute(w, data); err != nil {
		http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
	}
}

// handleGetControl displays the full control page with tunnel status
func (c *Controller) handleGetControl(w http.ResponseWriter, r *http.Request) {
	data := c.buildControlData()

	if err := templates.I2PTunnelControlTemplate.Execute(w, data); err != nil {
		http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
	}
}

// handlePostControl processes control actions (Start, Stop, Restart)
func (c *Controller) handlePostControl(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		c.renderControlWithError(w, fmt.Sprintf("Failed to parse form: %v", err))
		return
	}

	action := r.FormValue("action")
	var err error

	switch action {
	case "Start":
		err = c.handleStart()
	case "Stop":
		err = c.handleStop()
	case "Restart":
		err = c.handleRestart()
	default:
		c.renderControlWithError(w, fmt.Sprintf("Unknown action: %s", action))
		return
	}

	if err != nil {
		c.renderControlWithError(w, fmt.Sprintf("Action '%s' failed: %v", action, err))
		return
	}

	// Redirect back to control page to show updated status
	http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
}

// handleStart starts the tunnel if it's not already running.
// Start() is launched in a background goroutine because most tunnel types
// block indefinitely in an accept loop. The HTTP handler returns immediately
// so the browser receives a redirect response.
func (c *Controller) handleStart() error {
	status := c.Status()
	if status == i2ptunnel.I2PTunnelStatusRunning || status == i2ptunnel.I2PTunnelStatusStarting {
		return fmt.Errorf("tunnel is already %s", status)
	}

	go c.Start()

	return nil
}

// handleStop stops the tunnel if it's running
func (c *Controller) handleStop() error {
	status := c.Status()
	if status == i2ptunnel.I2PTunnelStatusStopped || status == i2ptunnel.I2PTunnelStatusStopping {
		return fmt.Errorf("tunnel is already %s", status)
	}

	if err := c.Stop(); err != nil {
		return fmt.Errorf("failed to stop tunnel: %w", err)
	}

	return nil
}

// handleRestart stops and then starts the tunnel.
// Start() is launched in a background goroutine because most tunnel types
// block indefinitely in an accept loop.
func (c *Controller) handleRestart() error {
	// Stop tunnel if running
	if c.Status() == i2ptunnel.I2PTunnelStatusRunning {
		if err := c.Stop(); err != nil {
			return fmt.Errorf("failed to stop tunnel during restart: %w", err)
		}
	}

	// Start tunnel in background goroutine
	go c.Start()

	return nil
}

// buildControlData creates the data structure for control templates
func (c *Controller) buildControlData() ControlData {
	localAddr, _ := c.LocalAddress()

	data := ControlData{
		Name:         c.Name(),
		ID:           c.ID(),
		Type:         c.Type(),
		Status:       string(c.Status()),
		LocalAddress: localAddr,
		Address:      c.Address(),
		Target:       c.Target(),
		Options:      c.Options(),
	}

	// Include error message if tunnel has an error
	if err := c.Error(); err != nil {
		data.Error = err.Error()
	}

	return data
}

// renderControlWithError displays the control page with an error message
func (c *Controller) renderControlWithError(w http.ResponseWriter, errMsg string) {
	data := c.buildControlData()
	data.Error = errMsg

	w.WriteHeader(http.StatusBadRequest)
	templates.I2PTunnelControlTemplate.Execute(w, data)
}

// NewController creates a Controller from a YAML configuration file.
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
