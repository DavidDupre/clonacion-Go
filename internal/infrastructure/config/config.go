package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// AppConfig encapsulates all runtime configuration knobs.
type AppConfig struct {
	App                AppSettings
	HTTP               HTTPSettings
	Auth               AuthSettings
	Log                LogSettings
	Database           DatabaseSettings
	Audit              AuditSettings
	InvoiceProviders   InvoiceProvidersSettings
	DocumentProcessing DocumentProcessingSettings
}

type AppSettings struct {
	Name        string
	Version     string
	Environment string
}

type HTTPSettings struct {
	Port                int
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	WriteTimeoutMassive time.Duration // Extended timeout for massive operations (>100 documents)
	IdleTimeout         time.Duration
	ShutdownTimeout     time.Duration
}

type AuthSettings struct {
	Enabled     bool
	IssuerURI   string
	JWKSetURI   string
	ClockSkew   time.Duration
	BypassPaths []string
}

type LogSettings struct {
	Level string
}

type DatabaseSettings struct {
	Host            string
	Port            int
	Database        string
	User            string
	Password        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type AuditSettings struct {
	Enabled         bool
	LogRequestBody  bool
	LogResponseBody bool
	MaxBodySize     int
}

type InvoiceProvidersSettings struct {
	Numrot NumrotSettings
}

// DocumentProcessingSettings contains configuration for concurrent document processing
type DocumentProcessingSettings struct {
	WorkerPoolSize        int    // Number of workers for document processing
	BatchSize             int    // Size of batch for Numrot API
	MaxConcurrentRequests int    // Maximum concurrent requests to Numrot (max 1000 per token)
	RateLimitRPS          int    // Rate limit in requests per second
	ConcurrentBatchLimit  int    // Maximum batches processing simultaneously (calculated)
	CdoAmbienteDefault    string // Default environment value for documents ("1"=production, "2"=test)
}

type NumrotSettings struct {
	BaseURL     string
	DSBaseURL   string // Base URL specifically for DS (Documento Soporte) documents. If empty, uses BaseURL
	Username    string
	Password    string
	TokenTTL    time.Duration
	APITimeout  time.Duration // Timeout for HTTP requests to Numrot API
	Key         string
	Secret      string
	RadianURL   string
	EmisorNit   string
	RazonSocial string
	// Event generator information (for Radian event registration)
	GeneratorNombre         string // First name of event generator
	GeneratorApellido       string // Last name of event generator
	GeneratorIdentificacion string // ID number of event generator
	// Resolution query settings (for testing environments)
	ResolutionsEnabled   bool   // Enable/disable resolution queries (default: true)
	HardcodedInvoiceAuth string // Hardcoded InvoiceAuthorization value (used when ResolutionsEnabled=false)
	HardcodedStartDate   string // Hardcoded StartDate value (used when ResolutionsEnabled=false)
	HardcodedEndDate     string // Hardcoded EndDate value (used when ResolutionsEnabled=false)
	HardcodedPrefix      string // Hardcoded Prefix value (used when ResolutionsEnabled=false)
	HardcodedFrom        string // Hardcoded From value (used when ResolutionsEnabled=false)
	HardcodedTo          string // Hardcoded To value (used when ResolutionsEnabled=false)
	// InvoicePeriod settings for NC without reference (CustomizationID "22")
	NCInvoicePeriodStartDate string // Start date for InvoicePeriod in NC documents
	NCInvoicePeriodStartTime string // Start time for InvoicePeriod in NC documents
	NCInvoicePeriodEndDate   string // End date for InvoicePeriod in NC documents
	NCInvoicePeriodEndTime   string // End time for InvoicePeriod in NC documents
}

// Load resolves the application configuration from environment variables.
// It first attempts to load variables from a .env file if it exists.
// Environment variables set in the system take precedence over .env file values.
func Load() (AppConfig, error) {
	// Try to load .env file (ignore error if file doesn't exist)
	// This allows the application to work both with .env files (local dev)
	// and environment variables (Docker, production)
	_ = godotenv.Load()

	cfg := AppConfig{
		App: AppSettings{
			Name:        getEnv("APP_NAME", "ms_facturacion_core"),
			Version:     getEnv("APP_VERSION", "0.1.0"),
			Environment: getEnv("APP_ENV", "local"),
		},
		HTTP: HTTPSettings{
			Port:                getEnvAsInt("APP_PORT", 8080),
			ReadTimeout:         getEnvAsDuration("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:        getEnvAsDuration("HTTP_WRITE_TIMEOUT", 10*time.Second),
			WriteTimeoutMassive: getEnvAsDuration("HTTP_WRITE_TIMEOUT_MASSIVE", 15*time.Minute),
			IdleTimeout:         getEnvAsDuration("HTTP_IDLE_TIMEOUT", 120*time.Second),
			ShutdownTimeout:     getEnvAsDuration("HTTP_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Auth: AuthSettings{
			Enabled:     getEnvAsBool("AUTH_ENABLED", true),
			IssuerURI:   strings.TrimSpace(os.Getenv("JWT_ISSUER_URI")),
			JWKSetURI:   strings.TrimSpace(os.Getenv("JWT_JWK_SET_URI")),
			ClockSkew:   getEnvAsDuration("AUTH_CLOCK_SKEW", 2*time.Minute),
			BypassPaths: getEnvAsCSV("AUTH_BYPASS_PATHS", []string{"/health"}),
		},
		Log: LogSettings{
			Level: getEnv("LOG_LEVEL", "info"),
		},
		Database: DatabaseSettings{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvAsInt("DB_PORT", 5433),
			Database:        getEnv("DB_NAME", "ms_facturacion_core"),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", ""),
			SSLMode:         getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvAsDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
		},
		Audit: AuditSettings{
			Enabled:         getEnvAsBool("AUDIT_ENABLED", true),
			LogRequestBody:  getEnvAsBool("AUDIT_LOG_REQUEST_BODY", true),
			LogResponseBody: getEnvAsBool("AUDIT_LOG_RESPONSE_BODY", true),
			MaxBodySize:     getEnvAsInt("AUDIT_MAX_BODY_SIZE", 102400),
		},
		InvoiceProviders: InvoiceProvidersSettings{
			Numrot: NumrotSettings{
				BaseURL:                  strings.TrimSpace(os.Getenv("NUMROT_BASE_URL")),
				DSBaseURL:                strings.TrimSpace(os.Getenv("NUMROT_DS_BASE_URL")), // URL específica para DS, si está vacía usa BaseURL
				Username:                 strings.TrimSpace(os.Getenv("NUMROT_USERNAME")),
				Password:                 strings.TrimSpace(os.Getenv("NUMROT_PASSWORD")),
				TokenTTL:                 getEnvAsDuration("NUMROT_TOKEN_TTL", 1*time.Hour),
				APITimeout:               getEnvAsDuration("NUMROT_API_TIMEOUT", 300*time.Second), // Increased from 30s to 300s (5min) for massive operations
				Key:                      strings.TrimSpace(os.Getenv("NUMROT_KEY")),
				Secret:                   strings.TrimSpace(os.Getenv("NUMROT_SECRET")),
				RadianURL:                strings.TrimSpace(os.Getenv("NUMROT_RADIAN_URL")),
				EmisorNit:                strings.TrimSpace(os.Getenv("NUMROT_EMISOR_NIT")),
				RazonSocial:              strings.TrimSpace(os.Getenv("NUMROT_RAZON_SOCIAL")),
				GeneratorNombre:          strings.TrimSpace(os.Getenv("NUMROT_GENERATOR_NOMBRE")),
				GeneratorApellido:        strings.TrimSpace(os.Getenv("NUMROT_GENERATOR_APELLIDO")),
				GeneratorIdentificacion:  strings.TrimSpace(os.Getenv("NUMROT_GENERATOR_IDENTIFICACION")),
				ResolutionsEnabled:       getEnvAsBool("NUMROT_RESOLUTIONS_ENABLED", true), // Default: enabled
				HardcodedInvoiceAuth:     strings.TrimSpace(os.Getenv("NUMROT_HARDCODED_INVOICE_AUTH")),
				HardcodedStartDate:       strings.TrimSpace(os.Getenv("NUMROT_HARDCODED_START_DATE")),
				HardcodedEndDate:         strings.TrimSpace(os.Getenv("NUMROT_HARDCODED_END_DATE")),
				HardcodedPrefix:          strings.TrimSpace(os.Getenv("NUMROT_HARDCODED_PREFIX")),
				HardcodedFrom:            strings.TrimSpace(os.Getenv("NUMROT_HARDCODED_FROM")),
				HardcodedTo:              strings.TrimSpace(os.Getenv("NUMROT_HARDCODED_TO")),
				NCInvoicePeriodStartDate: strings.TrimSpace(os.Getenv("NUMROT_NC_INVOICE_PERIOD_START_DATE")),
				NCInvoicePeriodStartTime: strings.TrimSpace(os.Getenv("NUMROT_NC_INVOICE_PERIOD_START_TIME")),
				NCInvoicePeriodEndDate:   strings.TrimSpace(os.Getenv("NUMROT_NC_INVOICE_PERIOD_END_DATE")),
				NCInvoicePeriodEndTime:   strings.TrimSpace(os.Getenv("NUMROT_NC_INVOICE_PERIOD_END_TIME")),
			},
		},
		DocumentProcessing: DocumentProcessingSettings{
			WorkerPoolSize:        getEnvAsInt("DOCUMENT_WORKER_POOL_SIZE", 10),
			BatchSize:             getEnvAsInt("DOCUMENT_BATCH_SIZE", 50),
			MaxConcurrentRequests: getEnvAsInt("DOCUMENT_MAX_CONCURRENT_REQUESTS", 50), // Reduced from 1000 to 50 for better stability
			RateLimitRPS:          getEnvAsInt("DOCUMENT_RATE_LIMIT_RPS", 50),          // Reduced from 100 to 50 to match concurrency
			CdoAmbienteDefault:    strings.TrimSpace(os.Getenv("CDO_AMBIENTE_DEFAULT")),
		},
	}

	// Validate CDO_AMBIENTE_DEFAULT
	if cfg.DocumentProcessing.CdoAmbienteDefault == "" {
		cfg.DocumentProcessing.CdoAmbienteDefault = "2" // Default to test environment
	}
	if cfg.DocumentProcessing.CdoAmbienteDefault != "1" && cfg.DocumentProcessing.CdoAmbienteDefault != "2" {
		return cfg, errors.New("invalid config: CDO_AMBIENTE_DEFAULT must be '1' (production) or '2' (test)")
	}

	// Validate and calculate concurrent batch limit
	if cfg.DocumentProcessing.MaxConcurrentRequests > 200 {
		return cfg, errors.New("invalid config: DOCUMENT_MAX_CONCURRENT_REQUESTS cannot exceed 200 (recommended max for stability)")
	}
	if cfg.DocumentProcessing.MaxConcurrentRequests <= 0 {
		return cfg, errors.New("invalid config: DOCUMENT_MAX_CONCURRENT_REQUESTS must be greater than 0")
	}
	if cfg.DocumentProcessing.BatchSize <= 0 {
		return cfg, errors.New("invalid config: DOCUMENT_BATCH_SIZE must be greater than 0")
	}

	// Calculate concurrent batch limit based on max concurrent requests and batch size
	cfg.DocumentProcessing.ConcurrentBatchLimit = cfg.DocumentProcessing.MaxConcurrentRequests / cfg.DocumentProcessing.BatchSize
	if cfg.DocumentProcessing.ConcurrentBatchLimit < 1 {
		cfg.DocumentProcessing.ConcurrentBatchLimit = 1
	}

	if cfg.Auth.Enabled {
		if cfg.Auth.IssuerURI == "" {
			return cfg, errors.New("invalid config: JWT_ISSUER_URI is required when AUTH_ENABLED=true")
		}
		if cfg.Auth.JWKSetURI == "" {
			return cfg, errors.New("invalid config: JWT_JWK_SET_URI is required when AUTH_ENABLED=true")
		}
	}

	return cfg, nil
}

// Address returns the HTTP listen address in host:port form.
func (h HTTPSettings) Address() string {
	return fmt.Sprintf(":%d", h.Port)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvAsCSV(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	if len(values) == 0 {
		return fallback
	}
	return values
}
