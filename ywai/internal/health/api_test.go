package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthEndpointReturns200(t *testing.T) {
	svc := NewService(":memory:", ":memory:")
	handler := NewHandler(svc)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}
}

func TestHealthEndpointContentType(t *testing.T) {
	svc := NewService(":memory:", ":memory:")
	handler := NewHandler(svc)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health failed: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	ct = strings.TrimSpace(ct)

	// Accept both "application/json" and "application/json; charset=utf-8"
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestHealthEndpointJSONFields(t *testing.T) {
	svc := NewService(":memory:", ":memory:")
	handler := NewHandler(svc)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}

	required := []string{"daemon_ok", "db_ok", "repo_count", "last_check"}
	for _, field := range required {
		if _, ok := body[field]; !ok {
			t.Errorf("response JSON missing required field: %s", field)
		}
	}
}

func TestHealthEndpointFieldTypes(t *testing.T) {
	svc := NewService(":memory:", ":memory:")
	handler := NewHandler(svc)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health failed: %v", err)
	}
	defer resp.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}

	// daemon_ok must be a boolean
	if _, ok := body["daemon_ok"].(bool); !ok {
		t.Errorf("daemon_ok should be boolean, got %T", body["daemon_ok"])
	}

	// db_ok must be a boolean
	if _, ok := body["db_ok"].(bool); !ok {
		t.Errorf("db_ok should be boolean, got %T", body["db_ok"])
	}

	// repo_count must be a float64 (JSON numbers decode as float64)
	if _, ok := body["repo_count"].(float64); !ok {
		t.Errorf("repo_count should be number, got %T", body["repo_count"])
	}

	// last_check must be a string (ISO 8601 timestamp)
	if _, ok := body["last_check"].(string); !ok {
		t.Errorf("last_check should be string, got %T", body["last_check"])
	}
}

func TestHealthEndpointMethodNotAllowed(t *testing.T) {
	svc := NewService(":memory:", ":memory:")
	handler := NewHandler(svc)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Post(server.URL+"/api/health", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 MethodNotAllowed for POST, got %d", resp.StatusCode)
	}
}

func TestHealthEndpointInvalidPath(t *testing.T) {
	svc := NewService(":memory:", ":memory:")
	handler := NewHandler(svc)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/healthz")
	if err != nil {
		t.Fatalf("GET /api/healthz failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unknown path, got %d", resp.StatusCode)
	}
}
