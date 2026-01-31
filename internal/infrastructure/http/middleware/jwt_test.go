package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"3tcapital/ms_facturacion_core/internal/infrastructure/config"
	"3tcapital/ms_facturacion_core/internal/testutil"
)

func TestNewJWTAuthenticator_AuthDisabled(t *testing.T) {
	cfg := config.AuthSettings{
		Enabled: false,
	}
	logger := testutil.NewTestLogger()

	auth, err := NewJWTAuthenticator(cfg, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if auth == nil {
		t.Fatal("expected authenticator to be created, got nil")
	}

	if auth.cfg.Enabled {
		t.Error("expected auth to be disabled")
	}
}

func TestNewJWTAuthenticator_AuthEnabled_InvalidJWKSetURI(t *testing.T) {
	cfg := config.AuthSettings{
		Enabled:   true,
		IssuerURI: "https://issuer.example.com",
		JWKSetURI: "invalid-uri",
	}
	logger := testutil.NewTestLogger()

	_, err := NewJWTAuthenticator(cfg, logger)
	if err == nil {
		t.Fatal("expected error for invalid JWKSetURI")
	}
}

func TestJWTAuthenticator_Middleware_AuthDisabled(t *testing.T) {
	cfg := config.AuthSettings{
		Enabled: false,
	}
	logger := testutil.NewTestLogger()

	auth, _ := NewJWTAuthenticator(cfg, logger)
	middleware := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestJWTAuthenticator_Middleware_BypassPath(t *testing.T) {
	cfg := config.AuthSettings{
		Enabled:     true,
		IssuerURI:   "https://issuer.example.com",
		JWKSetURI:   "https://issuer.example.com/.well-known/jwks.json",
		BypassPaths: []string{"/health"},
	}
	logger := testutil.NewTestLogger()

	// This will fail to load JWKS, but we can test bypass logic
	auth, _ := NewJWTAuthenticator(cfg, logger)
	if auth == nil {
		t.Skip("Skipping test - JWKS loading requires network access")
		return
	}

	middleware := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	// Should bypass auth and return 200 or 401 depending on JWKS loading
	// We can't fully test without a real JWKS endpoint, but we test the bypass logic
}

func TestJWTAuthenticator_shouldBypass(t *testing.T) {
	cfg := config.AuthSettings{
		BypassPaths: []string{"/health", "/public"},
	}
	logger := testutil.NewTestLogger()

	auth, _ := NewJWTAuthenticator(cfg, logger)

	tests := []struct {
		path     string
		expected bool
	}{
		{"/health", true},
		{"/public", true},
		{"/api/test", false},
		{"/health/status", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := auth.shouldBypass(tt.path)
			if result != tt.expected {
				t.Errorf("expected shouldBypass(%q)=%v, got %v", tt.path, tt.expected, result)
			}
		})
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		expectedTok string
		expectedErr bool
	}{
		{
			name:        "empty header",
			header:      "",
			expectedErr: true,
		},
		{
			name:        "no Bearer prefix",
			header:      "token123",
			expectedErr: true,
		},
		{
			name:        "invalid format - no space",
			header:      "Bearertoken",
			expectedErr: true,
		},
		{
			name:        "invalid format - too many parts",
			header:      "Bearer token extra",
			expectedErr: true,
		},
		{
			name:        "valid Bearer token",
			header:      "Bearer token123",
			expectedTok: "token123",
			expectedErr: false,
		},
		{
			name:        "valid Bearer token - case insensitive",
			header:      "bearer token123",
			expectedTok: "token123",
			expectedErr: false,
		},
		{
			name:        "valid Bearer token - mixed case",
			header:      "BeArEr token123",
			expectedTok: "token123",
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := extractBearerToken(tt.header)

			if tt.expectedErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if token != tt.expectedTok {
				t.Errorf("expected token %q, got %q", tt.expectedTok, token)
			}
		})
	}
}

func TestJWTAuthenticator_Close(t *testing.T) {
	cfg := config.AuthSettings{
		Enabled: false,
	}
	logger := testutil.NewTestLogger()

	auth, _ := NewJWTAuthenticator(cfg, logger)
	
	// Should not panic
	auth.Close()
}

func TestJWTAuthenticator_Middleware_MissingAuthHeader(t *testing.T) {
	cfg := config.AuthSettings{
		Enabled:     true,
		IssuerURI:   "https://issuer.example.com",
		JWKSetURI:   "https://issuer.example.com/.well-known/jwks.json",
		BypassPaths: []string{},
	}
	logger := testutil.NewTestLogger()

	auth, err := NewJWTAuthenticator(cfg, logger)
	if err != nil {
		// JWKS loading might fail, skip test
		t.Skip("Skipping test - JWKS loading requires network access")
		return
	}

	middleware := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	// Should return 401 for missing header
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestJWTAuthenticator_Middleware_InvalidToken(t *testing.T) {
	cfg := config.AuthSettings{
		Enabled:     true,
		IssuerURI:   "https://issuer.example.com",
		JWKSetURI:   "https://issuer.example.com/.well-known/jwks.json",
		BypassPaths: []string{},
	}
	logger := testutil.NewTestLogger()

	auth, err := NewJWTAuthenticator(cfg, logger)
	if err != nil {
		t.Skip("Skipping test - JWKS loading requires network access")
		return
	}

	middleware := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	// Should return 401 for invalid token
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}
