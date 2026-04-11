package main

import (
	"flag"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"github.com/go-i2p/go-i2ptunnel/webui/controller"
)

// TestWebMissingConfigFlag verifies the web UI binary exits non-zero for an invalid port.
// cmd/web uses panic (not os.Exit) on errors, so a bad port triggers a non-zero exit.
func TestWebInvalidPort(t *testing.T) {
	if os.Getenv("GO_TEST_SUBPROCESS") == "1" {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
		// port 99999 is out of valid range; net.Listen will fail and main() will panic
		os.Args = []string{"prog", "-port", "99999"}
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestWebInvalidPort")
	cmd.Env = append(os.Environ(), "GO_TEST_SUBPROCESS=1")
	if err := cmd.Run(); err == nil {
		t.Fatal("expected non-zero exit for invalid port, got success")
	}
}

// TestWebSmokeEndpoints starts a real HTTP server with NewControllerGroup
// (empty config directory) and verifies that /, /healthz, and /api/status
// all return HTTP 200.
func TestWebSmokeEndpoints(t *testing.T) {
	configDir := t.TempDir()

	cg, err := controller.NewControllerGroup(configDir)
	if err != nil {
		t.Fatalf("NewControllerGroup failed: %v", err)
	}

	ts := httptest.NewServer(cg)
	defer ts.Close()

	endpoints := []string{"/", "/healthz", "/api/status"}
	for _, path := range endpoints {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Errorf("GET %s failed: %v", path, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s returned %d, want 200", path, resp.StatusCode)
		}
	}
}

// TestWebSmokeMetrics verifies the /metrics endpoint returns HTTP 200.
func TestWebSmokeMetrics(t *testing.T) {
	configDir := t.TempDir()

	cg, err := controller.NewControllerGroup(configDir)
	if err != nil {
		t.Fatalf("NewControllerGroup failed: %v", err)
	}

	ts := httptest.NewServer(cg)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /metrics returned %d, want 200", resp.StatusCode)
	}
}

// TestWebListenAndServe verifies that the web UI can bind to a random port
// and serve requests, mimicking the main() flow without flag parsing.
func TestWebListenAndServe(t *testing.T) {
	configDir := t.TempDir()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen failed: %v", err)
	}

	cg, err := controller.NewControllerGroup(configDir)
	if err != nil {
		ln.Close()
		t.Fatalf("NewControllerGroup failed: %v", err)
	}

	go http.Serve(ln, cg)
	defer ln.Close()

	resp, err := http.Get("http://" + ln.Addr().String() + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /healthz returned %d, want 200", resp.StatusCode)
	}
}
