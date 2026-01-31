package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"3tcapital/ms_facturacion_core/internal/core/audit"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository implements the audit.Repository interface using PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

// NewRepository creates a new PostgreSQL audit repository.
func NewRepository(pool *pgxpool.Pool) audit.Repository {
	return &Repository{pool: pool, log: nil}
}

// NewRepositoryWithLogger creates a new PostgreSQL audit repository with logging.
func NewRepositoryWithLogger(pool *pgxpool.Pool, log *slog.Logger) audit.Repository {
	return &Repository{pool: pool, log: log}
}

// Save persists an audit log entry to the database.
func (r *Repository) Save(ctx context.Context, log audit.ProviderAuditLog) error {
	// Diagnostic logging when logger is available
	if r.log != nil {
		r.log.Debug("Attempting to save audit log",
			"correlation_id", log.CorrelationID,
			"provider", log.Provider,
			"operation", log.Operation,
			"method", log.RequestMethod,
			"url", log.RequestURL,
			"response_status", log.ResponseStatus,
			"duration_ms", log.DurationMs,
		)
	}

	query := `
		INSERT INTO provider_audit_log (
			correlation_id, provider, operation, request_method, request_url,
			request_headers, request_body, response_status, response_headers,
			response_body, duration_ms, error_message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	// Convert headers to JSON
	requestHeadersJSON, err := json.Marshal(log.RequestHeaders)
	if err != nil {
		errMsg := fmt.Errorf("marshal request headers: %w", err)
		if r.log != nil {
			r.log.Error("Failed to marshal request headers for audit log",
				"correlation_id", log.CorrelationID,
				"provider", log.Provider,
				"operation", log.Operation,
				"error", errMsg,
			)
		}
		return errMsg
	}

	responseHeadersJSON, err := json.Marshal(log.ResponseHeaders)
	if err != nil {
		errMsg := fmt.Errorf("marshal response headers: %w", err)
		if r.log != nil {
			r.log.Error("Failed to marshal response headers for audit log",
				"correlation_id", log.CorrelationID,
				"provider", log.Provider,
				"operation", log.Operation,
				"error", errMsg,
			)
		}
		return errMsg
	}

	// Handle nil request/response bodies
	var requestBodyJSON, responseBodyJSON interface{}
	if len(log.RequestBody) > 0 {
		requestBodyJSON = log.RequestBody
	}
	if len(log.ResponseBody) > 0 {
		responseBodyJSON = log.ResponseBody
	}

	_, err = r.pool.Exec(ctx, query,
		log.CorrelationID,
		log.Provider,
		log.Operation,
		log.RequestMethod,
		log.RequestURL,
		requestHeadersJSON,
		requestBodyJSON,
		log.ResponseStatus,
		responseHeadersJSON,
		responseBodyJSON,
		log.DurationMs,
		log.ErrorMessage,
	)
	if err != nil {
		errMsg := fmt.Errorf("insert audit log: %w", err)
		if r.log != nil {
			r.log.Error("Failed to insert audit log into database",
				"correlation_id", log.CorrelationID,
				"provider", log.Provider,
				"operation", log.Operation,
				"method", log.RequestMethod,
				"url", log.RequestURL,
				"response_status", log.ResponseStatus,
				"duration_ms", log.DurationMs,
				"error", errMsg,
			)
		}
		return errMsg
	}

	// Log successful save when logger is available
	if r.log != nil {
		r.log.Debug("Audit log saved successfully to database",
			"correlation_id", log.CorrelationID,
			"provider", log.Provider,
			"operation", log.Operation,
			"method", log.RequestMethod,
			"url", log.RequestURL,
			"response_status", log.ResponseStatus,
			"duration_ms", log.DurationMs,
		)
	}

	return nil
}

// FindByCorrelationID retrieves all audit logs with the given correlation ID.
func (r *Repository) FindByCorrelationID(ctx context.Context, correlationID string) ([]audit.ProviderAuditLog, error) {
	query := `
		SELECT id, correlation_id, provider, operation, request_method, request_url,
		       request_headers, request_body, response_status, response_headers,
		       response_body, duration_ms, error_message, created_at
		FROM provider_audit_log
		WHERE correlation_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, correlationID)
	if err != nil {
		return nil, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []audit.ProviderAuditLog
	for rows.Next() {
		var log audit.ProviderAuditLog
		var requestHeadersJSON, responseHeadersJSON []byte
		var requestBodyJSON, responseBodyJSON []byte

		err := rows.Scan(
			&log.ID,
			&log.CorrelationID,
			&log.Provider,
			&log.Operation,
			&log.RequestMethod,
			&log.RequestURL,
			&requestHeadersJSON,
			&requestBodyJSON,
			&log.ResponseStatus,
			&responseHeadersJSON,
			&responseBodyJSON,
			&log.DurationMs,
			&log.ErrorMessage,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}

		// Unmarshal headers
		if err := json.Unmarshal(requestHeadersJSON, &log.RequestHeaders); err != nil {
			return nil, fmt.Errorf("unmarshal request headers: %w", err)
		}
		if err := json.Unmarshal(responseHeadersJSON, &log.ResponseHeaders); err != nil {
			return nil, fmt.Errorf("unmarshal response headers: %w", err)
		}

		// Assign body bytes
		log.RequestBody = requestBodyJSON
		log.ResponseBody = responseBodyJSON

		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return logs, nil
}
