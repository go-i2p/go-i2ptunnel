package controller

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	"github.com/go-i2p/go-i2ptunnel/lib/loader"
	"github.com/go-i2p/go-i2ptunnel/webui/templates"
	"gopkg.in/yaml.v2"
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
	configPath string // Store config file path for persistence
}

// ConfigData holds data for rendering the configuration template
type ConfigData struct {
	Name    string
	ID      string
	Type    string
	Target  string
	Options map[string]string
	Error   string
}

// ServeHTTP handles both GET (display config form) and POST (save config)
func (c *Config) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c.handleGetConfig(w, r)
	case http.MethodPost:
		c.handlePostConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetConfig displays the configuration form with current settings
func (c *Config) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	data := ConfigData{
		Name:    c.Name(),
		ID:      c.ID(),
		Type:    c.Type(),
		Target:  c.Target(),
		Options: c.Options(),
	}

	if err := templates.I2PTunnelConfigTemplate.Execute(w, data); err != nil {
		http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
	}
}

// handlePostConfig processes configuration changes and persists them to disk
func (c *Config) handlePostConfig(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		c.renderConfigWithError(w, fmt.Sprintf("Failed to parse form: %v", err))
		return
	}

	// Validate that tunnel is not running before applying config changes
	if c.Status() == i2ptunnel.I2PTunnelStatusRunning {
		c.renderConfigWithError(w, "Cannot modify configuration while tunnel is running. Stop the tunnel first.")
		return
	}

	// Build new options map from form data
	newOptions := make(map[string]string)
	for key := range r.Form {
		// Skip non-option fields
		if key == "name" || key == "id" || key == "type" || key == "destination" {
			continue
		}
		newOptions[key] = r.FormValue(key)
	}

	// Validate port if provided
	if portStr := r.FormValue("port"); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			c.renderConfigWithError(w, "Invalid port number")
			return
		}
		if port < 1 || port > 65535 {
			c.renderConfigWithError(w, "Port must be between 1 and 65535")
			return
		}
		newOptions["port"] = portStr
	}

	// Validate host if provided
	if host := r.FormValue("host"); host != "" {
		newOptions["host"] = host
	}

	// Apply new options to tunnel
	if err := c.SetOptions(newOptions); err != nil {
		c.renderConfigWithError(w, fmt.Sprintf("Failed to apply options: %v", err))
		return
	}

	// Persist configuration to disk if config path is available
	if c.configPath != "" {
		if err := c.saveConfig(); err != nil {
			c.renderConfigWithError(w, fmt.Sprintf("Configuration applied but failed to save to disk: %v", err))
			return
		}
	}

	// Redirect to control page after successful save
	http.Redirect(w, r, fmt.Sprintf("/%s/control", c.ID()), http.StatusSeeOther)
}

// renderConfigWithError displays the config form with an error message
func (c *Config) renderConfigWithError(w http.ResponseWriter, errMsg string) {
	data := ConfigData{
		Name:    c.Name(),
		ID:      c.ID(),
		Type:    c.Type(),
		Target:  c.Target(),
		Options: c.Options(),
		Error:   errMsg,
	}
	w.WriteHeader(http.StatusBadRequest)
	templates.I2PTunnelConfigTemplate.Execute(w, data)
}

// saveConfig persists the current tunnel configuration to disk in YAML format.
// Uses the tunnels: wrapper format expected by loader.Load() / Converter.ParseInput().
func (c *Config) saveConfig() error {
	// Build inner tunnel config matching loader expectations
	tunnelConfig := map[string]interface{}{
		"name": c.Name(),
		"type": c.Type(),
	}

	if target := c.Target(); target != "" {
		tunnelConfig["target"] = target
	}

	opts := c.Options()
	if port, ok := opts["port"]; ok {
		if p, err := strconv.Atoi(port); err == nil {
			tunnelConfig["port"] = p
		}
	}
	if iface, ok := opts["interface"]; ok {
		tunnelConfig["interface"] = iface
	}

	// Persist I2CP options (encrypted LeaseSet, authentication, etc.)
	i2cpOpts := i2ptunnel.ExtractI2CPOptions(opts)
	if i2cpOpts != nil {
		tunnelConfig["i2cp"] = i2cpOpts
	}

	// Wrap in the "tunnels:" top-level key expected by the parser
	config := map[string]interface{}{
		"tunnels": map[string]interface{}{
			c.Name(): tunnelConfig,
		},
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(c.configPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write to file atomically using temp file + rename
	tempPath := c.configPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	if err := os.Rename(tempPath, c.configPath); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// NewConfig loads a tunnel configuration from a YAML file and returns a Config.
func NewConfig(yamlFile string) (*Config, error) {
	tunnel, err := loader.Load(yamlFile, "localhost:7656")
	if err != nil {
		return nil, err
	}
	return &Config{
		I2PTunnel:  tunnel,
		configPath: yamlFile,
	}, nil
}
