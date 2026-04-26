package controller

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

// Config is an HTTP handler that displays and accepts edits for a single tunnel's
// configuration. It wraps an I2PTunnel and persists changes via SetOptions and
// the tunnel's LoadConfig mechanism.
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
	if c.Status() == i2ptunnel.I2PTunnelStatusRunning {
		c.renderConfigWithError(w, "Cannot modify configuration while tunnel is running. Stop the tunnel first.")
		return
	}

	newOptions, errMsg := buildConfigOptions(r)
	if errMsg != "" {
		c.renderConfigWithError(w, errMsg)
		return
	}

	if err := c.SetOptions(newOptions); err != nil {
		c.renderConfigWithError(w, fmt.Sprintf("Failed to apply options: %v", err))
		return
	}
	if c.configPath != "" {
		if err := c.saveConfig(); err != nil {
			c.renderConfigWithError(w, fmt.Sprintf("Configuration applied but failed to save to disk: %v", err))
			return
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/%s/control", c.ID()), http.StatusSeeOther)
}

// buildConfigOptions extracts and validates config options from a POST form.
// Returns (options, errorMessage). errorMessage is empty on success.
func buildConfigOptions(r *http.Request) (map[string]string, string) {
	newOptions := make(map[string]string)
	for key := range r.Form {
		if key == "name" || key == "id" || key == "type" || key == "destination" {
			continue
		}
		newOptions[key] = r.FormValue(key)
	}
	if portStr := r.FormValue("port"); portStr != "" {
		if errMsg := validateAndSetPort(portStr, newOptions); errMsg != "" {
			return nil, errMsg
		}
	}
	authType := newOptions["i2cp.leaseSetAuthType"]
	if authType == "1" || authType == "2" {
		if strings.TrimSpace(newOptions["i2cp.leaseSetPrivKey"]) == "" {
			return nil, "i2cp.leaseSetPrivKey is required when LeaseSet authentication type is DH (1) or PSK (2)"
		}
	}
	if host := r.FormValue("host"); host != "" {
		newOptions["host"] = host
	}
	return newOptions, ""
}

// validateAndSetPort validates the port string and sets it in newOptions.
// Returns an error message if invalid, or empty string on success.
func validateAndSetPort(portStr string, newOptions map[string]string) string {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "Invalid port number"
	}
	if port < 1 || port > 65535 {
		return "Port must be between 1 and 65535"
	}
	newOptions["port"] = portStr
	return ""
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
func (c *Config) saveConfig() error {
	data, err := marshalTunnelConfig(c.Name(), c.Type(), c.Target(), c.Options())
	if err != nil {
		return err
	}
	dir := filepath.Dir(c.configPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	tempPath := c.configPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}
	if err := os.Rename(tempPath, c.configPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

// marshalTunnelConfig builds and marshals the YAML config structure.
func marshalTunnelConfig(name, tunnelType, target string, opts map[string]string) ([]byte, error) {
	tunnelConfig := map[string]interface{}{
		"name": name,
		"type": tunnelType,
	}
	if target != "" {
		tunnelConfig["target"] = target
	}
	if port, ok := opts["port"]; ok {
		if p, err := strconv.Atoi(port); err == nil {
			tunnelConfig["port"] = p
		}
	}
	if iface, ok := opts["interface"]; ok {
		tunnelConfig["interface"] = iface
	}
	if i2cpOpts := i2ptunnel.ExtractI2CPOptions(opts); i2cpOpts != nil {
		tunnelConfig["i2cp"] = i2cpOpts
	}
	config := map[string]interface{}{
		"tunnels": map[string]interface{}{
			name: tunnelConfig,
		},
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	return data, nil
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
