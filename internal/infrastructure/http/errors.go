package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorResponse represents a standardized error response format.
type ErrorResponse struct {
	Message string   `json:"message"`
	Errors  []string `json:"errors"`
}

// WriteError writes a standardized JSON error response to the HTTP response writer.
// It sets the appropriate Content-Type header, status code, and encodes the error response.
func WriteError(w http.ResponseWriter, statusCode int, message string, errors []string, log *slog.Logger) {
	response := ErrorResponse{
		Message: message,
		Errors:  errors,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		// If encoding fails, log the error but don't try to write again
		// as the status code has already been written
		if log != nil {
			log.Error("failed to encode error response", "error", err)
		}
	}
}
