package controller

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestCSRFRejectsCrossOriginPost(t *testing.T) {
	configFile := createTestConfig(t, "csrf-test", "tcpclient", "example.i2p", 8080)
	cg, err := NewControllerGroup(filepath.Dir(configFile))
	if err != nil {
		t.Fatalf("NewControllerGroup: %v", err)
	}

	form := "action=Stop"
	req := httptest.NewRequest(http.MethodPost, "/csrf-test/control", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()

	cg.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("cross-origin POST: got %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestCSRFAllowsSameOriginPost(t *testing.T) {
	configFile := createTestConfig(t, "csrf-same", "tcpclient", "example.i2p", 8080)
	cg, err := NewControllerGroup(filepath.Dir(configFile))
	if err != nil {
		t.Fatalf("NewControllerGroup: %v", err)
	}

	form := "action=Stop"
	req := httptest.NewRequest(http.MethodPost, "/csrf-same/control", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	w := httptest.NewRecorder()

	cg.ServeHTTP(w, req)

	// Should not be 403 — the request should be processed normally.
	if w.Code == http.StatusForbidden {
		t.Errorf("same-origin POST should not be rejected, got %d", w.Code)
	}
}

func TestCSRFAllowsGet(t *testing.T) {
	configFile := createTestConfig(t, "csrf-get", "tcpclient", "example.i2p", 8080)
	cg, err := NewControllerGroup(filepath.Dir(configFile))
	if err != nil {
		t.Fatalf("NewControllerGroup: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()

	cg.ServeHTTP(w, req)

	// GET is a safe method — always allowed regardless of origin.
	if w.Code == http.StatusForbidden {
		t.Errorf("cross-origin GET should be allowed, got %d", w.Code)
	}
}

func TestCSRFRejectsCrossOriginByOriginHeader(t *testing.T) {
	configFile := createTestConfig(t, "csrf-origin", "tcpclient", "example.i2p", 8080)
	cg, err := NewControllerGroup(filepath.Dir(configFile))
	if err != nil {
		t.Fatalf("NewControllerGroup: %v", err)
	}

	form := "action=Stop"
	req := httptest.NewRequest(http.MethodPost, "/csrf-origin/control", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Host = "localhost:8089"
	req.Header.Set("Origin", "http://evil.example.com")
	w := httptest.NewRecorder()

	cg.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("cross-origin POST via Origin header: got %d, want %d", w.Code, http.StatusForbidden)
	}
}
