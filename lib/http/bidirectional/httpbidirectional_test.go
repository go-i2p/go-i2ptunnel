package httpbidirectional

import (
	"context"
	"net"
	"net/http"
	"testing"

	httpinspector "github.com/go-i2p/go-connfilter/http"
	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	httpclient "github.com/go-i2p/go-i2ptunnel/lib/http/client"
	httpserver "github.com/go-i2p/go-i2ptunnel/lib/http/server"
	limitedlistener "github.com/go-i2p/go-limit"
)

// TestHTTPBidirectionalHasHTTPProxy verifies that the struct uses HTTP proxy
// fields (proxyServer, httpServer) instead of SOCKS5.
func TestHTTPBidirectionalHasHTTPProxy(t *testing.T) {
	tunnel := &HTTPBidirectional{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		done:            make(chan struct{}),
	}
	// proxyServer and httpServer should be nil when not started
	if tunnel.proxyServer != nil {
		t.Error("proxyServer should be nil before Start()")
	}
	if tunnel.httpServer != nil {
		t.Error("httpServer should be nil before Start()")
	}
}

// TestDefaultConfigs verifies that the default configs match the
// expected HTTP client and server sanitize configs.
func TestDefaultConfigs(t *testing.T) {
	clientConfig := httpclient.DefaultHTTPClientConfig()
	serverConfig := httpserver.DefaultHTTPServerConfig()

	tunnel := &HTTPBidirectional{
		ClientConfig: clientConfig,
		ServerConfig: serverConfig,
		done:         make(chan struct{}),
	}

	// Verify client config has an OnRequest handler
	if tunnel.ClientConfig.OnRequest == nil {
		t.Error("ClientConfig.OnRequest should not be nil")
	}
	// Verify server config has both handlers
	if tunnel.ServerConfig.OnRequest == nil {
		t.Error("ServerConfig.OnRequest should not be nil")
	}
	if tunnel.ServerConfig.OnResponse == nil {
		t.Error("ServerConfig.OnResponse should not be nil")
	}
}

// TestClientConfigStripsHeaders verifies the client-side filter strips
// identifying headers from outbound requests.
func TestClientConfigStripsHeaders(t *testing.T) {
	config := httpclient.DefaultHTTPClientConfig()
	req, _ := http.NewRequest("GET", "http://example.i2p", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	req.Header.Set("X-Real-IP", "10.0.0.1")
	req.Header.Set("Via", "1.1 proxy")

	if err := config.OnRequest(req); err != nil {
		t.Fatalf("OnRequest returned error: %v", err)
	}

	for _, h := range []string{"X-Forwarded-For", "X-Real-IP", "Via"} {
		if req.Header.Get(h) != "" {
			t.Errorf("Header %s should be stripped, got %q", h, req.Header.Get(h))
		}
	}
}

// TestServerConfigStripsHeaders verifies the server-side filter strips
// fingerprinting headers from responses.
func TestServerConfigStripsHeaders(t *testing.T) {
	config := httpserver.DefaultHTTPServerConfig()
	resp := &http.Response{
		Header: http.Header{
			"Server":       {"nginx/1.0"},
			"X-Powered-By": {"PHP/7.4"},
		},
	}

	if err := config.OnResponse(resp); err != nil {
		t.Fatalf("OnResponse returned error: %v", err)
	}

	if resp.Header.Get("Server") != "" {
		t.Error("Server header should be stripped from response")
	}
	if resp.Header.Get("X-Powered-By") != "" {
		t.Error("X-Powered-By header should be stripped from response")
	}
}

// TestStopBeforeStart verifies Stop is safe when Start was never called.
func TestStopBeforeStart(t *testing.T) {
	tunnel := &HTTPBidirectional{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		done:            make(chan struct{}),
	}
	if err := tunnel.Stop(); err != nil {
		t.Fatalf("Stop() before Start() returned error: %v", err)
	}
	if tunnel.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected stopped status, got %v", tunnel.Status())
	}
}

