package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_DefaultValues(t *testing.T) {
	// Clear all relevant env vars
	envVars := []string{
		"APP_NAME", "APP_VERSION", "APP_ENV", "APP_PORT",
		"HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT", "HTTP_SHUTDOWN_TIMEOUT",
		"AUTH_ENABLED", "JWT_ISSUER_URI", "JWT_JWK_SET_URI", "AUTH_CLOCK_SKEW", "AUTH_BYPASS_PATHS",
		"LOG_LEVEL", "NUMROT_BASE_URL", "NUMROT_USERNAME", "NUMROT_PASSWORD", "NUMROT_TOKEN_TTL",
		"NUMROT_KEY", "NUMROT_SECRET", "NUMROT_RADIAN_URL",
		"NUMROT_KEY", "NUMROT_SECRET", "NUMROT_RADIAN_URL",
	}

	for _, key := range envVars {
		os.Unsetenv(key)
	}
	
	// Set AUTH_ENABLED=false to avoid requiring JWT config
	os.Setenv("AUTH_ENABLED", "false")
	defer os.Unsetenv("AUTH_ENABLED")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.App.Name != "ms_facturacion_core" {
		t.Errorf("expected default app name 'ms_facturacion_core', got %q", cfg.App.Name)
	}

	if cfg.App.Version != "0.1.0" {
		t.Errorf("expected default version '0.1.0', got %q", cfg.App.Version)
	}

	if cfg.App.Environment != "local" {
		t.Errorf("expected default environment 'local', got %q", cfg.App.Environment)
	}

	if cfg.HTTP.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.HTTP.Port)
	}

	// We set AUTH_ENABLED=false in the test, so it should be false
	if cfg.Auth.Enabled != false {
		t.Errorf("expected auth enabled false (as set in test), got %v", cfg.Auth.Enabled)
	}
}

func TestLoad_WithCustomValues(t *testing.T) {
	// Set custom values
	os.Setenv("APP_NAME", "test-app")
	os.Setenv("APP_VERSION", "2.0.0")
	os.Setenv("APP_ENV", "production")
	os.Setenv("APP_PORT", "9090")
	os.Setenv("AUTH_ENABLED", "false")
	defer func() {
		os.Unsetenv("APP_NAME")
		os.Unsetenv("APP_VERSION")
		os.Unsetenv("APP_ENV")
		os.Unsetenv("APP_PORT")
		os.Unsetenv("AUTH_ENABLED")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.App.Name != "test-app" {
		t.Errorf("expected app name 'test-app', got %q", cfg.App.Name)
	}

	if cfg.App.Version != "2.0.0" {
		t.Errorf("expected version '2.0.0', got %q", cfg.App.Version)
	}

	if cfg.App.Environment != "production" {
		t.Errorf("expected environment 'production', got %q", cfg.App.Environment)
	}

	if cfg.HTTP.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.HTTP.Port)
	}

	if cfg.Auth.Enabled != false {
		t.Errorf("expected auth enabled false, got %v", cfg.Auth.Enabled)
	}
}

func TestLoad_AuthEnabled_MissingIssuerURI(t *testing.T) {
	os.Setenv("AUTH_ENABLED", "true")
	os.Unsetenv("JWT_ISSUER_URI")
	os.Unsetenv("JWT_JWK_SET_URI")
	defer func() {
		os.Unsetenv("AUTH_ENABLED")
	}()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when AUTH_ENABLED=true and JWT_ISSUER_URI is missing")
	}

	if err.Error() != "invalid config: JWT_ISSUER_URI is required when AUTH_ENABLED=true" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoad_AuthEnabled_MissingJWKSetURI(t *testing.T) {
	os.Setenv("AUTH_ENABLED", "true")
	os.Setenv("JWT_ISSUER_URI", "https://issuer.example.com")
	os.Unsetenv("JWT_JWK_SET_URI")
	defer func() {
		os.Unsetenv("AUTH_ENABLED")
		os.Unsetenv("JWT_ISSUER_URI")
	}()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when AUTH_ENABLED=true and JWT_JWK_SET_URI is missing")
	}

	if err.Error() != "invalid config: JWT_JWK_SET_URI is required when AUTH_ENABLED=true" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestHTTPSettings_Address(t *testing.T) {
	settings := HTTPSettings{Port: 8080}
	addr := settings.Address()

	if addr != ":8080" {
		t.Errorf("expected address ':8080', got %q", addr)
	}
}

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_KEY", "test-value")
	defer os.Unsetenv("TEST_KEY")

	value := getEnv("TEST_KEY", "default")
	if value != "test-value" {
		t.Errorf("expected 'test-value', got %q", value)
	}

	value = getEnv("NON_EXISTENT_KEY", "default-value")
	if value != "default-value" {
		t.Errorf("expected 'default-value', got %q", value)
	}
}

