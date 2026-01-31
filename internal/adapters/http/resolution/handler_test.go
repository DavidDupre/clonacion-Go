package resolution

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	appresolution "3tcapital/ms_facturacion_core/internal/application/resolution"
	"3tcapital/ms_facturacion_core/internal/core/resolution"
	"3tcapital/ms_facturacion_core/internal/testutil"
)

func TestNewHandler(t *testing.T) {
	service := &appresolution.Service{}
	emisorNit := "860011153"
	handler := NewHandler(service, emisorNit)

	if handler == nil {
		t.Fatal("expected handler to be created, got nil")
	}

	if handler.service != service {
		t.Error("expected handler to have the provided service")
	}

	if handler.emisorNit != emisorNit {
		t.Errorf("expected handler to have emisorNit %s, got %s", emisorNit, handler.emisorNit)
	}
}

func TestHandler_GetResolutions(t *testing.T) {
	tests := []struct {
		name           string
		emisorNit      string
		setupService   func() *appresolution.Service
		expectedStatus int
		expectedBody   func(t *testing.T, body map[string]interface{})
	}{
		{
			name:      "success with valid NIT",
			emisorNit: "123456789",
			setupService: func() *appresolution.Service {
				mockProvider := &testutil.MockProvider{
					GetResolutionsFunc: func(ctx context.Context, nit string) ([]resolution.Resolution, error) {
						return []resolution.Resolution{
							{ResolutionNumber: "RES001"},
						}, nil
					},
				}
				return appresolution.NewService(mockProvider)
			},
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				resolutions, ok := body["resolutions"].([]interface{})
				if !ok {
					t.Fatal("expected resolutions array")
				}
				if len(resolutions) != 1 {
					t.Errorf("expected 1 resolution, got %d", len(resolutions))
				}
			},
		},
		{
			name:      "empty NIT configuration",
			emisorNit: "",
			setupService: func() *appresolution.Service {
				mockProvider := &testutil.MockProvider{}
				return appresolution.NewService(mockProvider)
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["message"] != "Error de Validación" {
					t.Errorf("expected message 'Error de Validación', got %v", body["message"])
				}
			},
		},
		{
			name:      "empty result",
			emisorNit: "123456789",
			setupService: func() *appresolution.Service {
				mockProvider := &testutil.MockProvider{
					GetResolutionsFunc: func(ctx context.Context, nit string) ([]resolution.Resolution, error) {
						return []resolution.Resolution{}, nil
					},
				}
				return appresolution.NewService(mockProvider)
			},
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				resolutions, ok := body["resolutions"].([]interface{})
				if !ok {
					t.Fatal("expected resolutions array")
				}
				if len(resolutions) != 0 {
					t.Errorf("expected 0 resolutions, got %d", len(resolutions))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := tt.setupService()
			handler := NewHandler(service, tt.emisorNit)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/configuracion/lista-resoluciones-facturacion", nil)

			w := httptest.NewRecorder()
			handler.GetResolutions(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var body map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if tt.expectedBody != nil {
				tt.expectedBody(t, body)
			}
		})
	}
}

func TestHandler_handleError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedMsg    string
		expectedErr    string
	}{
		{
			name:           "nit is required error",
			err:            errors.New("nit is required"),
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Error de Validación",
			expectedErr:    "El parámetro NIT es requerido",
		},
		{
			name:           "invalid nit format error",
			err:            errors.New("invalid nit format: must be between 9 and 15 characters"),
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Error de Validación",
			expectedErr:    "Formato de NIT inválido",
		},
		{
			name:           "authentication failed error",
			err:            errors.New("authentication failed"),
			expectedStatus: http.StatusBadGateway,
			expectedMsg:    "Error de Autenticación",
			expectedErr:    "Error de autenticación con el proveedor",
		},
		{
			name:           "numrot authentication failed error",
			err:            errors.New("numrot authentication failed"),
			expectedStatus: http.StatusBadGateway,
			expectedMsg:    "Error de Autenticación",
			expectedErr:    "Error de autenticación con el proveedor",
		},
		{
			name:           "get authentication token error",
			err:            errors.New("get authentication token: failed"),
			expectedStatus: http.StatusBadGateway,
			expectedMsg:    "Error de Autenticación",
			expectedErr:    "Error de autenticación con el proveedor",
		},
		{
			name:           "unexpected status code error",
			err:            errors.New("unexpected status code 500"),
			expectedStatus: http.StatusBadGateway,
			expectedMsg:    "Error del Proveedor",
			expectedErr:    "Servicio del proveedor no disponible",
		},
		{
			name:           "execute request error",
			err:            errors.New("execute request: timeout"),
			expectedStatus: http.StatusBadGateway,
			expectedMsg:    "Error del Proveedor",
			expectedErr:    "Servicio del proveedor no disponible",
		},
		{
			name:           "read response body error",
			err:            errors.New("read response body: EOF"),
			expectedStatus: http.StatusBadGateway,
			expectedMsg:    "Error del Proveedor",
			expectedErr:    "Servicio del proveedor no disponible",
		},
		{
			name:           "numrot API error",
			err:            errors.New("numrot API error: invalid request"),
			expectedStatus: http.StatusBadGateway,
			expectedMsg:    "Error del Proveedor",
			expectedErr:    "numrot API error: invalid request",
		},
		{
			name:           "unmarshal response error",
			err:            errors.New("unmarshal response: invalid JSON"),
			expectedStatus: http.StatusBadGateway,
			expectedMsg:    "Error del Proveedor",
			expectedErr:    "Error en el formato de respuesta del proveedor",
		},
		{
			name:           "provider error",
			err:            errors.New("provider error: connection failed"),
			expectedStatus: http.StatusBadGateway,
			expectedMsg:    "Error del Proveedor",
			expectedErr:    "Servicio del proveedor no disponible",
		},
		{
			name:           "unknown error",
			err:            errors.New("unknown error"),
			expectedStatus: http.StatusInternalServerError,
			expectedMsg:    "Error Interno del Servidor",
			expectedErr:    "Ha ocurrido un error interno",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(&appresolution.Service{}, "860011153")
			w := httptest.NewRecorder()

			handler.handleError(w, tt.err)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if response["message"] != tt.expectedMsg {
				t.Errorf("expected message %q, got %q", tt.expectedMsg, response["message"])
			}

			errors, ok := response["errors"].([]interface{})
			if !ok || len(errors) == 0 {
				t.Fatal("expected errors array with at least one error")
			}

			if errors[0].(string) != tt.expectedErr {
				t.Errorf("expected error %q, got %q", tt.expectedErr, errors[0].(string))
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "case insensitive match",
			s:        "Hello World",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "case insensitive match uppercase",
			s:        "HELLO WORLD",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "exact match",
			s:        "test string",
			substr:   "test",
			expected: true,
		},
		{
			name:     "no match",
			s:        "test string",
			substr:   "notfound",
			expected: false,
		},
		{
			name:     "empty string",
			s:        "",
			substr:   "test",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, expected %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}