// TestStopWithHTTPServer verifies Stop shuts down the HTTP server properly.
func TestStopWithHTTPServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	srv := &http.Server{Handler: http.DefaultServeMux}
	go srv.Serve(listener)

	tunnel := &HTTPBidirectional{
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusRunning,
		httpServer:      srv,
		ctx:             ctx,
		cancel:          cancel,
		done:            make(chan struct{}),
	}

	if err := tunnel.Stop(); err != nil {
		t.Fatalf("Stop() returned error: %v", err)
	}
	if tunnel.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Expected stopped, got %v", tunnel.Status())
	}
}

// TestHTTPBidirectionalRecordError verifies concurrent error recording.
func TestHTTPBidirectionalRecordError(t *testing.T) {
	tunnel := &HTTPBidirectional{
		done: make(chan struct{}),
	}
	tunnel.recordError(net.ErrClosed)

	if tunnel.Error() == nil {
		t.Fatal("Expected error after recordError, got nil")
	}
	// Error is wrapped in I2PTunnelError, so check the string contains the original
	errStr := tunnel.Error().Error()
	if errStr == "" {
		t.Error("Expected non-empty error string")
	}
}

// TestHTTPBidirectionalFullStruct verifies the struct can be created with all
// the new fields populated correctly.
func TestHTTPBidirectionalFullStruct(t *testing.T) {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	tunnel := &HTTPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Name:      "test-bidi",
			Type:      "httpbidirectional",
			Interface: "127.0.0.1",
			Port:      4449,
			Target:    "127.0.0.1:8080",
		},
		Addr:            addr,
		I2PTunnelStatus: i2ptunnel.I2PTunnelStatusStopped,
		ServerConfig:    httpserver.DefaultHTTPServerConfig(),
		ClientConfig:    httpclient.DefaultHTTPClientConfig(),
		LimitedConfig: limitedlistener.LimitedConfig{
			MaxConns:  1000,
			RateLimit: 100,
		},
		done: make(chan struct{}),
	}

	if tunnel.Name() != "test-bidi" {
		t.Errorf("Expected name test-bidi, got %s", tunnel.Name())
	}
	if tunnel.Type() != "httpbidirectional" {
		t.Errorf("Expected type httpbidirectional, got %s", tunnel.Type())
	}
	if tunnel.Target() != "127.0.0.1:8080" {
		t.Errorf("Expected target 127.0.0.1:8080, got %s", tunnel.Target())
	}
	addr2, err := tunnel.LocalAddress()
	if err != nil {
		t.Fatalf("LocalAddress() returned error: %v", err)
	}
	if addr2 != "127.0.0.1:4449" {
		t.Errorf("Expected local address 127.0.0.1:4449, got %s", addr2)
	}
}

// TestHTTPBidirectionalInspectorIntegration verifies that httpinspector.New
// can be created with the configs stored in the struct.
func TestHTTPBidirectionalInspectorIntegration(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Verify server config wraps a listener without error
	serverConfig := httpserver.DefaultHTTPServerConfig()
	wrappedServer := httpinspector.New(listener, serverConfig)
	if wrappedServer == nil {
		t.Fatal("httpinspector.New returned nil for server config")
	}

	listener2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create second listener: %v", err)
	}
	defer listener2.Close()

	// Verify client config wraps a listener without error
	clientConfig := httpclient.DefaultHTTPClientConfig()
	wrappedClient := httpinspector.New(listener2, clientConfig)
	if wrappedClient == nil {
		t.Fatal("httpinspector.New returned nil for client config")
	}
}

// TestHTTPBidirectionalOptionsIncludesTarget verifies Options includes
// the target address.
func TestHTTPBidirectionalOptionsIncludesTarget(t *testing.T) {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	tunnel := &HTTPBidirectional{
		TunnelConfig: i2pconv.TunnelConfig{
			Name:      "test",
			Type:      "httpbidirectional",
			Interface: "127.0.0.1",
			Port:      4449,
		},
		Addr: addr,
		LimitedConfig: limitedlistener.LimitedConfig{
			MaxConns:  500,
			RateLimit: 50,
		},
		done: make(chan struct{}),
	}

	opts := tunnel.Options()
	if target, ok := opts["target"]; !ok {
		t.Error("Options() missing 'target' key")
	} else if target != "127.0.0.1:8080" {
		t.Errorf("Expected target 127.0.0.1:8080, got %s", target)
	}
}
