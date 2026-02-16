package validate

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/go-i2p/i2pkeys"
)

// ValidationError represents a configuration validation error with actionable context.
type ValidationError struct {
	Field   string // Field name that failed validation
	Value   string // The invalid value
	Message string // User-friendly error message
	Hint    string // Suggestion for fixing the error
}

func (e *ValidationError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s: %s (hint: %s)", e.Field, e.Message, e.Hint)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Port validates a port number is in the valid range (1-65535).
// For non-root deployments, ports below 1024 trigger a warning in the hint.
//
// Why: Privileged ports (<1024) require root/capabilities on Unix systems.
// Production deployments should use unprivileged ports for security.
func Port(port int) error {
	if port < 1 || port > 65535 {
		return &ValidationError{
			Field:   "port",
			Value:   strconv.Itoa(port),
			Message: "port must be between 1 and 65535",
			Hint:    "choose a port in the valid range (1024-65535 recommended for non-root)",
		}
	}
	// Privileged ports (<1024) are valid but may require root privileges.
	// As documented: warn but don't fail validation.
	// Callers that need to warn can check port < 1024 separately.
	return nil
}

// PortString validates a port number provided as a string.
// Returns the parsed port value and any validation error.
func PortString(portStr string) (int, error) {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, &ValidationError{
			Field:   "port",
			Value:   portStr,
			Message: "invalid port format",
			Hint:    "port must be a numeric value between 1 and 65535",
		}
	}
	if err := Port(port); err != nil {
		return 0, err
	}
	return port, nil
}

// I2PAddress validates an I2P destination address format.
// Accepts base32.i2p, base64, or full I2P addresses.
//
// Why: Invalid addresses cause runtime connection failures. Fail fast at config time.
// Design: Uses i2pkeys.Lookup which handles multiple formats and performs validation.
func I2PAddress(addr string) error {
	if addr == "" {
		return &ValidationError{
			Field:   "target",
			Value:   addr,
			Message: "I2P address cannot be empty",
			Hint:    "provide a valid base32.i2p or base64 I2P address",
		}
	}

	// Use i2pkeys library to validate address format
	// This handles base32, base64, and full destination formats
	_, err := i2pkeys.Lookup(addr)
	if err != nil {
		return &ValidationError{
			Field:   "target",
			Value:   addr,
			Message: "invalid I2P address format",
			Hint:    "provide a valid base32.i2p address (e.g., example.b32.i2p) or base64 destination",
		}
	}
	return nil
}

// NetworkAddress validates a network address in host:port format.
// Used for local service targets (e.g., localhost:8080, 127.0.0.1:3000).
//
// Why: Server tunnels forward to local services. Invalid addresses cause runtime failures.
// Design: Uses net.ResolveTCPAddr for validation, ensuring both hostname and port are valid.
func NetworkAddress(addr string) error {
	if addr == "" {
		return &ValidationError{
			Field:   "target",
			Value:   addr,
			Message: "network address cannot be empty",
			Hint:    "provide a valid host:port address (e.g., localhost:8080, 127.0.0.1:3000)",
		}
	}

	// Validate format using standard library
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return &ValidationError{
			Field:   "target",
			Value:   addr,
			Message: "invalid network address format",
			Hint:    "provide address as host:port (e.g., localhost:8080, 192.168.1.1:9000)",
		}
	}

	// Validate the port component
	if err := Port(tcpAddr.Port); err != nil {
		// Wrap the port error with network address context
		if portErr, ok := err.(*ValidationError); ok {
			return &ValidationError{
				Field:   "target",
				Value:   addr,
				Message: portErr.Message,
				Hint:    portErr.Hint,
			}
		}
		return err
	}

	return nil
}

