package validate

import (
	"strings"
	"testing"
)

// TestPort verifies port validation with various valid and invalid values
func TestPort(t *testing.T) {
	tests := []struct {
		name      string
		port      int
		wantError bool
		wantHint  string
	}{
		{
			name:      "valid unprivileged port",
			port:      8080,
			wantError: false,
		},
		{
			name:      "valid high port",
			port:      65535,
			wantError: false,
		},
		{
			name:      "valid low unprivileged port",
			port:      1024,
			wantError: false,
		},
		{
			name:      "privileged port (valid, no error)",
			port:      80,
			wantError: false,
		},
		{
			name:      "port zero",
			port:      0,
			wantError: true,
		},
		{
			name:      "negative port",
			port:      -1,
			wantError: true,
		},
		{
			name:      "port too high",
			port:      65536,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Port(tt.port)
			if tt.wantError && err == nil {
				t.Errorf("Port(%d) expected error but got none", tt.port)
			}
			if !tt.wantError && err != nil {
				t.Errorf("Port(%d) unexpected error: %v", tt.port, err)
			}
			if tt.wantHint != "" && err != nil {
				if !strings.Contains(err.Error(), tt.wantHint) {
					t.Errorf("Port(%d) error missing hint '%s': %v", tt.port, tt.wantHint, err)
				}
			}
		})
	}
}

// TestPortString verifies port string validation and parsing
func TestPortString(t *testing.T) {
	tests := []struct {
		name      string
		portStr   string
		wantPort  int
		wantError bool
	}{
		{
			name:      "valid port string",
			portStr:   "8080",
			wantPort:  8080,
			wantError: false,
		},
		{
			name:      "invalid non-numeric",
			portStr:   "abc",
			wantError: true,
		},
		{
			name:      "invalid float",
			portStr:   "80.5",
			wantError: true,
		},
		{
			name:      "invalid empty",
			portStr:   "",
			wantError: true,
		},
		{
			name:      "valid privileged port",
			portStr:   "443",
			wantPort:  443,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, err := PortString(tt.portStr)
			if tt.wantError && err == nil {
				t.Errorf("PortString(%q) expected error but got none", tt.portStr)
			}
			if !tt.wantError && err != nil {
				t.Errorf("PortString(%q) unexpected error: %v", tt.portStr, err)
			}
			if !tt.wantError && port != tt.wantPort {
				t.Errorf("PortString(%q) = %d, want %d", tt.portStr, port, tt.wantPort)
			}
		})
	}
}

// TestI2PAddress verifies I2P address validation
func TestI2PAddress(t *testing.T) {
	tests := []struct {
		name      string
		addr      string
		wantError bool
	}{
		{
			name:      "empty address",
			addr:      "",
			wantError: true,
		},
		{
			name:      "invalid format",
			addr:      "not-an-address",
			wantError: true,
		},
		{
			name:      "invalid base32",
			addr:      "invalid!!!.b32.i2p",
			wantError: true,
		},
		// Note: Valid addresses require actual I2P keys or lookups
		// The i2pkeys.Lookup function validates format and may do DNS lookups
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := I2PAddress(tt.addr)
			if tt.wantError && err == nil {
				t.Errorf("I2PAddress(%q) expected error but got none", tt.addr)
			}
			if !tt.wantError && err != nil {
				t.Errorf("I2PAddress(%q) unexpected error: %v", tt.addr, err)
			}
		})
	}
}

// TestNetworkAddress verifies network address validation
func TestNetworkAddress(t *testing.T) {
	tests := []struct {
		name      string
		addr      string
		wantError bool
	}{
		{
			name:      "valid localhost",
			addr:      "localhost:8080",
			wantError: false,
		},
		{
			name:      "valid IPv4",
			addr:      "127.0.0.1:3000",
			wantError: false,
		},
		{
			name:      "valid IPv6",
			addr:      "[::1]:8080",
			wantError: false,
		},
		{
			name:      "empty address",
			addr:      "",
			wantError: true,
		},
		{
			name:      "missing port",
			addr:      "localhost",
			wantError: true,
		},
		{
			name:      "invalid port",
			addr:      "localhost:99999",
			wantError: true,
		},
		{
			name:      "invalid format",
			addr:      "not-valid",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NetworkAddress(tt.addr)
			if tt.wantError && err == nil {
				t.Errorf("NetworkAddress(%q) expected error but got none", tt.addr)
			}
			if !tt.wantError && err != nil {
				t.Errorf("NetworkAddress(%q) unexpected error: %v", tt.addr, err)
			}
		})
	}
}

