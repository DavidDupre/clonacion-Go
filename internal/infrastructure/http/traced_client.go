package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"3tcapital/ms_facturacion_core/internal/core/audit"
	ctxutil "3tcapital/ms_facturacion_core/internal/infrastructure/context"
	"3tcapital/ms_facturacion_core/internal/infrastructure/security"
)

// TracedClient wraps an HTTP client to provide comprehensive request/response tracing.
// It logs all requests and responses, sanitizes sensitive data, and persists audit trails.
type TracedClient struct {
	client       *http.Client
	log          *slog.Logger
	auditRepo    audit.Repository
	provider     string
	auditEnabled bool
	logReqBody   bool
	logRespBody  bool
	maxBodySize  int
}

// TracedClientConfig holds configuration for the traced HTTP client.
type TracedClientConfig struct {
	Timeout         time.Duration
	AuditEnabled    bool
	LogRequestBody  bool
	LogResponseBody bool
	MaxBodySize     int
	MaxConnsPerHost int // Maximum connections per host (0 = use default 50)
}

// NewTracedClient creates a new traced HTTP client with proper connection pooling.
func NewTracedClient(cfg *TracedClientConfig, log *slog.Logger, auditRepo audit.Repository, provider string) *TracedClient {
	if cfg.MaxBodySize == 0 {
		cfg.MaxBodySize = 102400 // 100KB default
	}

	// Configure transport with connection pooling and proper timeouts
	maxConnsPerHost := cfg.MaxConnsPerHost
	if maxConnsPerHost == 0 {
		maxConnsPerHost = 50 // Default to 50 connections per host
	}
	maxIdleConnsPerHost := maxConnsPerHost // Keep same number of idle connections

	// Set ResponseHeaderTimeout to be at least as long as the client timeout
	// This prevents premature connection closure when waiting for response headers
	responseHeaderTimeout := cfg.Timeout
	if responseHeaderTimeout < 60*time.Second {
		// Minimum 60 seconds for ResponseHeaderTimeout to handle slow APIs
		responseHeaderTimeout = 60 * time.Second
	}

	transport := &http.Transport{
		MaxIdleConns:          100,                 // Maximum idle connections across all hosts
		MaxIdleConnsPerHost:   maxIdleConnsPerHost, // Maximum idle connections per host
		MaxConnsPerHost:       maxConnsPerHost,     // Maximum connections per host (prevents overwhelming server)
		IdleConnTimeout:       90 * time.Second,    // Close idle connections after 90s
		DisableKeepAlives:     false,               // Enable keep-alive for connection reuse
		DisableCompression:    false,               // Enable compression
		DialContext:           nil,                 // Use default dialer
		TLSHandshakeTimeout:   10 * time.Second,    // TLS handshake timeout
		ResponseHeaderTimeout: responseHeaderTimeout, // Time to wait for response headers (matches or exceeds client timeout)
		ExpectContinueTimeout: 1 * time.Second,     // Time to wait for 100-continue
	}

	return &TracedClient{
		client: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: transport,
		},
		log:          log,
		auditRepo:    auditRepo,
		provider:     provider,
		auditEnabled: cfg.AuditEnabled,
		logReqBody:   cfg.LogRequestBody,
		logRespBody:  cfg.LogResponseBody,
		maxBodySize:  cfg.MaxBodySize,
	}
}

