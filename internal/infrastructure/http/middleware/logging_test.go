package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"3tcapital/ms_facturacion_core/internal/testutil"
)

func TestRequestLogger(t *testing.T) {
	logger := testutil.NewTestLogger()
	middleware := RequestLogger(logger)

	tests := []struct {
		name           string
		statusCode     int
		expectedLogLevel string // "info", "warn", "error"
		setupRequest   func() *http.Request
	}{
		{
			name:       "2xx status logs as info",
			statusCode: http.StatusOK,
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/test", nil)
			},
		},
		{
			name:       "3xx status logs as info",
			statusCode: http.StatusMovedPermanently,
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/test", nil)
			},
		},
		{
			name:       "4xx status logs as warn",
			statusCode: http.StatusBadRequest,
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/test", nil)
			},
		},
		{
			name:       "5xx status logs as error",
			statusCode: http.StatusInternalServerError,
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/test", nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()
			w := httptest.NewRecorder()

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte("test response"))
			}))

			handler.ServeHTTP(w, req)

			if w.Code != tt.statusCode {
				t.Errorf("expected status code %d, got %d", tt.statusCode, w.Code)
			}
		})
	}
}

func TestRequestLogger_WithRequestID(t *testing.T) {
	logger := testutil.NewTestLogger()
	middleware := RequestLogger(logger)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rctx := chi.NewRouteContext()
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = req.WithContext(context.WithValue(req.Context(), chimw.RequestIDKey, "test-request-id"))

	w := httptest.NewRecorder()

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRequestLogger_WithUserAgent(t *testing.T) {
	logger := testutil.NewTestLogger()
	middleware := RequestLogger(logger)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("User-Agent", "test-agent/1.0")

	w := httptest.NewRecorder()

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
	}
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	base := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: base,
		statusCode:     http.StatusOK,
	}

	rw.WriteHeader(http.StatusNotFound)

	if rw.statusCode != http.StatusNotFound {
		t.Errorf("expected status code %d, got %d", http.StatusNotFound, rw.statusCode)
	}

	if base.Code != http.StatusNotFound {
		t.Errorf("expected base status code %d, got %d", http.StatusNotFound, base.Code)
	}
}

func TestResponseWriter_Write(t *testing.T) {
	base := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: base,
		statusCode:     0, // Not set yet
	}

	data := []byte("test data")
	n, err := rw.Write(data)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if n != len(data) {
		t.Errorf("expected to write %d bytes, got %d", len(data), n)
	}

	if rw.statusCode != http.StatusOK {
		t.Errorf("expected default status code %d, got %d", http.StatusOK, rw.statusCode)
	}

	if rw.bytesWritten != int64(len(data)) {
		t.Errorf("expected bytesWritten %d, got %d", len(data), rw.bytesWritten)
	}
}

func TestResponseWriter_Write_AfterWriteHeader(t *testing.T) {
	base := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: base,
		statusCode:     http.StatusCreated,
	}

	data := []byte("test")
	rw.Write(data)

	if rw.statusCode != http.StatusCreated {
		t.Errorf("expected status code to remain %d, got %d", http.StatusCreated, rw.statusCode)
	}
}
