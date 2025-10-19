package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestControllerServeHTTPGet tests the GET request for control page
func TestControllerServeHTTPGet(t *testing.T) {
	configFile := createTestConfig(t, "test-tcp-client", "tcpclient", "example.i2p", 8080)

	controller, err := NewController(configFile)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test-tcp-client/control", nil)
	w := httptest.NewRecorder()

	controller.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "test-tcp-client") {
		t.Errorf("Response should contain tunnel name")
	}
	if !strings.Contains(body, "stopped") {
		t.Errorf("Response should show tunnel status")
	}
}

// TestControllerStart tests starting a tunnel via POST
func TestControllerStart(t *testing.T) {
	t.Skip("Skipping test that requires I2P router connection - known i2cp.leaseSetEncType duplicate parameter issue")

	configFile := createTestConfig(t, "test-tcp-client", "tcpclient", "example.i2p", 8080)

	controller, err := NewController(configFile)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}
	defer controller.Stop()

	formData := url.Values{}
	formData.Set("action", "Start")

	req := httptest.NewRequest(http.MethodPost, "/test-tcp-client/control", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	controller.ServeHTTP(w, req)

	// Should redirect on success
	if w.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303 (redirect), got %d", w.Code)
	}

	// Verify tunnel is running
	status := controller.Status()
	if status != "running" {
		t.Errorf("Expected tunnel to be running, got status: %s", status)
	}
}

// TestControllerStop tests stopping a running tunnel
func TestControllerStop(t *testing.T) {
	t.Skip("Skipping test that requires I2P router connection - known i2cp.leaseSetEncType duplicate parameter issue")

	configFile := createTestConfig(t, "test-tcp-client", "tcpclient", "example.i2p", 8080)

	controller, err := NewController(configFile)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Start tunnel first
	if err := controller.Start(); err != nil {
		t.Fatalf("Failed to start tunnel: %v", err)
	}

	formData := url.Values{}
	formData.Set("action", "Stop")

	req := httptest.NewRequest(http.MethodPost, "/test-tcp-client/control", strings.NewReader(formData.Encode()))
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
	t.Skip("Skipping test that requires I2P router connection - known i2cp.leaseSetEncType duplicate parameter issue")

	configFile := createTestConfig(t, "test-tcp-client", "tcpclient", "example.i2p", 8080)

	controller, err := NewController(configFile)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}
	defer controller.Stop()

	// Start tunnel first
	if err := controller.Start(); err != nil {
		t.Fatalf("Failed to start tunnel: %v", err)
	}

	formData := url.Values{}
	formData.Set("action", "Restart")

	req := httptest.NewRequest(http.MethodPost, "/test-tcp-client/control", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	controller.ServeHTTP(w, req)

	// Should redirect on success
	if w.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303 (redirect), got %d", w.Code)
	}

	// Verify tunnel is running after restart
	status := controller.Status()
	if status != "running" {
		t.Errorf("Expected tunnel to be running after restart, got status: %s", status)
	}
}

// TestControllerInvalidAction tests handling of invalid actions
func TestControllerInvalidAction(t *testing.T) {
	configFile := createTestConfig(t, "test-tcp-client", "tcpclient", "example.i2p", 8080)

	controller, err := NewController(configFile)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	formData := url.Values{}
	formData.Set("action", "InvalidAction")

	req := httptest.NewRequest(http.MethodPost, "/test-tcp-client/control", strings.NewReader(formData.Encode()))
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
	configFile := createTestConfig(t, "test-tcp-client", "tcpclient", "example.i2p", 8080)

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
	if !strings.Contains(body, "test-tcp-client") {
		t.Errorf("Response should contain tunnel name")
	}
}
