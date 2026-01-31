package middleware

import (
	"log/slog"
	"net/http"
	"time"

	ctxutil "3tcapital/ms_facturacion_core/internal/infrastructure/context"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// responseWriter wraps http.ResponseWriter to capture status code and bytes written.
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)
	return n, err
}

// RequestLogger returns a middleware that logs HTTP requests and responses.
// It logs request details (method, path, IP, user agent, request ID) and
// response details (status code, duration, bytes written).
// It also adds the correlation ID to the request context for downstream use.
// Log levels are determined by status code:
//   - Info: 2xx, 3xx
//   - Warn: 4xx
//   - Error: 5xx
func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Get request ID from Chi middleware and add to context as correlation ID
			requestID := chimw.GetReqID(r.Context())
			ctx := ctxutil.WithCorrelationID(r.Context(), requestID)

			// Wrap the response writer to capture status code and bytes written
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // Default status code
			}

			// Execute the next handler with updated context
			next.ServeHTTP(rw, r.WithContext(ctx))

			// Calculate duration
			duration := time.Since(start)
			durationMs := float64(duration.Nanoseconds()) / 1e6

			// Build log attributes efficiently
			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"status", rw.statusCode,
				"duration_ms", durationMs,
				"bytes", rw.bytesWritten,
			}

			// Add correlation ID (request ID) if available
			if requestID != "" {
				attrs = append(attrs, "correlation_id", requestID)
				attrs = append(attrs, "request_id", requestID)
			}

			// Add user agent if available
			if userAgent := r.Header.Get("User-Agent"); userAgent != "" {
				attrs = append(attrs, "user_agent", userAgent)
			}

			// Log based on status code
			switch {
			case rw.statusCode >= 500:
				log.Error("HTTP request", attrs...)
			case rw.statusCode >= 400:
				log.Warn("HTTP request", attrs...)
			default:
				log.Info("HTTP request", attrs...)
			}
		})
	}
}
