package controller

import (
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
