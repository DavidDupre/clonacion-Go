package main

import (
	"3tcapital/goclonacion/internal/core/audit"
	"3tcapital/goclonacion/internal/infrastructure/config"
	"3tcapital/goclonacion/internal/infrastructure/http/server"
	"3tcapital/goclonacion/internal/infrastructure/logger"
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "service stopped: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logger.New(cfg.App.Name, cfg.Log.Level, cfg.App.Environment)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Initialize database connection
	var auditRepo audit.Repository
	var sqlDB *sql.DB
	if cfg.Database.Host != "" && cfg.Database.Database != "" {
		connString := fmt.Sprintf(
			"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.Database,
			cfg.Database.User,
			cfg.Database.Password,
			cfg.Database.SSLMode,
		)
		db, err := sql.Open("postgres", connString)
		if err != nil {
			log.Warn("Failed to open database, audit trail and acquirer service will be disabled",
				"error", err,
				"host", cfg.Database.Host,
				"database", cfg.Database.Database,
				"user", cfg.Database.User,
				"password_set", cfg.Database.Password != "")
			log.Info("Acquirer endpoints will be available but will return 503 until database connection is established")
		} else if err := db.PingContext(ctx); err != nil {
			_ = db.Close()
			log.Warn("Failed to connect to database, audit trail and acquirer service will be disabled",
				"error", err,
				"host", cfg.Database.Host,
				"database", cfg.Database.Database,
				"user", cfg.Database.User,
				"password_set", cfg.Database.Password != "")
			log.Info("Acquirer endpoints will be available but will return 503 until database connection is established")
		} else {
			sqlDB = db
			defer sqlDB.Close()
			log.Info("Database connection established", "database", cfg.Database.Database)
		}
	} else {
		log.Info("Database not configured, audit trail and acquirer service will be disabled",
			"audit_enabled_config", cfg.Audit.Enabled,
			"audit_repo_available", false,
		)
	}

	// Log overall audit configuration status
	if cfg.Audit.Enabled {
		if auditRepo != nil {
			log.Info("Audit trail configuration: ENABLED",
				"database_connected", sqlDB != nil,
				"audit_repo_available", auditRepo != nil,
				"max_body_size", cfg.Audit.MaxBodySize,
			)
		} else {
			log.Warn("Audit trail configuration: DISABLED - Database connection required",
				"audit_enabled_config", cfg.Audit.Enabled,
				"database_connected", false,
				"audit_repo_available", false,
			)
		}
	} else {
		log.Info("Audit trail configuration: DISABLED - Audit not enabled in configuration",
			"audit_enabled_config", cfg.Audit.Enabled,
		)
	}

	// Check if we can create a Numrot provider
	canCreateProvider := cfg.InvoiceProviders.Numrot.BaseURL != ""
	canCreateResolutions := canCreateProvider && cfg.InvoiceProviders.Numrot.Username != "" && cfg.InvoiceProviders.Numrot.Password != ""
	canCreateInvoices := canCreateProvider && cfg.InvoiceProviders.Numrot.Key != "" && cfg.InvoiceProviders.Numrot.Secret != ""

	if canCreateResolutions || canCreateInvoices {
		// Create traced HTTP client for external API calls with audit support
		// Use Numrot-specific API timeout if configured, otherwise use HTTP ReadTimeout
		apiTimeout := cfg.InvoiceProviders.Numrot.APITimeout
		if apiTimeout == 0 {
			// Fallback to HTTP ReadTimeout if APITimeout not configured
			apiTimeout = cfg.HTTP.ReadTimeout
		}
		// Configure MaxConnsPerHost based on concurrency settings
		maxConnsPerHost := cfg.DocumentProcessing.MaxConcurrentRequests
		if maxConnsPerHost > 100 {
			maxConnsPerHost = 100 // Cap at 100 to prevent overwhelming the server
		}
		if maxConnsPerHost == 0 {
			maxConnsPerHost = 50 // Default if not configured
		}

		// For document registration, disable console logging of request/response bodies
		auditEnabled := cfg.Audit.Enabled && auditRepo != nil

		// Log detailed audit configuration for Numrot provider
		if auditEnabled {
			log.Info("Audit trail enabled for Numrot provider",
				"max_body_size", cfg.Audit.MaxBodySize,
				"audit_repo", auditRepo != nil,
				"provider", "numrot",
			)
		} else {
			var reason string
			if !cfg.Audit.Enabled {
				reason = "audit disabled in configuration"
			} else if auditRepo == nil {
				reason = "audit repository not available (database not connected)"
			}
			log.Warn("Audit trail disabled for Numrot provider",
				"audit_enabled_config", cfg.Audit.Enabled,
				"audit_repo_available", auditRepo != nil,
				"reason", reason,
				"provider", "numrot",
			)
		}

		log.Info("Numrot provider configured", "baseURL", cfg.InvoiceProviders.Numrot.BaseURL, "dsBaseURL", cfg.InvoiceProviders.Numrot.DSBaseURL, "radianURL", cfg.InvoiceProviders.Numrot.RadianURL)

		// Log resolution configuration
		if !cfg.InvoiceProviders.Numrot.ResolutionsEnabled {
			log.Warn("Resolution queries DISABLED - using hardcoded values",
				"hardcoded_invoice_auth", cfg.InvoiceProviders.Numrot.HardcodedInvoiceAuth,
				"hardcoded_prefix", cfg.InvoiceProviders.Numrot.HardcodedPrefix,
				"hardcoded_start_date", cfg.InvoiceProviders.Numrot.HardcodedStartDate,
				"hardcoded_end_date", cfg.InvoiceProviders.Numrot.HardcodedEndDate,
				"hardcoded_from", cfg.InvoiceProviders.Numrot.HardcodedFrom,
				"hardcoded_to", cfg.InvoiceProviders.Numrot.HardcodedTo,
			)
		} else {
			log.Info("Resolution queries ENABLED - will query resolutions from API")
		}
	}

	// Create and start HTTP server
	if sqlDB == nil {
		return fmt.Errorf("database connection required")
	}

	srv, err := server.New(server.Options{
		Addr:   fmt.Sprintf(":%d", cfg.HTTP.Port),
		Logger: log,
		DB:     sqlDB,
	})
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	defer srv.Close()

	log.Info("Starting HTTP server", "port", cfg.HTTP.Port)
	return srv.Run(ctx)
}
