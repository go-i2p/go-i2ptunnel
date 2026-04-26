package controller

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/metrics"
	templates "github.com/go-i2p/go-i2ptunnel/webui/templates"
	"gopkg.in/yaml.v2"
)

// ControllerGroup is the top-level HTTP handler for the web management UI.
// It holds all active tunnel Controllers and routes incoming requests to the
// appropriate tunnel handler, metrics endpoint, or home page.
type ControllerGroup struct {
	I2PTunnels     []Controller
	configDir      string
	metricsHandler *metrics.Handler
	csrfProtection *http.CrossOriginProtection
}

// ServeHTTP dispatches requests to the appropriate handler based on URL path.
func (cg *ControllerGroup) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if cg.csrfProtection != nil {
		if err := cg.csrfProtection.Check(r); err != nil {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}
	if cg.handleMetricsRoute(w, r) {
		return
	}
	cg.HandleHTMLHeader(r, w)
	defer cg.HandleHTMLFooter(r, w)
	cg.routeRequest(w, r)
}

// routeRequest dispatches the request to the correct handler after headers are written.
func (cg *ControllerGroup) routeRequest(w http.ResponseWriter, r *http.Request) {
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

// handleMetricsRoute serves /metrics, /healthz, and /api/status without auth.
// Returns true if the request was handled.
func (cg *ControllerGroup) handleMetricsRoute(w http.ResponseWriter, r *http.Request) bool {
	if cg.metricsHandler == nil {
		return false
	}
	switch r.URL.Path {
	case "/metrics":
		cg.metricsHandler.HandleMetrics(w, r)
		return true
	case "/healthz":
		cg.metricsHandler.HandleHealth(w, r)
		return true
	case "/api/status":
		cg.metricsHandler.HandleStatus(w, r)
		return true
	}
	return false
}

// HandleHTMLHeader writes the HTML header template to the response.
func (cg *ControllerGroup) HandleHTMLHeader(r *http.Request, w http.ResponseWriter) {
	templates.HeaderTemplate.Execute(w, nil)
}

// HandleHTMLFooter writes the HTML footer template to the response.
func (cg *ControllerGroup) HandleHTMLFooter(r *http.Request, w http.ResponseWriter) {
	templates.FooterTemplate.Execute(w, nil)
}

// HandleGroup renders the tunnel group overview page.
func (cg *ControllerGroup) HandleGroup(r *http.Request, w http.ResponseWriter) {
	for _, controller := range cg.I2PTunnels {
		controller.MiniServeHTTP(w, r)
	}
	templates.I2PTunnelGroupTemplate.Execute(w, nil)
}

// HandleError redirects unmatched tunnel requests back to the home page.
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

	name, tunnelType, validErr := validateNewTunnelForm(r, cg.I2PTunnels)
	if validErr != "" {
		cg.renderNewWithError(w, validErr)
		return
	}

	cleanName := i2ptunnel.Clean(name)
	configPath := filepath.Join(cg.configDir, cleanName+".yaml")
	if err := writeTunnelConfigYAML(r, name, tunnelType, configPath); err != nil {
		cg.renderNewWithError(w, err.Error())
		return
	}

	controller, err := cg.createAndRegisterController(configPath)
	if err != nil {
		os.Remove(configPath)
		cg.renderNewWithError(w, err.Error())
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/%s/control", controller.ID()), http.StatusSeeOther)
}

// createAndRegisterController loads a tunnel config, appends it to I2PTunnels, and registers metrics.
func (cg *ControllerGroup) createAndRegisterController(configPath string) (*Controller, error) {
	controller, err := NewController(configPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to create tunnel: %v", err)
	}
	cg.I2PTunnels = append(cg.I2PTunnels, *controller)
	if cg.metricsHandler != nil {
		m := cg.metricsHandler.Registry.Register(controller.Name(), controller.ID(), controller.Type())
		if bearer, ok := controller.I2PTunnel.(metrics.MetricsBearer); ok {
			bearer.SetTunnelMetrics(m)
		}
	}
	return controller, nil
}

// validateNewTunnelForm checks required fields and duplicate names.
// Returns (name, tunnelType, errorMessage). errorMessage is empty on success.
func validateNewTunnelForm(r *http.Request, tunnels []Controller) (name, tunnelType, errMsg string) {
	name = r.FormValue("name")
	tunnelType = r.FormValue("type")
	if name == "" {
		return "", "", "Tunnel name is required"
	}
	if tunnelType == "" {
		return "", "", "Tunnel type is required"
	}
	cleanName := i2ptunnel.Clean(name)
	for _, c := range tunnels {
		if i2ptunnel.Clean(c.Name()) == cleanName {
			return "", "", fmt.Sprintf("A tunnel named %q already exists", name)
		}
	}
	return name, tunnelType, ""
}

// writeTunnelConfigYAML builds the tunnel config map from form values and writes it to path.
func writeTunnelConfigYAML(r *http.Request, name, tunnelType, configPath string) error {
	tunnelConfig, err := buildTunnelConfigMap(r, name, tunnelType)
	if err != nil {
		return err
	}
	config := map[string]interface{}{
		"tunnels": map[string]interface{}{name: tunnelConfig},
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("Failed to generate config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("Failed to save config: %v", err)
	}
	return nil
}

// buildTunnelConfigMap assembles the tunnel config map from a POST form.
func buildTunnelConfigMap(r *http.Request, name, tunnelType string) (map[string]interface{}, error) {
	cfg := map[string]interface{}{
		"name": name,
		"type": tunnelType,
	}
	if target := r.FormValue("destination"); target != "" {
		cfg["target"] = target
	}
	if portStr := r.FormValue("port"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 && p <= 65535 {
			cfg["port"] = p
		}
	}
	if iface := r.FormValue("interface"); iface != "" {
		cfg["interface"] = iface
	}
	if err := applyI2CPConfig(r, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// applyI2CPConfig extracts i2cp.* form fields, validates auth, and adds the result to cfg.
func applyI2CPConfig(r *http.Request, cfg map[string]interface{}) error {
	i2cpForm := make(map[string]string)
	for key := range r.Form {
		if strings.HasPrefix(key, "i2cp.") && r.FormValue(key) != "" {
			i2cpForm[key] = r.FormValue(key)
		}
	}
	authType := i2cpForm["i2cp.leaseSetAuthType"]
	if authType == "1" || authType == "2" {
		if strings.TrimSpace(i2cpForm["i2cp.leaseSetPrivKey"]) == "" {
			return fmt.Errorf("i2cp.leaseSetPrivKey is required when LeaseSet authentication type is DH (1) or PSK (2)")
		}
	}
	if extracted := i2ptunnel.ExtractI2CPOptions(i2cpForm); extracted != nil {
		cfg["i2cp"] = extracted
	}
	return nil
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

// NewControllerGroup scans directory for YAML config files and creates a
// ControllerGroup with one Controller per file.
func NewControllerGroup(directory string) (*ControllerGroup, error) {
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	registry := metrics.NewRegistry()
	group := &ControllerGroup{
		I2PTunnels:     make([]Controller, 0),
		configDir:      directory,
		csrfProtection: http.NewCrossOriginProtection(),
	}
	if err := group.loadControllersFromFiles(files, directory, registry); err != nil {
		return nil, err
	}
	group.metricsHandler = metrics.NewHandler(registry, group.tunnelStatus)
	return group, nil
}

// loadControllersFromFiles loads each non-directory config file and registers metrics.
func (cg *ControllerGroup) loadControllersFromFiles(files []os.DirEntry, directory string, registry *metrics.Registry) error {
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		controller, err := NewController(filepath.Join(directory, file.Name()))
		if err != nil {
			return err
		}
		cg.I2PTunnels = append(cg.I2PTunnels, *controller)
		m := registry.Register(controller.Name(), controller.ID(), controller.Type())
		if bearer, ok := controller.I2PTunnel.(metrics.MetricsBearer); ok {
			bearer.SetTunnelMetrics(m)
		}
	}
	return nil
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
