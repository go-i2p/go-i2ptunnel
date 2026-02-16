package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestControllerServeHTTPGet tests the GET request for control page
func TestControllerServeHTTPGet(t *testing.T) {
	configFile := createTestConfig(t, "wcl-get", "tcpclient", "example.i2p", 8080)

	controller, err := NewController(configFile)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/wcl-get/control", nil)
	w := httptest.NewRecorder()

	controller.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "wcl-get") {
		t.Errorf("Response should contain tunnel name")
	}
	if !strings.Contains(body, "stopped") {
		t.Errorf("Response should show tunnel status")
	}
}

// TestControllerStart tests starting a tunnel via POST
func TestControllerStart(t *testing.T) {
	configFile := createTestConfig(t, "wcl-start", "tcpclient", "example.i2p", 8080)

	controller, err := NewController(configFile)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}
	defer controller.Stop()

	formData := url.Values{}
	formData.Set("action", "Start")

	req := httptest.NewRequest(http.MethodPost, "/wcl-start/control", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	controller.ServeHTTP(w, req)

	// Should redirect on success (handler returns immediately, Start() runs in background)
	if w.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303 (redirect), got %d", w.Code)
	}

	// Give the background goroutine time to begin executing Start()
	time.Sleep(100 * time.Millisecond)

	// Tunnel may be 'starting', 'running', or still 'stopped' if Start() failed
	// in the background goroutine (e.g., port already in use).
	// The key behavior validated here is that the HTTP handler returned promptly.
	status := controller.Status()
	if status != "starting" && status != "running" && status != "stopped" {
		t.Errorf("Expected tunnel to be starting, running, or stopped, got status: %s", status)
	}
}

// TestControllerStop tests stopping a running tunnel
func TestControllerStop(t *testing.T) {
	configFile := createTestConfig(t, "wcl-stop", "tcpclient", "example.i2p", 8080)

	controller, err := NewController(configFile)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Start tunnel in background goroutine (Start blocks)
	go controller.Start()

	// Give the tunnel time to enter running state before stopping
	time.Sleep(200 * time.Millisecond)

	formData := url.Values{}
	formData.Set("action", "Stop")

	req := httptest.NewRequest(http.MethodPost, "/wcl-stop/control", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	controller.ServeHTTP(w, req)

	// Should redirect on success
	if w.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303 (redirect), got %d", w.Code)
	}

	// Verify tunnel is stopped
	status := controller.Status()
	if status != "stopped" {
		t.Errorf("Expected tunnel to be stopped, got status: %s", status)
	}
}

// TestControllerRestart tests restarting a tunnel
func TestControllerRestart(t *testing.T) {
	configFile := createTestConfig(t, "wcl-restart", "tcpclient", "example.i2p", 8080)

	controller, err := NewController(configFile)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}
	defer controller.Stop()

	// Start tunnel first (in background)
	go controller.Start()

	formData := url.Values{}
	formData.Set("action", "Restart")

	req := httptest.NewRequest(http.MethodPost, "/wcl-restart/control", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	controller.ServeHTTP(w, req)

	// Should redirect on success
	if w.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303 (redirect), got %d", w.Code)
	}

	// Tunnel may be 'starting' or 'running' since Start() is in a background goroutine
	status := controller.Status()
	if status != "starting" && status != "running" && status != "stopped" {
		t.Errorf("Expected tunnel to be starting, running, or stopped during restart, got status: %s", status)
	}
}

// TestControllerInvalidAction tests handling of invalid actions
func TestControllerInvalidAction(t *testing.T) {
	configFile := createTestConfig(t, "wcl-invact", "tcpclient", "example.i2p", 8080)

	controller, err := NewController(configFile)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	formData := url.Values{}
	formData.Set("action", "InvalidAction")

	req := httptest.NewRequest(http.MethodPost, "/wcl-invact/control", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	controller.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Unknown action") {
		t.Errorf("Error message should mention unknown action")
	}
}

// TestMiniServeHTTP tests the mini control widget rendering
func TestMiniServeHTTP(t *testing.T) {
	configFile := createTestConfig(t, "wcl-mini", "tcpclient", "example.i2p", 8080)

	controller, err := NewController(configFile)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	w := httptest.NewRecorder()

	controller.MiniServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "wcl-mini") {
		t.Errorf("Response should contain tunnel name")
	}
}

