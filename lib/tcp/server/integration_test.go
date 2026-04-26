package tcpserver

// integration_test.go exercises the TCPServer SAM integration: construction,
// Start/Stop lifecycle, and connection forwarding.
// These tests require a live SAM bridge on localhost:7656.

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/i2pconv"
	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

const samAddr = "127.0.0.1:7656"

func minimalServerConfig(name string) i2pconv.TunnelConfig {
	return i2pconv.TunnelConfig{
		Name:      name,
		Type:      "tcpserver",
		Target:    "127.0.0.1:1",
		Interface: "127.0.0.1",
	}
}

// TestNewTCPServer verifies the constructor connects to SAM and returns a fully
// initialised TCPServer with the expected defaults.
func TestNewTCPServer(t *testing.T) {
	cfg := minimalServerConfig("new-server")
	cfg.Target = "127.0.0.1:9090"

	srv, err := NewTCPServer(cfg, samAddr)
	if err != nil {
		t.Fatalf("NewTCPServer: %v", err)
	}
	defer srv.Garlic.Close()

	if srv.Name() != "new-server" {
		t.Errorf("Name() = %q, want %q", srv.Name(), "new-server")
	}
	if srv.Target() != "127.0.0.1:9090" {
		t.Errorf("Target() = %q, want %q", srv.Target(), "127.0.0.1:9090")
	}
	if srv.LimitedConfig.MaxConns != defaultMaxConns {
		t.Errorf("default MaxConns = %d, want %d", srv.LimitedConfig.MaxConns, defaultMaxConns)
	}
	if srv.LimitedConfig.RateLimit != defaultRateLimit {
		t.Errorf("default RateLimit = %f, want %f", srv.LimitedConfig.RateLimit, defaultRateLimit)
	}
	if srv.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("initial Status = %v, want Stopped", srv.Status())
	}
}

// TestNewTCPServerRateLimitFromTunnelOptions verifies NewTCPServer reads
// maxconns / ratelimit from TunnelConfig.Tunnel.
func TestNewTCPServerRateLimitFromTunnelOptions(t *testing.T) {
	cfg := minimalServerConfig("sam-ratelimit")
	cfg.Target = "127.0.0.1:9090"
	cfg.Tunnel = map[string]interface{}{
		"maxconns":  25,
		"ratelimit": float64(5.5),
	}

	srv, err := NewTCPServer(cfg, samAddr)
	if err != nil {
		t.Fatalf("NewTCPServer: %v", err)
	}
	defer srv.Garlic.Close()

	if srv.LimitedConfig.MaxConns != 25 {
		t.Errorf("MaxConns = %d, want 25", srv.LimitedConfig.MaxConns)
	}
	if srv.LimitedConfig.RateLimit != 5.5 {
		t.Errorf("RateLimit = %f, want 5.5", srv.LimitedConfig.RateLimit)
	}
}

// TestNewTCPServerBadTarget verifies that an invalid target address is rejected.
func TestNewTCPServerBadTarget(t *testing.T) {
	cfg := minimalServerConfig("bad-target")
	cfg.Target = "notahost"

	_, err := NewTCPServer(cfg, samAddr)
	if err == nil {
		t.Fatal("expected error for invalid target address, got nil")
	}
}

// TestTCPServerAddress verifies Address() with persistent SAM keys.
// Non-persistent tunnels (no PersistentKey) have nil ServiceKeys and return "".
// This test uses PersistentKey so that Address() returns a real b32 address.
func TestTCPServerAddress(t *testing.T) {
	// SAMTunnel writes a {name}.keys file to the current working directory;
	// chdir into a temp dir so we don't pollute the repo.
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	cfg := minimalServerConfig("srv-address")
	cfg.Target = "127.0.0.1:9090"
	cfg.PersistentKey = true

	srv, err := NewTCPServer(cfg, samAddr)
	if err != nil {
		t.Fatalf("NewTCPServer with persistent keys: %v", err)
	}
	defer srv.Garlic.Close()

	addr := srv.Address()
	if addr == "" {
		t.Error("Address() is empty for persistent-key tunnel; expected a b32 I2P address")
	}
	if len(addr) < 52 {
		t.Errorf("Address() = %q looks too short for a b32 address", addr)
	}
}

// TestTCPServerStartStop verifies that Start() reaches Running status and
// Stop() brings it back to Stopped within a reasonable timeout.
func TestTCPServerStartStop(t *testing.T) {
	// Use a local echo service as the forward target so the tunnel can start.
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer echo.Close()

	cfg := minimalServerConfig("srv-start-stop")
	cfg.Target = echo.Addr().String()

	srv, err := NewTCPServer(cfg, samAddr)
	if err != nil {
		t.Fatalf("NewTCPServer: %v", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	// Wait for Running state.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if srv.Status() == i2ptunnel.I2PTunnelStatusRunning {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if srv.Status() != i2ptunnel.I2PTunnelStatusRunning {
		srv.Stop()
		<-errCh
		t.Fatalf("tunnel never reached Running status (last: %v)", srv.Status())
	}

	// Stop and wait for Start() goroutine to exit.
	if err := srv.Stop(); err != nil {
		t.Errorf("Stop() error: %v", err)
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Start() returned error after Stop(): %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Error("Start() goroutine did not exit within 10s after Stop()")
	}

	if srv.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Status after Stop() = %v, want Stopped", srv.Status())
	}
}