// TestInterface verifies network interface validation
func TestInterface(t *testing.T) {
	tests := []struct {
		name      string
		iface     string
		wantError bool
	}{
		{
			name:      "empty (all interfaces)",
			iface:     "",
			wantError: false,
		},
		{
			name:      "localhost IPv4",
			iface:     "127.0.0.1",
			wantError: false,
		},
		{
			name:      "localhost IPv6",
			iface:     "::1",
			wantError: false,
		},
		{
			name:      "any IPv4",
			iface:     "0.0.0.0",
			wantError: false,
		},
		{
			name:      "any IPv6",
			iface:     "::",
			wantError: false,
		},
		{
			name:      "invalid format",
			iface:     "not-an-ip",
			wantError: true,
		},
		{
			name:      "invalid IPv4",
			iface:     "999.999.999.999",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Interface(tt.iface)
			if tt.wantError && err == nil {
				t.Errorf("Interface(%q) expected error but got none", tt.iface)
			}
			if !tt.wantError && err != nil {
				t.Errorf("Interface(%q) unexpected error: %v", tt.iface, err)
			}
		})
	}
}

// TestRequiredString verifies required field validation
func TestRequiredString(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		value     string
		wantError bool
	}{
		{
			name:      "valid value",
			field:     "name",
			value:     "my-tunnel",
			wantError: false,
		},
		{
			name:      "empty string",
			field:     "name",
			value:     "",
			wantError: true,
		},
		{
			name:      "whitespace only",
			field:     "name",
			value:     "   ",
			wantError: true,
		},
		{
			name:      "tabs only",
			field:     "type",
			value:     "\t\t",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RequiredString(tt.field, tt.value)
			if tt.wantError && err == nil {
				t.Errorf("RequiredString(%q, %q) expected error but got none", tt.field, tt.value)
			}
			if !tt.wantError && err != nil {
				t.Errorf("RequiredString(%q, %q) unexpected error: %v", tt.field, tt.value, err)
			}
			if err != nil {
				// Verify error contains field name
				if !strings.Contains(err.Error(), tt.field) {
					t.Errorf("RequiredString error missing field name '%s': %v", tt.field, err)
				}
			}
		})
	}
}

// TestTunnelType verifies tunnel type validation
func TestTunnelType(t *testing.T) {
	tests := []struct {
		name       string
		tunnelType string
		wantError  bool
	}{
		{
			name:       "tcpclient",
			tunnelType: "tcpclient",
			wantError:  false,
		},
		{
			name:       "tcpserver",
			tunnelType: "tcpserver",
			wantError:  false,
		},
		{
			name:       "httpclient",
			tunnelType: "httpclient",
			wantError:  false,
		},
		{
			name:       "httpserver",
			tunnelType: "httpserver",
			wantError:  false,
		},
		{
			name:       "ircclient",
			tunnelType: "ircclient",
			wantError:  false,
		},
		{
			name:       "ircserver",
			tunnelType: "ircserver",
			wantError:  false,
		},
		{
			name:       "udpclient",
			tunnelType: "udpclient",
			wantError:  false,
		},
		{
			name:       "udpserver",
			tunnelType: "udpserver",
			wantError:  false,
		},
		{
			name:       "socksclient",
			tunnelType: "socksclient",
			wantError:  false,
		},
		{
			name:       "socks",
			tunnelType: "socks",
			wantError:  false,
		},
		{
			name:       "tcpbidirectional",
			tunnelType: "tcpbidirectional",
			wantError:  false,
		},
		{
			name:       "udpbidirectional",
			tunnelType: "udpbidirectional",
			wantError:  false,
		},
		{
			name:       "httpbidirectional",
			tunnelType: "httpbidirectional",
			wantError:  false,
		},
		{
			name:       "invalid type",
			tunnelType: "invalidtype",
			wantError:  true,
		},
		{
			name:       "empty type",
			tunnelType: "",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TunnelType(tt.tunnelType)
			if tt.wantError && err == nil {
				t.Errorf("TunnelType(%q) expected error but got none", tt.tunnelType)
			}
			if !tt.wantError && err != nil {
				t.Errorf("TunnelType(%q) unexpected error: %v", tt.tunnelType, err)
			}
		})
	}
}

