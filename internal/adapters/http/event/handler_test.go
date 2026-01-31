package event

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appevent "3tcapital/ms_facturacion_core/internal/application/event"
	"3tcapital/ms_facturacion_core/internal/core/event"
	"3tcapital/ms_facturacion_core/internal/core/invoice"
	"3tcapital/ms_facturacion_core/internal/testutil"
)

func TestNewHandler(t *testing.T) {
	mockService := &appevent.Service{}
	handler := NewHandler(mockService)

	if handler == nil {
		t.Fatal("expected handler to be created, got nil")
	}

	if handler.service != mockService {
		t.Error("expected handler to have the provided service")
	}
}

func TestHandler_RegisterEvent_Success(t *testing.T) {
	mockProvider := &testutil.MockProvider{
		RegisterEventFunc: func(ctx context.Context, evt event.Event, emisorNit, razonSocial string) (*invoice.EventRegistrationResult, error) {
			return &invoice.EventRegistrationResult{
				Code:            "1000",
				NumeroDocumento: "FAC12345",
				Resultado: []invoice.EventResult{
					{
						TipoEvento:      "030",
						Mensaje:         "Procesado Correctamente.",
						MensajeError:    "",
						CodigoRespuesta: "1000",
					},
				},
				MensajeError: "",
			}, nil
		},
	}

	service := appevent.NewService(mockProvider, "123456789", "PROVEDOR NOMBRE")
	handler := NewHandler(service)

	reqBody := RegisterEventRequest{
		EventType:              "ACUSE",
		DocumentoNumeroCompleto: "FAC12345",
		NombreGenerador:        "Mauricio",
		ApellidoGenerador:      "Alemán",
		IdentificacionGenerador: "1061239585",
		FechaGeneracionEvento:  time.Now().Format("2006-01-02 15:04:05"),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/eventos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RegisterEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response RegisterEventResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "200" {
		t.Errorf("expected status '200', got %q", response.Status)
	}

	if response.Data == nil {
		t.Fatal("expected data, got nil")
	}

	if response.Data.Code != "1000" {
		t.Errorf("expected Code '1000', got %q", response.Data.Code)
	}
}

func TestHandler_RegisterEvent_WithRejectionCode(t *testing.T) {
	mockProvider := &testutil.MockProvider{
		RegisterEventFunc: func(ctx context.Context, evt event.Event, emisorNit, razonSocial string) (*invoice.EventRegistrationResult, error) {
			return &invoice.EventRegistrationResult{
				Code:            "1000",
				NumeroDocumento: "FAC12345",
				Resultado: []invoice.EventResult{
					{
						TipoEvento:      "031",
						Mensaje:         "Procesado Correctamente.",
						MensajeError:    "",
						CodigoRespuesta: "1000",
					},
				},
				MensajeError: "",
			}, nil
		},
	}

	service := appevent.NewService(mockProvider, "123456789", "PROVEDOR NOMBRE")
	handler := NewHandler(service)

	reqBody := RegisterEventRequest{
		EventType:              "RECLAMO",
		DocumentoNumeroCompleto: "FAC12345",
		NombreGenerador:        "Mauricio",
		ApellidoGenerador:      "Alemán",
		IdentificacionGenerador: "1061239585",
		CodigoRechazo:          "01",
		FechaGeneracionEvento:  time.Now().Format("2006-01-02 15:04:05"),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/eventos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RegisterEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_RegisterEvent_InvalidMethod(t *testing.T) {
	service := appevent.NewService(&testutil.MockProvider{}, "123456789", "PROVEDOR NOMBRE")
	handler := NewHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/eventos", nil)
	w := httptest.NewRecorder()

	handler.RegisterEvent(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandler_RegisterEvent_InvalidJSON(t *testing.T) {
	service := appevent.NewService(&testutil.MockProvider{}, "123456789", "PROVEDOR NOMBRE")
	handler := NewHandler(service)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/eventos", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RegisterEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_RegisterEvent_MissingFields(t *testing.T) {
	service := appevent.NewService(&testutil.MockProvider{}, "123456789", "PROVEDOR NOMBRE")
	handler := NewHandler(service)

	tests := []struct {
		name    string
		reqBody RegisterEventRequest
	}{
		{"missing EventType", RegisterEventRequest{DocumentoNumeroCompleto: "FAC12345"}},
		{"missing DocumentoNumeroCompleto", RegisterEventRequest{EventType: "ACUSE"}},
		{"missing NombreGenerador", RegisterEventRequest{EventType: "ACUSE", DocumentoNumeroCompleto: "FAC12345"}},
		{"missing ApellidoGenerador", RegisterEventRequest{EventType: "ACUSE", DocumentoNumeroCompleto: "FAC12345", NombreGenerador: "Mauricio"}},
		{"missing IdentificacionGenerador", RegisterEventRequest{EventType: "ACUSE", DocumentoNumeroCompleto: "FAC12345", NombreGenerador: "Mauricio", ApellidoGenerador: "Alemán"}},
		{"missing FechaGeneracionEvento", RegisterEventRequest{EventType: "ACUSE", DocumentoNumeroCompleto: "FAC12345", NombreGenerador: "Mauricio", ApellidoGenerador: "Alemán", IdentificacionGenerador: "1061239585"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/eventos", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.RegisterEvent(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", w.Code)
			}
		})
	}
}

func TestHandler_RegisterEvent_InvalidEventType(t *testing.T) {
	service := appevent.NewService(&testutil.MockProvider{}, "123456789", "PROVEDOR NOMBRE")
	handler := NewHandler(service)

	reqBody := RegisterEventRequest{
		EventType:              "INVALID",
		DocumentoNumeroCompleto: "FAC12345",
		NombreGenerador:        "Mauricio",
		ApellidoGenerador:      "Alemán",
		IdentificacionGenerador: "1061239585",
		FechaGeneracionEvento:  time.Now().Format("2006-01-02 15:04:05"),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/eventos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RegisterEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_RegisterEvent_InvalidDateFormat(t *testing.T) {
	service := appevent.NewService(&testutil.MockProvider{}, "123456789", "PROVEDOR NOMBRE")
	handler := NewHandler(service)

	reqBody := RegisterEventRequest{
		EventType:              "ACUSE",
		DocumentoNumeroCompleto: "FAC12345",
		NombreGenerador:        "Mauricio",
		ApellidoGenerador:      "Alemán",
		IdentificacionGenerador: "1061239585",
		FechaGeneracionEvento:  "invalid-date",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/eventos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RegisterEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_RegisterEvent_InvalidRejectionCode(t *testing.T) {
	service := appevent.NewService(&testutil.MockProvider{}, "123456789", "PROVEDOR NOMBRE")
	handler := NewHandler(service)

	reqBody := RegisterEventRequest{
		EventType:              "RECLAMO",
		DocumentoNumeroCompleto: "FAC12345",
		NombreGenerador:        "Mauricio",
		ApellidoGenerador:      "Alemán",
		IdentificacionGenerador: "1061239585",
		CodigoRechazo:          "99",
		FechaGeneracionEvento:  time.Now().Format("2006-01-02 15:04:05"),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/eventos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RegisterEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_RegisterEvent_ProviderError(t *testing.T) {
	mockProvider := &testutil.MockProvider{
		RegisterEventFunc: func(ctx context.Context, evt event.Event, emisorNit, razonSocial string) (*invoice.EventRegistrationResult, error) {
			return nil, errors.New("provider connection error")
		},
	}

	service := appevent.NewService(mockProvider, "123456789", "PROVEDOR NOMBRE")
	handler := NewHandler(service)

	reqBody := RegisterEventRequest{
		EventType:              "ACUSE",
		DocumentoNumeroCompleto: "FAC12345",
		NombreGenerador:        "Mauricio",
		ApellidoGenerador:      "Alemán",
		IdentificacionGenerador: "1061239585",
		FechaGeneracionEvento:  time.Now().Format("2006-01-02 15:04:05"),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/eventos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RegisterEvent(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected status 502, got %d", w.Code)
	}
}

func TestHandler_RegisterEvent_DocumentNotFound(t *testing.T) {
	mockProvider := &testutil.MockProvider{
		RegisterEventFunc: func(ctx context.Context, evt event.Event, emisorNit, razonSocial string) (*invoice.EventRegistrationResult, error) {
			return nil, errors.New("document not found: No se encontró el documento en la base de datos")
		},
	}

	service := appevent.NewService(mockProvider, "123456789", "PROVEDOR NOMBRE")
	handler := NewHandler(service)

	reqBody := RegisterEventRequest{
		EventType:              "ACUSE",
		DocumentoNumeroCompleto: "FAC99999",
		NombreGenerador:        "Mauricio",
		ApellidoGenerador:      "Alemán",
		IdentificacionGenerador: "1061239585",
		FechaGeneracionEvento:  time.Now().Format("2006-01-02 15:04:05"),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/eventos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RegisterEvent(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandler_RegisterEvent_AllEventTypes(t *testing.T) {
	eventTypes := []string{"ACUSE", "RECIBOBIEN", "ACEPTACION", "RECLAMO"}

	for _, eventType := range eventTypes {
		t.Run(eventType, func(t *testing.T) {
			mockProvider := &testutil.MockProvider{
				RegisterEventFunc: func(ctx context.Context, evt event.Event, emisorNit, razonSocial string) (*invoice.EventRegistrationResult, error) {
					return &invoice.EventRegistrationResult{
						Code:            "1000",
						NumeroDocumento:  "FAC12345",
						Resultado:       []invoice.EventResult{},
						MensajeError:     "",
					}, nil
				},
			}

			service := appevent.NewService(mockProvider, "123456789", "PROVEDOR NOMBRE")
			handler := NewHandler(service)

			reqBody := RegisterEventRequest{
				EventType:              eventType,
				DocumentoNumeroCompleto: "FAC12345",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "1061239585",
				FechaGeneracionEvento:  time.Now().Format("2006-01-02 15:04:05"),
			}

			if eventType == "RECLAMO" {
				reqBody.CodigoRechazo = "01"
			}

			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/eventos", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.RegisterEvent(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200 for %s, got %d", eventType, w.Code)
			}
		})
	}
}
