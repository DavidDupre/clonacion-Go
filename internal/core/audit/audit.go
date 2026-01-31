package audit

import (
	"context"
	"encoding/json"
	"time"
)

// ProviderAuditLog represents an audit record for external provider API calls.
// It captures complete request/response details for debugging, compliance, and monitoring.
type ProviderAuditLog struct {
	ID              int64
	CorrelationID   string
	Provider        string
	Operation       string
	RequestMethod   string
	RequestURL      string
	RequestHeaders  map[string]string
	RequestBody     json.RawMessage
	ResponseStatus  *int
	ResponseHeaders map[string]string
	ResponseBody    json.RawMessage
	DurationMs      int64
	ErrorMessage    string
	CreatedAt       time.Time
}

// Repository defines the contract for persisting and retrieving audit logs.
type Repository interface {
	// Save persists an audit log entry to storage.
	Save(ctx context.Context, log ProviderAuditLog) error

	// FindByCorrelationID retrieves all audit logs associated with a correlation ID.
	// This is useful for debugging and tracking the complete flow of a request.
	FindByCorrelationID(ctx context.Context, correlationID string) ([]ProviderAuditLog, error)
}
