package controller

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
	templates "github.com/go-i2p/go-i2ptunnel/webui/templates"
	"gopkg.in/yaml.v2"
)

type ControllerGroup struct {
	I2PTunnels     []Controller
	configDir      string
	metricsHandler *metrics.Handler
}

func (cg *ControllerGroup) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// API endpoints return JSON/text — no HTML wrapper.
	if cg.metricsHandler != nil {
		switch r.URL.Path {
		case "/metrics":
			cg.metricsHandler.HandleMetrics(w, r)
			return
		case "/healthz":
			cg.metricsHandler.HandleHealth(w, r)
			return
		case "/api/status":
			cg.metricsHandler.HandleStatus(w, r)
			return
		}
	}

	cg.HandleHTMLHeader(r, w)
	defer cg.HandleHTMLFooter(r, w)
	switch handler(r) {
	case "group":
		cg.HandleGroup(r, w)
	case "new":
		cg.HandleNew(r, w)
	case "control":
		for _, controller := range cg.I2PTunnels {
			if i2ptunnel.Clean(controller.Name()) == tunnel(r) {
				controller.ServeHTTP(w, r)
				return
			}
		}
		cg.HandleError(r, w)
	case "config":
		for _, controller := range cg.I2PTunnels {
			if i2ptunnel.Clean(controller.Name()) == tunnel(r) {
				controller.Config.ServeHTTP(w, r)
				return
			}
		}
		cg.HandleError(r, w)
	default:
		cg.HandleGroup(r, w)
	}
}

func (cg *ControllerGroup) HandleHTMLHeader(r *http.Request, w http.ResponseWriter) {
	templates.HeaderTemplate.Execute(w, nil)
}

func (cg *ControllerGroup) HandleHTMLFooter(r *http.Request, w http.ResponseWriter) {
	templates.FooterTemplate.Execute(w, nil)
}

func (cg *ControllerGroup) HandleGroup(r *http.Request, w http.ResponseWriter) {
	for _, controller := range cg.I2PTunnels {
		controller.MiniServeHTTP(w, r)
	}
	templates.I2PTunnelGroupTemplate.Execute(w, nil)
}

func (cg *ControllerGroup) HandleError(r *http.Request, w http.ResponseWriter) {
	r.Form = nil
	// just redirect back to /home
	http.Redirect(w, r, "/home", 302)
}

// HandleNew handles the /new route for creating new tunnels.
// GET: displays the config template with default values.
// POST: creates a new tunnel config file and adds it to the group.
func (cg *ControllerGroup) HandleNew(r *http.Request, w http.ResponseWriter) {
	switch r.Method {
	case http.MethodGet:
		cg.handleGetNew(w, r)
	case http.MethodPost:
		cg.handlePostNew(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (cg *ControllerGroup) handleGetNew(w http.ResponseWriter, r *http.Request) {
	data := ConfigData{
		Options: map[string]string{
			"interface": "127.0.0.1",
		},
	}
	if err := templates.I2PTunnelConfigTemplate.Execute(w, data); err != nil {
		http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
	}
}

func (cg *ControllerGroup) handlePostNew(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	tunnelType := r.FormValue("type")

	if name == "" {
		cg.renderNewWithError(w, "Tunnel name is required")
		return
	}
	if tunnelType == "" {
		cg.renderNewWithError(w, "Tunnel type is required")
		return
	}

	// Check for duplicate name
	cleanName := i2ptunnel.Clean(name)
	for _, c := range cg.I2PTunnels {
		if i2ptunnel.Clean(c.Name()) == cleanName {
			cg.renderNewWithError(w, fmt.Sprintf("A tunnel named %q already exists", name))
			return
		}
	}

	// Build tunnel config
	tunnelConfig := map[string]interface{}{
		"name": name,
		"type": tunnelType,
	}

	if target := r.FormValue("destination"); target != "" {
		tunnelConfig["target"] = target
	}

	if portStr := r.FormValue("port"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 && p <= 65535 {
			tunnelConfig["port"] = p
		}
	}

	if iface := r.FormValue("interface"); iface != "" {
		tunnelConfig["interface"] = iface
	}

	// Wrap in "tunnels:" top-level key expected by loader
	config := map[string]interface{}{
		"tunnels": map[string]interface{}{
			name: tunnelConfig,
		},
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		cg.renderNewWithError(w, fmt.Sprintf("Failed to generate config: %v", err))
		return
	}

	// Write config file
	configPath := filepath.Join(cg.configDir, cleanName+".yaml")
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		cg.renderNewWithError(w, fmt.Sprintf("Failed to save config: %v", err))
		return
	}

	// Load the new tunnel
	controller, err := NewController(configPath)
	if err != nil {
		os.Remove(configPath) // Clean up on failure
		cg.renderNewWithError(w, fmt.Sprintf("Failed to create tunnel: %v", err))
		return
	}

	cg.I2PTunnels = append(cg.I2PTunnels, *controller)

	// Register new tunnel in metrics registry.
	if cg.metricsHandler != nil {
		m := cg.metricsHandler.Registry.Register(controller.Name(), controller.ID(), controller.Type())
		if bearer, ok := controller.I2PTunnel.(metrics.MetricsBearer); ok {
			bearer.SetTunnelMetrics(m)
		}
	}

	// Redirect to the new tunnel's control page
	http.Redirect(w, r, fmt.Sprintf("/%s/control", controller.ID()), http.StatusSeeOther)
}

func (cg *ControllerGroup) renderNewWithError(w http.ResponseWriter, errMsg string) {
	data := ConfigData{
		Options: map[string]string{
			"interface": "127.0.0.1",
		},
		Error: errMsg,
	}
	w.WriteHeader(http.StatusBadRequest)
	templates.I2PTunnelConfigTemplate.Execute(w, data)
}

func NewControllerGroup(directory string) (*ControllerGroup, error) {
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	registry := metrics.NewRegistry()

	group := &ControllerGroup{
		I2PTunnels: make([]Controller, 0),
		configDir:  directory,
	}

	for _, file := range files {
		if !file.IsDir() {
			controller, err := NewController(filepath.Join(directory, file.Name()))
			if err != nil {
				return nil, err
			}
			group.I2PTunnels = append(group.I2PTunnels, *controller)
			m := registry.Register(controller.Name(), controller.ID(), controller.Type())
			if bearer, ok := controller.I2PTunnel.(metrics.MetricsBearer); ok {
				bearer.SetTunnelMetrics(m)
			}
		}
	}

	group.metricsHandler = metrics.NewHandler(registry, group.tunnelStatus)

	return group, nil
}

// tunnelStatus returns the live state of all tunnels for the metrics handler.
func (cg *ControllerGroup) tunnelStatus() []metrics.TunnelStatus {
	statuses := make([]metrics.TunnelStatus, 0, len(cg.I2PTunnels))
	for _, c := range cg.I2PTunnels {
		localAddr, _ := c.LocalAddress()
		ts := metrics.TunnelStatus{
			Name:         c.Name(),
			ID:           c.ID(),
			Type:         c.Type(),
			Status:       string(c.Status()),
			Address:      c.Address(),
			Target:       c.Target(),
			LocalAddress: localAddr,
		}
		if err := c.Error(); err != nil {
			ts.Error = err.Error()
		}
		statuses = append(statuses, ts)
	}
	return statuses
}
