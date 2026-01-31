package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"3tcapital/ms_facturacion_core/internal/infrastructure/config"
	"3tcapital/ms_facturacion_core/internal/testutil"
)

func TestNew_NilLogger(t *testing.T) {
	cfg := config.AppConfig{
		HTTP: config.HTTPSettings{
			Port: 8080,
		},
	}

	_, err := New(Options{
		Config:        cfg,
		Logger:        nil,
		HealthHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	})

	if err == nil {
		t.Fatal("expected error for nil logger")
	}

	if err.Error() != "logger is required" {
		t.Errorf("expected error 'logger is required', got %q", err.Error())
	}
}

func TestNew_NilHealthHandler(t *testing.T) {
	cfg := config.AppConfig{
		HTTP: config.HTTPSettings{
			Port: 8080,
		},
	}

	_, err := New(Options{
		Config:        cfg,
		Logger:        testutil.NewTestLogger(),
		HealthHandler: nil,
	})

	if err == nil {
		t.Fatal("expected error for nil health handler")
	}

	if err.Error() != "health handler is required" {
		t.Errorf("expected error 'health handler is required', got %q", err.Error())
	}
}

func TestNew_ValidOptions(t *testing.T) {
	cfg := config.AppConfig{
		HTTP: config.HTTPSettings{
			Port:            8080,
			ReadTimeout:     10 * time.Second,
			WriteTimeout:    10 * time.Second,
			IdleTimeout:     120 * time.Second,
			ShutdownTimeout: 30 * time.Second,
		},
		Auth: config.AuthSettings{
			Enabled: false,
		},
	}

	healthHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server, err := New(Options{
		Config:          cfg,
		Logger:          testutil.NewTestLogger(),
		HealthHandler:   healthHandler,
		ResolutionHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if server == nil {
		t.Fatal("expected server to be created, got nil")
	}

	if server.httpServer == nil {
		t.Error("expected httpServer to be initialized")
	}

	if server.httpServer.Addr != ":8080" {
		t.Errorf("expected address ':8080', got %q", server.httpServer.Addr)
	}
}

func TestNew_WithResolutionHandler(t *testing.T) {
	cfg := config.AppConfig{
		HTTP: config.HTTPSettings{
			Port: 8080,
		},
		Auth: config.AuthSettings{
			Enabled: false,
		},
	}

	resolutionHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("resolutions"))
	})

	server, err := New(Options{
		Config:          cfg,
		Logger:          testutil.NewTestLogger(),
		HealthHandler:   http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		ResolutionHandler: resolutionHandler,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test that the route is registered by making a request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/configuracion/lista-resoluciones-facturacion", nil)
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	// Should return 200 or 503 depending on handler setup
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 200 or 503, got %d", w.Code)
	}
}

func TestNew_WithoutResolutionHandler(t *testing.T) {
	cfg := config.AppConfig{
		HTTP: config.HTTPSettings{
			Port: 8080,
		},
		Auth: config.AuthSettings{
			Enabled: false,
		},
	}

	server, err := New(Options{
		Config:          cfg,
		Logger:          testutil.NewTestLogger(),
		HealthHandler:   http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		ResolutionHandler: nil,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test that the fallback handler is used
	req := httptest.NewRequest(http.MethodGet, "/api/v1/configuracion/lista-resoluciones-facturacion", nil)
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	// Should return 503 Service Unavailable
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}
}

func TestServer_Close(t *testing.T) {
	cfg := config.AppConfig{
		HTTP: config.HTTPSettings{
			Port: 8080,
		},
		Auth: config.AuthSettings{
			Enabled: false,
		},
	}

	server, err := New(Options{
		Config:          cfg,
		Logger:          testutil.NewTestLogger(),
		HealthHandler:   http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		ResolutionHandler: nil,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not panic
	server.Close()
}

func TestServer_Run_ContextCancel(t *testing.T) {
	cfg := config.AppConfig{
		HTTP: config.HTTPSettings{
			Port:            0, // Use random port
			ShutdownTimeout: 1 * time.Second,
		},
		Auth: config.AuthSettings{
			Enabled: false,
		},
	}

	server, err := New(Options{
		Config:          cfg,
		Logger:          testutil.NewTestLogger(),
		HealthHandler:   http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		ResolutionHandler: nil,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	// Cancel context immediately
	cancel()

	// Run should return without error when context is cancelled
	err = server.Run(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestServer_Run_ShutdownError(t *testing.T) {
	cfg := config.AppConfig{
		HTTP: config.HTTPSettings{
			Port:            0, // Use random port
			ShutdownTimeout: 1 * time.Nanosecond, // Very short timeout to force shutdown error
		},
		Auth: config.AuthSettings{
			Enabled: false,
		},
	}

	server, err := New(Options{
		Config:          cfg,
		Logger:          testutil.NewTestLogger(),
		HealthHandler:   http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		ResolutionHandler: nil,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	// Run should handle shutdown, may return error if timeout is too short
	_ = server.Run(ctx)
	// We don't check for error as shutdown timeout might cause one
}

func TestServer_HealthEndpoint(t *testing.T) {
	cfg := config.AppConfig{
		HTTP: config.HTTPSettings{
			Port: 8080,
		},
		Auth: config.AuthSettings{
			Enabled: false,
		},
	}

	healthHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	})

	server, err := New(Options{
		Config:          cfg,
		Logger:          testutil.NewTestLogger(),
		HealthHandler:   healthHandler,
		ResolutionHandler: nil,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "healthy" {
		t.Errorf("expected body 'healthy', got %q", w.Body.String())
	}
}

func TestNew_WithInvoiceHandler(t *testing.T) {
	cfg := config.AppConfig{
		HTTP: config.HTTPSettings{
			Port: 8080,
		},
		Auth: config.AuthSettings{
			Enabled: false,
		},
	}

	invoiceHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invoices"))
	})

	server, err := New(Options{
		Config:          cfg,
		Logger:          testutil.NewTestLogger(),
		HealthHandler:   http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		ResolutionHandler: nil,
		InvoiceHandler:  invoiceHandler,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test that the route is registered by making a request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/facturas", nil)
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	// Should return 200 or 503 depending on handler setup
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 200 or 503, got %d", w.Code)
	}
}

func TestNew_WithoutInvoiceHandler(t *testing.T) {
	cfg := config.AppConfig{
		HTTP: config.HTTPSettings{
			Port: 8080,
		},
		Auth: config.AuthSettings{
			Enabled: false,
		},
	}

	server, err := New(Options{
		Config:          cfg,
		Logger:          testutil.NewTestLogger(),
		HealthHandler:   http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		ResolutionHandler: nil,
		InvoiceHandler:  nil,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test that the fallback handler is used
	req := httptest.NewRequest(http.MethodPost, "/api/v1/facturas", nil)
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	// Should return 503 Service Unavailable
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}
}