// Interface validates a network interface address for binding.
// Accepts IP addresses or empty string for all interfaces.
//
// Why: Invalid interface addresses cause bind failures at startup.
// Design: Uses net.ParseIP for validation. Empty string means bind to all interfaces (0.0.0.0).
func Interface(iface string) error {
	// Empty string means bind to all interfaces (standard behavior)
	if iface == "" {
		return nil
	}

	// Validate IP address format
	ip := net.ParseIP(iface)
	if ip == nil {
		return &ValidationError{
			Field:   "interface",
			Value:   iface,
			Message: "invalid interface address",
			Hint:    "provide a valid IP address (e.g., 127.0.0.1, ::1) or leave empty for all interfaces",
		}
	}

	return nil
}

// RequiredString validates that a required string field is not empty.
// Used for critical configuration fields like tunnel name and type.
func RequiredString(field, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return &ValidationError{
			Field:   field,
			Value:   value,
			Message: fmt.Sprintf("%s is required", field),
			Hint:    fmt.Sprintf("provide a non-empty value for %s", field),
		}
	}
	return nil
}

// TunnelType validates that a tunnel type matches one of the supported types.
// Supported types: tcpclient, tcpserver, httpclient, httpserver, ircclient, ircserver,
// udpclient, udpserver, socksclient.
func TunnelType(tunnelType string) error {
	validTypes := map[string]bool{
		"tcpclient":         true,
		"tcpserver":         true,
		"httpclient":        true,
		"httpserver":        true,
		"ircclient":         true,
		"ircserver":         true,
		"udpclient":         true,
		"udpserver":         true,
		"socks":             true,
		"socksclient":       true,
		"tcpbidirectional":  true,
		"udpbidirectional":  true,
		"httpbidirectional": true,
	}

	if !validTypes[tunnelType] {
		return &ValidationError{
			Field:   "type",
			Value:   tunnelType,
			Message: "unsupported tunnel type",
			Hint:    "supported types: tcpclient, tcpserver, httpclient, httpserver, ircclient, ircserver, udpclient, udpserver, socks, socksclient, tcpbidirectional, udpbidirectional, httpbidirectional",
		}
	}
	return nil
}

// MaxConnections validates the maximum connections setting for server tunnels.
// Zero means unlimited. Negative values are rejected.
//
// Why: Limits resource usage and prevents DoS. Validation prevents configuration mistakes.
func MaxConnections(maxConns int) error {
	if maxConns < 0 {
		return &ValidationError{
			Field:   "maxconns",
			Value:   strconv.Itoa(maxConns),
			Message: "maxconns cannot be negative",
			Hint:    "use 0 for unlimited connections or a positive number to set a limit",
		}
	}
	// Warn about very high connection limits (but allow them)
	if maxConns > 10000 {
		log.WithField("maxconns", maxConns).Warn("maxconns is very high (> 10000); high connection limits may cause resource exhaustion")
	}
	return nil
}

// RateLimit validates the rate limit setting for server tunnels.
// Zero means no rate limiting. Negative values are rejected.
//
// Why: Rate limiting protects against abuse. Validation prevents configuration errors.
func RateLimit(rateLimit float64) error {
	if rateLimit < 0 {
		return &ValidationError{
			Field:   "ratelimit",
			Value:   strconv.FormatFloat(rateLimit, 'f', -1, 64),
			Message: "ratelimit cannot be negative",
			Hint:    "use 0 for no rate limiting or a positive number for requests/second",
		}
	}
	return nil
}

// RateLimitString validates a rate limit provided as a string.
// Returns the parsed rate limit value and any validation error.
func RateLimitString(rateLimitStr string) (float64, error) {
	rateLimit, err := strconv.ParseFloat(rateLimitStr, 64)
	if err != nil {
		return 0, &ValidationError{
			Field:   "ratelimit",
			Value:   rateLimitStr,
			Message: "invalid ratelimit format",
			Hint:    "ratelimit must be a numeric value (e.g., 10.0, 100.5)",
		}
	}
	if err := RateLimit(rateLimit); err != nil {
		return 0, err
	}
	return rateLimit, nil
}