// TestMaxConnections verifies max connections validation
func TestMaxConnections(t *testing.T) {
	tests := []struct {
		name      string
		maxConns  int
		wantError bool
		wantHint  string
	}{
		{
			name:      "unlimited (zero)",
			maxConns:  0,
			wantError: false,
		},
		{
			name:      "reasonable limit",
			maxConns:  100,
			wantError: false,
		},
		{
			name:      "high limit",
			maxConns:  1000,
			wantError: false,
		},
		{
			name:      "very high limit with warning (logged, not error)",
			maxConns:  15000,
			wantError: false,
		},
		{
			name:      "negative value",
			maxConns:  -1,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MaxConnections(tt.maxConns)
			if tt.wantError && err == nil {
				t.Errorf("MaxConnections(%d) expected error but got none", tt.maxConns)
			}
			if !tt.wantError && err != nil {
				t.Errorf("MaxConnections(%d) unexpected error: %v", tt.maxConns, err)
			}
			if tt.wantHint != "" && err != nil {
				if !strings.Contains(err.Error(), tt.wantHint) {
					t.Errorf("MaxConnections(%d) error missing hint '%s': %v", tt.maxConns, tt.wantHint, err)
				}
			}
		})
	}
}

// TestRateLimit verifies rate limit validation
func TestRateLimit(t *testing.T) {
	tests := []struct {
		name      string
		rateLimit float64
		wantError bool
	}{
		{
			name:      "no limit (zero)",
			rateLimit: 0,
			wantError: false,
		},
		{
			name:      "reasonable limit",
			rateLimit: 10.0,
			wantError: false,
		},
		{
			name:      "high limit",
			rateLimit: 1000.5,
			wantError: false,
		},
		{
			name:      "fractional limit",
			rateLimit: 0.5,
			wantError: false,
		},
		{
			name:      "negative value",
			rateLimit: -1.0,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RateLimit(tt.rateLimit)
			if tt.wantError && err == nil {
				t.Errorf("RateLimit(%f) expected error but got none", tt.rateLimit)
			}
			if !tt.wantError && err != nil {
				t.Errorf("RateLimit(%f) unexpected error: %v", tt.rateLimit, err)
			}
		})
	}
}

// TestRateLimitString verifies rate limit string validation and parsing
func TestRateLimitString(t *testing.T) {
	tests := []struct {
		name          string
		rateLimitStr  string
		wantRateLimit float64
		wantError     bool
	}{
		{
			name:          "valid rate limit",
			rateLimitStr:  "10.5",
			wantRateLimit: 10.5,
			wantError:     false,
		},
		{
			name:          "zero limit",
			rateLimitStr:  "0",
			wantRateLimit: 0,
			wantError:     false,
		},
		{
			name:         "invalid non-numeric",
			rateLimitStr: "abc",
			wantError:    true,
		},
		{
			name:         "invalid empty",
			rateLimitStr: "",
			wantError:    true,
		},
		{
			name:         "negative value",
			rateLimitStr: "-5.0",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rateLimit, err := RateLimitString(tt.rateLimitStr)
			if tt.wantError && err == nil {
				t.Errorf("RateLimitString(%q) expected error but got none", tt.rateLimitStr)
			}
			if !tt.wantError && err != nil {
				t.Errorf("RateLimitString(%q) unexpected error: %v", tt.rateLimitStr, err)
			}
			if !tt.wantError && rateLimit != tt.wantRateLimit {
				t.Errorf("RateLimitString(%q) = %f, want %f", tt.rateLimitStr, rateLimit, tt.wantRateLimit)
			}
		})
	}
}

// TestValidationError verifies ValidationError formatting
func TestValidationError(t *testing.T) {
	tests := []struct {
		name      string
		err       *ValidationError
		wantError string
	}{
		{
			name: "error with hint",
			err: &ValidationError{
				Field:   "port",
				Value:   "abc",
				Message: "invalid format",
				Hint:    "use numeric value",
			},
			wantError: "port: invalid format (hint: use numeric value)",
		},
		{
			name: "error without hint",
			err: &ValidationError{
				Field:   "name",
				Value:   "",
				Message: "cannot be empty",
			},
			wantError: "name: cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.wantError {
				t.Errorf("ValidationError.Error() = %q, want %q", got, tt.wantError)
			}
		})
	}
}
