package middleware

import (
	"context"
	"net/http"

	"3tcapital/ms_facturacion_core/internal/infrastructure/config"
)

// ExtendedTimeout wraps a handler to apply an extended timeout for massive operations.
// This is useful for endpoints that process large amounts of data and need more time
// than the default WriteTimeout. The extended timeout is applied to the request context.
func ExtendedTimeout(cfg config.HTTPSettings) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create context with extended timeout for massive operations
			// Note: This extends the context timeout, but the server's WriteTimeout
			// still applies. Streaming with periodic flushes is the primary solution
			// to prevent WriteTimeout issues.
			ctx, cancel := context.WithTimeout(r.Context(), cfg.WriteTimeoutMassive)
			defer cancel()

			// Use the extended timeout context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