func TestGetEnvAsBool(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		fallback bool
		expected bool
	}{
		{"true value", "true", false, true},
		{"false value", "false", true, false},
		{"True value", "True", false, true},
		{"FALSE value", "FALSE", true, false},
		{"invalid value", "invalid", true, true},
		{"missing key", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("TEST_BOOL", tt.envValue)
				defer os.Unsetenv("TEST_BOOL")
			} else {
				os.Unsetenv("TEST_BOOL")
			}

			result := getEnvAsBool("TEST_BOOL", tt.fallback)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetEnvAsInt(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		fallback int
		expected int
	}{
		{"valid int", "123", 0, 123},
		{"zero", "0", 999, 0},
		{"negative", "-10", 0, -10},
		{"invalid value", "not-a-number", 42, 42},
		{"missing key", "", 42, 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("TEST_INT", tt.envValue)
				defer os.Unsetenv("TEST_INT")
			} else {
				os.Unsetenv("TEST_INT")
			}

			result := getEnvAsInt("TEST_INT", tt.fallback)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestGetEnvAsDuration(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		fallback time.Duration
		expected time.Duration
	}{
		{"valid duration", "10s", 0, 10 * time.Second},
		{"minutes", "5m", 0, 5 * time.Minute},
		{"hours", "2h", 0, 2 * time.Hour},
		{"invalid value", "not-a-duration", 30 * time.Second, 30 * time.Second},
		{"empty value", "", 30 * time.Second, 30 * time.Second},
		{"missing key", "", 30 * time.Second, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("TEST_DURATION", tt.envValue)
				defer os.Unsetenv("TEST_DURATION")
			} else {
				os.Unsetenv("TEST_DURATION")
			}

			result := getEnvAsDuration("TEST_DURATION", tt.fallback)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetEnvAsCSV(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		fallback []string
		expected []string
	}{
		{
			name:     "single value",
			envValue: "value1",
			fallback: []string{"default"},
			expected: []string{"value1"},
		},
		{
			name:     "multiple values",
			envValue: "value1,value2,value3",
			fallback: []string{"default"},
			expected: []string{"value1", "value2", "value3"},
		},
		{
			name:     "with spaces",
			envValue: "value1, value2 , value3",
			fallback: []string{"default"},
			expected: []string{"value1", "value2", "value3"},
		},
		{
			name:     "empty values filtered",
			envValue: "value1,,value2, ,value3",
			fallback: []string{"default"},
			expected: []string{"value1", "value2", "value3"},
		},
		{
			name:     "empty string",
			envValue: "",
			fallback: []string{"default"},
			expected: []string{"default"},
		},
		{
			name:     "only spaces",
			envValue: " , , ",
			fallback: []string{"default"},
			expected: []string{"default"},
		},
		{
			name:     "missing key",
			envValue: "",
			fallback: []string{"default1", "default2"},
			expected: []string{"default1", "default2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("TEST_CSV", tt.envValue)
				defer os.Unsetenv("TEST_CSV")
			} else {
				os.Unsetenv("TEST_CSV")
			}

			result := getEnvAsCSV("TEST_CSV", tt.fallback)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d values, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("expected[%d] %q, got %q", i, expected, result[i])
				}
			}
		})
	}
}

func TestLoad_NumrotKeySecretRadianURL(t *testing.T) {
	os.Setenv("AUTH_ENABLED", "false")
	os.Setenv("NUMROT_KEY", "test-key-123")
	os.Setenv("NUMROT_SECRET", "test-secret-456")
	os.Setenv("NUMROT_RADIAN_URL", "https://radian.example.com")
	defer func() {
		os.Unsetenv("AUTH_ENABLED")
		os.Unsetenv("NUMROT_KEY")
		os.Unsetenv("NUMROT_SECRET")
		os.Unsetenv("NUMROT_RADIAN_URL")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.InvoiceProviders.Numrot.Key != "test-key-123" {
		t.Errorf("expected NUMROT_KEY 'test-key-123', got %q", cfg.InvoiceProviders.Numrot.Key)
	}

	if cfg.InvoiceProviders.Numrot.Secret != "test-secret-456" {
		t.Errorf("expected NUMROT_SECRET 'test-secret-456', got %q", cfg.InvoiceProviders.Numrot.Secret)
	}

	if cfg.InvoiceProviders.Numrot.RadianURL != "https://radian.example.com" {
		t.Errorf("expected NUMROT_RADIAN_URL 'https://radian.example.com', got %q", cfg.InvoiceProviders.Numrot.RadianURL)
	}
}

func TestLoad_NumrotRadianURL_DefaultsToBaseURL(t *testing.T) {
	os.Setenv("AUTH_ENABLED", "false")
	os.Setenv("NUMROT_BASE_URL", "https://base.example.com")
	os.Unsetenv("NUMROT_RADIAN_URL")
	defer func() {
		os.Unsetenv("AUTH_ENABLED")
		os.Unsetenv("NUMROT_BASE_URL")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// RadianURL should be empty when not set (will default in client)
	if cfg.InvoiceProviders.Numrot.RadianURL != "" {
		t.Errorf("expected empty RadianURL when not set, got %q", cfg.InvoiceProviders.Numrot.RadianURL)
	}
}