// Do executes an HTTP request with full tracing and audit capabilities.
// It captures request/response details, sanitizes sensitive data, and persists audit logs.
func (c *TracedClient) Do(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	correlationID := ctxutil.GetCorrelationID(ctx)
	operation := c.extractOperation(req)
	start := time.Now()

	// Add correlation ID header for downstream tracing
	if correlationID != "" {
		req.Header.Set("X-Correlation-ID", correlationID)
	}

	// Capture request body for logging/audit
	var requestBody []byte
	if req.Body != nil {
		var err error
		requestBody, err = io.ReadAll(req.Body)
		if err != nil {
			c.log.Error("Failed to read request body for tracing",
				"error", err,
				"correlation_id", correlationID,
			)
		}
		// Restore body for actual request
		req.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	}

	// Log request
	c.logRequest(ctx, correlationID, operation, req, requestBody)

	// Execute request
	resp, err := c.client.Do(req)
	duration := time.Since(start)

	// Capture response body for logging/audit
	var responseBody []byte
	if resp != nil && resp.Body != nil {
		responseBody, _ = io.ReadAll(resp.Body)
		// Restore body for caller
		resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))
	}

	// Log response
	c.logResponse(ctx, correlationID, operation, req, resp, err, duration, responseBody)

	// Persist audit log asynchronously (don't block on audit failures)
	if c.auditEnabled && c.auditRepo != nil {
		// Ensure we have a correlation ID for audit tracking
		if correlationID == "" {
			// Generate a fallback correlation ID if missing
			correlationID = fmt.Sprintf("audit-%d", time.Now().UnixNano())
			c.log.Warn("Missing correlation ID, generated fallback",
				"fallback_id", correlationID,
				"operation", operation,
			)
		}

		// Log that audit persistence is starting
		c.log.Debug("Starting audit log persistence",
			"correlation_id", correlationID,
			"provider", c.provider,
			"operation", operation,
			"method", req.Method,
			"url", security.SanitizeURL(req.URL.String()),
		)

		// Persist audit log asynchronously using Background context
		// This ensures audit logs persist even after the HTTP request completes
		go func() {
			defer func() {
				if r := recover(); r != nil {
					c.log.Error("Panic in audit log persistence",
						"panic", r,
						"correlation_id", correlationID,
						"operation", operation,
						"provider", c.provider,
						"method", req.Method,
						"url", security.SanitizeURL(req.URL.String()),
					)
				}
			}()

			// Use Background context to ensure audit log persists independently of request lifecycle
			// The request context (ctx) gets cancelled when the HTTP response completes,
			// which would prevent audit log persistence. Using Background ensures the
			// goroutine can complete its work even after the request has finished.
			saveCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			c.persistAuditLog(saveCtx, correlationID, operation, req, resp, err, duration, requestBody, responseBody)
		}()
	} else {
		// Log when audit is skipped
		var reason string
		if !c.auditEnabled {
			reason = "audit disabled in configuration"
		} else if c.auditRepo == nil {
			reason = "audit repository not available"
		}
		c.log.Debug("Audit log skipped",
			"correlation_id", correlationID,
			"provider", c.provider,
			"operation", operation,
			"reason", reason,
		)
	}

	return resp, err
}

// logRequest logs the outgoing HTTP request.
func (c *TracedClient) logRequest(ctx context.Context, correlationID, operation string, req *http.Request, body []byte) {
	attrs := []any{
		"correlation_id", correlationID,
		"provider", c.provider,
		"operation", operation,
		"method", req.Method,
		"url", security.SanitizeURL(req.URL.String()),
	}

	// Log request body if enabled
	if c.logReqBody && len(body) > 0 {
		sanitizedBody := security.SanitizeBody(body, c.maxBodySize)
		attrs = append(attrs, "request_body", string(sanitizedBody))
	}

	c.log.Info("provider_request", attrs...)
}

