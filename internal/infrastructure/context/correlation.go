package context

import "context"

// contextKey is an unexported type for context keys to prevent collisions.
type contextKey string

// CorrelationIDKey is the context key for correlation IDs.
const CorrelationIDKey contextKey = "correlation_id"

// WithCorrelationID adds a correlation ID to the context.
// The correlation ID is used to track a request through the entire system,
// from the initial HTTP request through all external provider calls.
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, correlationID)
}

// GetCorrelationID retrieves the correlation ID from the context.
// Returns an empty string if no correlation ID is present.
func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return id
	}
	return ""
}
