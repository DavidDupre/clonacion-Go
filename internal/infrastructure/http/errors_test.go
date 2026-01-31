package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"3tcapital/ms_facturacion_core/internal/testutil"
)

// failingResponseWriter is a ResponseWriter that can simulate write failures
type failingResponseWriter struct {
	http.ResponseWriter
	failOnWrite bool
}

func (f *failingResponseWriter) Write(p []byte) (int, error) {
	if f.failOnWrite {
		// Return an error to simulate write failure
		return 0, &json.MarshalerError{}
	}
	return f.ResponseWriter.Write(p)
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		message        string
		errors         []string
		withLogger     bool
		expectedStatus int
		expectedBody   ErrorResponse
	}{
		{
			name:           "valid error response",
			statusCode:     http.StatusBadRequest,
			message:        "Error de Validación",
			errors:         []string{"El parámetro NIT es requerido"},
			withLogger:     true,
			expectedStatus: http.StatusBadRequest,
			expectedBody: ErrorResponse{
				Message: "Error de Validación",
				Errors:  []string{"El parámetro NIT es requerido"},
			},
		},
		{
			name:           "multiple errors",
			statusCode:     http.StatusUnprocessableEntity,
			message:        "Error de Validación",
			errors:         []string{"Error 1", "Error 2", "Error 3"},
			withLogger:     false,
			expectedStatus: http.StatusUnprocessableEntity,
			expectedBody: ErrorResponse{
				Message: "Error de Validación",
				Errors:  []string{"Error 1", "Error 2", "Error 3"},
			},
		},
		{
			name:           "empty errors array",
			statusCode:     http.StatusInternalServerError,
			message:        "Error Interno",
			errors:         []string{},
			withLogger:     true,
			expectedStatus: http.StatusInternalServerError,
			expectedBody: ErrorResponse{
				Message: "Error Interno",
				Errors:  []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			var logger *slog.Logger
			if tt.withLogger {
				logger = testutil.NewTestLogger()
			}

			WriteError(w, tt.statusCode, tt.message, tt.errors, logger)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status code %d, got %d", tt.expectedStatus, w.Code)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", contentType)
			}

			var response ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if response.Message != tt.expectedBody.Message {
				t.Errorf("expected message %q, got %q", tt.expectedBody.Message, response.Message)
			}

			if len(response.Errors) != len(tt.expectedBody.Errors) {
				t.Errorf("expected %d errors, got %d", len(tt.expectedBody.Errors), len(response.Errors))
			}

			for i, expectedErr := range tt.expectedBody.Errors {
				if i < len(response.Errors) && response.Errors[i] != expectedErr {
					t.Errorf("expected error[%d] %q, got %q", i, expectedErr, response.Errors[i])
				}
			}
		})
	}
}

func TestWriteError_WithNilLogger(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusBadRequest, "Test", []string{"Error"}, nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestWriteError_JSONEncodingError tests the error path when JSON encoding fails
// This is difficult to test directly, but we can verify the function handles it gracefully
func TestWriteError_JSONEncodingError(t *testing.T) {
	// Create a response writer that will fail on Write
	w := &failingResponseWriter{
		ResponseWriter: httptest.NewRecorder(),
		failOnWrite:    true,
	}

	logger := testutil.NewTestLogger()
	WriteError(w, http.StatusBadRequest, "Test", []string{"Error"}, logger)

	// Function should not panic even if encoding fails
	// The error is logged but the function completes
}