// logResponse logs the HTTP response received.
func (c *TracedClient) logResponse(ctx context.Context, correlationID, operation string, req *http.Request, resp *http.Response, err error, duration time.Duration, body []byte) {
	durationMs := duration.Milliseconds()

	attrs := []any{
		"correlation_id", correlationID,
		"provider", c.provider,
		"operation", operation,
		"method", req.Method,
		"url", security.SanitizeURL(req.URL.String()),
		"duration_ms", durationMs,
	}

	if err != nil {
		attrs = append(attrs, "error", err.Error())
		c.log.Error("provider_request_failed", attrs...)
		return
	}

	attrs = append(attrs, "status", resp.StatusCode)
	attrs = append(attrs, "response_size_bytes", len(body))

	// Log response body if enabled
	if c.logRespBody && len(body) > 0 {
		sanitizedBody := security.SanitizeBody(body, c.maxBodySize)
		attrs = append(attrs, "response_body", string(sanitizedBody))
	}

	// Determine log level based on status code
	switch {
	case resp.StatusCode >= 500:
		c.log.Error("provider_response", attrs...)
	case resp.StatusCode >= 400:
		c.log.Warn("provider_response", attrs...)
	default:
		c.log.Info("provider_response", attrs...)
	}
}

// persistAuditLog saves the request/response audit trail to the database.
func (c *TracedClient) persistAuditLog(ctx context.Context, correlationID, operation string, req *http.Request, resp *http.Response, err error, duration time.Duration, requestBody, responseBody []byte) {
	c.log.Info("DEBUG: Attempting to persist audit log",
		"correlation_id", correlationID,
		"operation", operation,
		"url", req.URL.String(),
		"audit_enabled", c.auditEnabled,
		"audit_repo_available", c.auditRepo != nil,
	)

	// Create audit log entry
	auditLog := audit.ProviderAuditLog{
		CorrelationID:  correlationID,
		Provider:       c.provider,
		Operation:      operation,
		RequestMethod:  req.Method,
		RequestURL:     security.SanitizeURL(req.URL.String()),
		RequestHeaders: security.SanitizeHeaders(req.Header),
		DurationMs:     duration.Milliseconds(),
	}

	// Sanitize and add request body
	if len(requestBody) > 0 {
		auditLog.RequestBody = security.SanitizeBody(requestBody, c.maxBodySize)
	}

	// Handle response data
	if resp != nil {
		status := resp.StatusCode
		auditLog.ResponseStatus = &status
		auditLog.ResponseHeaders = security.SanitizeHeaders(resp.Header)

		// Sanitize and add response body
		if len(responseBody) > 0 {
			auditLog.ResponseBody = security.SanitizeBody(responseBody, c.maxBodySize)
		}
	}

	// Add error message if present
	if err != nil {
		auditLog.ErrorMessage = err.Error()
	}

	// Use the context passed from caller (already has timeout)
	if err := c.auditRepo.Save(ctx, auditLog); err != nil {
		// Log error with full context but don't fail the request
		c.log.Error("Failed to persist audit log",
			"error", err,
			"correlation_id", correlationID,
			"provider", c.provider,
			"operation", operation,
			"method", req.Method,
			"url", security.SanitizeURL(req.URL.String()),
			"response_status", auditLog.ResponseStatus,
			"duration_ms", auditLog.DurationMs,
		)
		return
	}

	// Log successful audit log persistence
	c.log.Debug("Audit log persisted successfully",
		"correlation_id", correlationID,
		"provider", c.provider,
		"operation", operation,
		"method", req.Method,
		"url", security.SanitizeURL(req.URL.String()),
		"response_status", auditLog.ResponseStatus,
		"duration_ms", auditLog.DurationMs,
	)
}

// extractOperation attempts to extract a meaningful operation name from the request.
func (c *TracedClient) extractOperation(req *http.Request) string {
	// Try to extract from URL path
	path := req.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// If we have meaningful path segments, use the last one
	if len(parts) > 0 && parts[len(parts)-1] != "" {
		operation := parts[len(parts)-1]
		// Capitalize first letter for consistency
		if len(operation) > 0 {
			operation = strings.ToUpper(operation[:1]) + operation[1:]
		}
		return operation
	}

	// Fallback to method + provider
	return fmt.Sprintf("%s_%s", req.Method, c.provider)
}

// Client returns the underlying HTTP client for compatibility.
func (c *TracedClient) Client() *http.Client {
	return c.client
}