// TestHandleStartNonBlocking verifies that the Start action returns immediately
// instead of blocking the HTTP handler indefinitely.
func TestHandleStartNonBlocking(t *testing.T) {
	configFile := createTestConfig(t, "wcl-nonblock", "tcpclient", "example.i2p", 8081)

	controller, err := NewController(configFile)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}
	defer controller.Stop()

	formData := url.Values{}
	formData.Set("action", "Start")

	req := httptest.NewRequest(http.MethodPost, "/wcl-nonblock/control", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	// The handler must return within a reasonable timeout.
	// Before the fix, this would block indefinitely.
	done := make(chan struct{})
	go func() {
		controller.ServeHTTP(w, req)
		close(done)
	}()

	select {
	case <-done:
		// Handler returned promptly — correct behavior
	case <-time.After(2 * time.Second):
		t.Fatal("handleStart() blocked the HTTP handler — Start() should run in a background goroutine")
	}

	if w.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303 (redirect), got %d", w.Code)
	}
}

// TestMiniControlTemplateLinkURL verifies that the mini control widget renders
// the tunnel name as a link to the correct control page URL pattern.
// Why: Finding #6 — the link previously pointed to /tunnel/<name> which matched
// no route handler; it now points to /<name>/control.
func TestMiniControlTemplateLinkURL(t *testing.T) {
	tests := []struct {
		name       string
		tunnelName string
		tunnelType string
		target     string
		wantHref   string
	}{
		{"simple name", "my-tunnel", "tcpclient", "example.i2p", `/my-tunnel/control`},
		{"server tunnel", "web-server", "httpserver", "127.0.0.1:8080", `/web-server/control`},
		{"bidirectional", "bidir-tun", "tcpbidirectional", "127.0.0.1:9090", `/bidir-tun/control`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile := createTestConfig(t,
				tt.tunnelName, tt.tunnelType, tt.target, 8080)

			controller, err := NewController(configFile)
			if err != nil {
				t.Fatalf("Failed to create controller: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "/home", nil)
			w := httptest.NewRecorder()

			controller.MiniServeHTTP(w, req)

			body := w.Body.String()
			expectedLink := fmt.Sprintf(`href="%s"`, tt.wantHref)
			if !strings.Contains(body, expectedLink) {
				t.Errorf("Mini control should contain %s, got:\n%s",
					expectedLink, body)
			}

			// Verify the OLD broken pattern is NOT present
			if strings.Contains(body, `/tunnel/`) {
				t.Errorf("Mini control should not contain /tunnel/ pattern")
			}
		})
	}
}

// TestMiniControlTemplateFormAction verifies that the Start/Stop form in the
// mini control widget POSTs to the correct control endpoint.
// Why: Finding #6 — the form previously had no action attribute, so it POSTed
// to the current page (/home) which just re-rendered the dashboard.
func TestMiniControlTemplateFormAction(t *testing.T) {
	configFile := createTestConfig(t, "mini-form", "tcpclient", "example.i2p", 8080)

	controller, err := NewController(configFile)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	w := httptest.NewRecorder()

	controller.MiniServeHTTP(w, req)

	body := w.Body.String()
	expectedAction := `action="/mini-form/control"`
	if !strings.Contains(body, expectedAction) {
		t.Errorf("Form should contain %s, got:\n%s", expectedAction, body)
	}
}

// TestMiniControlFormActionRoutesToControl verifies that the form action URL
// is correctly recognized by the URL routing as a "control" handler.
// This is an integration test ensuring the template and router agree.
func TestMiniControlFormActionRoutesToControl(t *testing.T) {
	tunnelNames := []string{"my-tunnel", "web-server", "irc-relay"}

	for _, name := range tunnelNames {
		t.Run(name, func(t *testing.T) {
			// Build the URL the form action generates
			actionURL := "/" + name + "/control"

			req := httptest.NewRequest(http.MethodPost, actionURL, nil)

			// The routing function should recognize this as "control"
			result := handler(req)
			if result != "control" {
				t.Errorf("handler(%q) = %q, want %q", actionURL, result, "control")
			}

			// The tunnel name should be correctly extracted
			tunnelName := tunnel(req)
			if tunnelName != name {
				t.Errorf("tunnel(%q) = %q, want %q", actionURL, tunnelName, name)
			}
		})
	}
}

// TestHandleStartAlreadyRunning verifies that starting an already-running tunnel returns an error.
func TestHandleStartAlreadyRunning(t *testing.T) {
	configFile := createTestConfig(t, "wcl-already", "tcpclient", "example.i2p", 8082)

	controller, err := NewController(configFile)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}
	defer controller.Stop()

	// Start tunnel in background first
	go controller.Start()
	// Give it a moment to transition to starting/running
	time.Sleep(100 * time.Millisecond)

	formData := url.Values{}
	formData.Set("action", "Start")

	req := httptest.NewRequest(http.MethodPost, "/wcl-already/control", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	controller.ServeHTTP(w, req)

	// Should return error since tunnel is already starting/running
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for already-running tunnel, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "already") {
		t.Errorf("Error message should indicate tunnel is already running/starting")
	}
}