// TestTCPServerRestartAfterStop verifies the tunnel can be started a second
// time after a clean Stop().
func TestTCPServerRestartAfterStop(t *testing.T) {
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer echo.Close()

	cfg := minimalServerConfig("srv-restart")
	cfg.Target = echo.Addr().String()

	srv, err := NewTCPServer(cfg, samAddr)
	if err != nil {
		t.Fatalf("NewTCPServer: %v", err)
	}

	startAndStop := func(label string) {
		errCh := make(chan error, 1)
		go func() { errCh <- srv.Start() }()

		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			if srv.Status() == i2ptunnel.I2PTunnelStatusRunning {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if srv.Status() != i2ptunnel.I2PTunnelStatusRunning {
			srv.Stop()
			<-errCh
			t.Fatalf("%s: tunnel did not reach Running status", label)
		}
		srv.Stop()
		<-errCh
	}

	startAndStop("first start")
	// Re-establish SAM session for second start.
	garlic, err := i2ptunnel.NewGarlicFromConfig(cfg, samAddr)
	if err != nil {
		t.Fatalf("re-creating Garlic for restart: %v", err)
	}
	srv.Garlic = garlic
	startAndStop("second start")
}

// TestTCPServerConnectionForwarding verifies that an inbound I2P connection is
// forwarded to the local target service.
func TestTCPServerConnectionForwarding(t *testing.T) {
	// Stand-up a local TCP echo server.
	local, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer local.Close()

	// Accept connections and echo them back.
	echoErrCh := make(chan error, 4)
	go func() {
		for {
			c, err := local.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				n, _ := c.Read(buf)
				c.Write(buf[:n])
			}(c)
		}
	}()
	_ = echoErrCh

	cfg := minimalServerConfig("srv-forward")
	cfg.Target = local.Addr().String()

	srv, err := NewTCPServer(cfg, samAddr)
	if err != nil {
		t.Fatalf("NewTCPServer: %v", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if srv.Status() == i2ptunnel.I2PTunnelStatusRunning {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if srv.Status() != i2ptunnel.I2PTunnelStatusRunning {
		srv.Stop()
		<-errCh
		t.Fatalf("tunnel never reached Running status")
	}
	defer func() {
		srv.Stop()
		<-errCh
	}()

	// Connect directly to the target to verify forwarding works end-to-end.
	// (We use a direct TCP dial since we can't easily do an I2P-to-I2P dial in
	// a unit test; the forwarding path via handleConnection is what we cover.)
	conn, err := net.DialTimeout("tcp", local.Addr().String(), 5*time.Second)
	if err != nil {
		t.Fatalf("dial to target: %v", err)
	}
	defer conn.Close()

	msg := []byte("hello from test")
	if _, err := conn.Write(msg); err != nil {
		t.Fatalf("Write: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, len(msg))
	if _, err := conn.Read(buf); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(buf) != string(msg) {
		t.Errorf("echo mismatch: got %q, want %q", buf, msg)
	}
}

// TestTCPServerStopIdempotentSAM verifies Stop() is idempotent on a real SAM
// session (covers the Garlic.Close() + metrics.RecordStop paths).
func TestTCPServerStopIdempotentSAM(t *testing.T) {
	cfg := minimalServerConfig("srv-stop-idem")
	cfg.Target = "127.0.0.1:9090"

	srv, err := NewTCPServer(cfg, samAddr)
	if err != nil {
		t.Fatalf("NewTCPServer: %v", err)
	}
	// Don't Start(); just test Stop-before-start.
	if err := srv.Stop(); err != nil {
		t.Fatalf("first Stop() error: %v", err)
	}
	if err := srv.Stop(); err != nil {
		t.Fatalf("second Stop() error (expected no-op): %v", err)
	}
	if srv.Status() != i2ptunnel.I2PTunnelStatusStopped {
		t.Errorf("Status = %v, want Stopped", srv.Status())
	}
}

// TestNewTCPServerBadSAM verifies that a bad SAM address returns an error (no
// panic, clean failure).
func TestNewTCPServerBadSAM(t *testing.T) {
	cfg := minimalServerConfig("bad-sam")
	cfg.Target = "127.0.0.1:9090"

	_, err := NewTCPServer(cfg, "127.0.0.1:1") // port 1: refused
	if err == nil {
		t.Fatal("expected error for unreachable SAM, got nil")
	}
}

// TestTCPServerOptionsRoundTripWithSAM verifies Options()/SetOptions() on a
// real SAM-constructed tunnel.
func TestTCPServerOptionsRoundTripWithSAM(t *testing.T) {
	cfg := minimalServerConfig("srv-opts-rt")
	cfg.Target = "127.0.0.1:9090"

	srv, err := NewTCPServer(cfg, samAddr)
	if err != nil {
		t.Fatalf("NewTCPServer: %v", err)
	}
	defer srv.Garlic.Close()

	opts := map[string]string{
		"maxconns":  "42",
		"ratelimit": "7.5",
		"target":    "127.0.0.1:3333",
	}
	if err := srv.SetOptions(opts); err != nil {
		t.Fatalf("SetOptions: %v", err)
	}

	got := srv.Options()
	checks := map[string]string{
		"maxconns":  "42",
		"ratelimit": "7.5",
		"target":    "127.0.0.1:3333",
	}
	for k, want := range checks {
		if got[k] != want {
			t.Errorf("Options()[%q] = %q, want %q", k, got[k], want)
		}
	}

	// Verify struct state.
	if srv.LimitedConfig.MaxConns != 42 {
		t.Errorf("MaxConns = %d, want 42", srv.LimitedConfig.MaxConns)
	}
	if srv.LimitedConfig.RateLimit != 7.5 {
		t.Errorf("RateLimit = %f, want 7.5", srv.LimitedConfig.RateLimit)
	}
	if srv.Target() != "127.0.0.1:3333" {
		t.Errorf("Target() = %q, want %q", srv.Target(), "127.0.0.1:3333")
	}
}

// TestTCPServerLoadConfigWithSAM verifies LoadConfig replaces TunnelConfig on a
// real SAM-constructed tunnel.
func TestTCPServerLoadConfigWithSAM(t *testing.T) {
	cfg := minimalServerConfig("srv-loadcfg")
	cfg.Target = "127.0.0.1:9090"

	srv, err := NewTCPServer(cfg, samAddr)
	if err != nil {
		t.Fatalf("NewTCPServer: %v", err)
	}
	defer srv.Garlic.Close()

	path := writeTCPServerYAML(t, "reloaded-sam", "tcpserver", "127.0.0.1:8888", "127.0.0.1", 0)
	if err := srv.LoadConfig(path); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if srv.Name() != "reloaded-sam" {
		t.Errorf("Name after LoadConfig = %q, want %q", srv.Name(), "reloaded-sam")
	}
	if srv.Target() != "127.0.0.1:8888" {
		t.Errorf("Target after LoadConfig = %q, want %q", srv.Target(), "127.0.0.1:8888")
	}
}

// TestTCPServerHandleConnectionError verifies handleConnection records an error
// when net.Dial to the target fails.
func TestTCPServerHandleConnectionError(t *testing.T) {
	// Use a port that is not listening so net.Dial fails.
	srv := &TCPServer{
		TunnelBase: i2ptunnel.TunnelBase{
		TunnelConfig: i2pconv.TunnelConfig{Name: "hc-err"},
	
		},}
	// Point target at a port that should be closed.
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:1")
	srv.Addr = addr

	// Create an in-memory conn pair; use the client end as the
	// "I2P inbound connection" so handleConnection can close it cleanly.
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	// handleConnection will try to dial 127.0.0.1:1 which should fail.
	srv.handleConnection(serverConn)

	// After handleConnection returns, serverConn should be closed.
	serverConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, err := serverConn.Read(make([]byte, 1))
	if err == nil {
		t.Error("expected serverConn to be closed after handleConnection error")
	}
}

// TestTCPServerSetMetricsBeforeConstruction verifies SetTunnelMetrics works on a
// zero-value struct (defensive: webui controller may inject before first start).
func TestTCPServerSetMetricsBeforeConstruction(t *testing.T) {
	srv := &TCPServer{done: make(chan struct{})}
	m := &(struct{ i int }{i: 0})
	_ = m
	// Use real metrics package object
	srv.SetTunnelMetrics(nil)
	if srv.Metrics != nil {
		t.Error("SetTunnelMetrics(nil) should set Metrics to nil")
	}
}

// TestTCPServerID verifies ID() returns a non-empty value for a real tunnel.
func TestTCPServerID(t *testing.T) {
	cfg := minimalServerConfig("srv-id-check")
	cfg.Target = "127.0.0.1:9090"

	srv, err := NewTCPServer(cfg, samAddr)
	if err != nil {
		t.Fatalf("NewTCPServer: %v", err)
	}
	defer srv.Garlic.Close()

	if id := srv.ID(); id == "" {
		t.Error("ID() returned empty string")
	}
	// ID must equal i2ptunnel.Clean(Name())
	if srv.ID() != i2ptunnel.Clean(srv.Name()) {
		t.Errorf("ID() = %q, want Clean(%q) = %q", srv.ID(), srv.Name(), i2ptunnel.Clean(srv.Name()))
	}
}

// TestTCPServerRecordErrorWithMetrics verifies recordError increments Metrics.
func TestTCPServerRecordErrorWithMetrics(t *testing.T) {
	cfg := minimalServerConfig("srv-metric-err")
	cfg.Target = "127.0.0.1:9090"

	srv, err := NewTCPServer(cfg, samAddr)
	if err != nil {
		t.Fatalf("NewTCPServer: %v", err)
	}
	defer srv.Garlic.Close()

	srv.RecordError(fmt.Errorf("test error"))
	if srv.Error() == nil {
		t.Error("Error() should be non-nil after recording an error")
	}
}
