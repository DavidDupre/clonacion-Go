package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apphealth "3tcapital/ms_facturacion_core/internal/application/health"
	corehealth "3tcapital/ms_facturacion_core/internal/core/health"
)

func TestNewHandler(t *testing.T) {
	service := &apphealth.Service{}
	handler := NewHandler(service)

	if handler == nil {
		t.Fatal("expected handler to be created, got nil")
	}

	if handler.service != service {
		t.Error("expected handler to have the provided service")
	}
}

func TestHandler_Status(t *testing.T) {
	meta := apphealth.Metadata{
		Service:     "test-service",
		Version:     "1.0.0",
		Environment: "test",
	}

	service := apphealth.NewService(meta)
	handler := NewHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.Status(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	var status corehealth.Status
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if status.Service != meta.Service {
		t.Errorf("expected service %q, got %q", meta.Service, status.Service)
	}

	if status.Version != meta.Version {
		t.Errorf("expected version %q, got %q", meta.Version, status.Version)
	}

	if status.Environment != meta.Environment {
		t.Errorf("expected environment %q, got %q", meta.Environment, status.Environment)
	}

	if status.Status != "UP" {
		t.Errorf("expected status 'UP', got %q", status.Status)
	}
}
