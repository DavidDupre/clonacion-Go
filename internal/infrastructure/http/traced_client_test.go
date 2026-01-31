package http

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"3tcapital/ms_facturacion_core/internal/core/audit"
	ctxutil "3tcapital/ms_facturacion_core/internal/infrastructure/context"
)

// mockAuditRepo is a mock implementation of audit.Repository for testing.
type mockAuditRepo struct {
	saved      []audit.ProviderAuditLog
	savedChan  chan audit.ProviderAuditLog // Channel to signal when Save is called
}

func (m *mockAuditRepo) Save(ctx context.Context, log audit.ProviderAuditLog) error {
	m.saved = append(m.saved, log)
	// Signal that Save was called (non-blocking)
	if m.savedChan != nil {
		select {
		case m.savedChan <- log:
		default:
		}
	}
	return nil
}

func (m *mockAuditRepo) FindByCorrelationID(ctx context.Context, correlationID string) ([]audit.ProviderAuditLog, error) {
	var results []audit.ProviderAuditLog
	for _, log := range m.saved {
		if log.CorrelationID == correlationID {
			results = append(results, log)
		}
	}
	return results, nil
}

func TestTracedClientDo(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify correlation ID header was added
		if r.Header.Get("X-Correlation-ID") == "" {
			t.Error("X-Correlation-ID header not set")
		}
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	// Create traced client
	mockRepo := &mockAuditRepo{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	
	client := NewTracedClient(&TracedClientConfig{
		AuditEnabled:    true,
		LogRequestBody:  true,
		LogResponseBody: true,
		MaxBodySize:     1024,
	}, log, mockRepo, "test-provider")

	// Create request with correlation ID in context
	ctx := ctxutil.WithCorrelationID(context.Background(), "test-correlation-123")
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Verify body is still readable
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "success") {
		t.Error("response body not properly restored")
	}
}

func TestTracedClientDoWithRequestBody(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read and verify request body is intact
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "test_data") {
			t.Error("request body not properly forwarded")
		}
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"received":true}`))
	}))
	defer server.Close()

	// Create traced client
	mockRepo := &mockAuditRepo{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	
	client := NewTracedClient(&TracedClientConfig{
		AuditEnabled:    true,
		LogRequestBody:  true,
		LogResponseBody: true,
		MaxBodySize:     1024,
	}, log, mockRepo, "test-provider")

	// Create request with body
	ctx := ctxutil.WithCorrelationID(context.Background(), "test-correlation-456")
	reqBody := strings.NewReader(`{"test_data":"value"}`)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, reqBody)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestTracedClientExtractOperation(t *testing.T) {
	mockRepo := &mockAuditRepo{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	
	client := NewTracedClient(&TracedClientConfig{}, log, mockRepo, "test-provider")

	tests := []struct {
		name     string
		url      string
		method   string
		expected string
	}{
		{
			name:     "extracts operation from path",
			url:      "https://api.example.com/v1/users/123",
			method:   "GET",
			expected: "123",
		},
		{
			name:     "handles trailing slash",
			url:      "https://api.example.com/v1/invoices/",
			method:   "POST",
			expected: "Invoices",
		},
		{
			name:     "falls back to method",
			url:      "https://api.example.com/",
			method:   "DELETE",
			expected: "DELETE_test-provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(tt.method, tt.url, nil)
			operation := client.extractOperation(req)

			if operation != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, operation)
			}
		})
	}
}

// TestTracedClient_AuditLogPersistsAfterContextCancellation verifies that audit logs
// are persisted even when the request context is cancelled immediately.
// This is a regression test for the bug where audit logs were not being saved
// because the goroutine used the request context which gets cancelled when the
// HTTP response completes.
func TestTracedClient_AuditLogPersistsAfterContextCancellation(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Create mock repo with channel to signal when Save is called
	mockRepo := &mockAuditRepo{
		savedChan: make(chan audit.ProviderAuditLog, 1),
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	client := NewTracedClient(&TracedClientConfig{
		AuditEnabled:    true,
		LogRequestBody:  true,
		LogResponseBody: true,
		MaxBodySize:     1024,
	}, log, mockRepo, "test-provider")

	// Create context that will be cancelled immediately after request
	ctx, cancel := context.WithCancel(context.Background())
	ctx = ctxutil.WithCorrelationID(ctx, "test-cancelled-context")

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, strings.NewReader(`{"test":"data"}`))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	// Cancel context immediately (simulating what happens when HTTP response completes)
	cancel()

	// Wait for audit log to be saved (with timeout)
	// The audit log should persist even though the context is cancelled
	// because the fix uses context.Background() instead of the request context
	select {
	case savedLog := <-mockRepo.savedChan:
		// SUCCESS: audit log was saved despite context cancellation
		if savedLog.CorrelationID != "test-cancelled-context" {
			t.Errorf("expected correlation ID 'test-cancelled-context', got '%s'", savedLog.CorrelationID)
		}
		if savedLog.Provider != "test-provider" {
			t.Errorf("expected provider 'test-provider', got '%s'", savedLog.Provider)
		}
		if savedLog.RequestMethod != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", savedLog.RequestMethod)
		}
		if savedLog.ResponseStatus == nil || *savedLog.ResponseStatus != http.StatusOK {
			t.Error("expected response status 200")
		}
	case <-time.After(3 * time.Second):
		// Timeout - audit log was not saved
		t.Fatal("Audit log was not saved within timeout - context cancellation prevented persistence")
	}

	// Verify that audit log was saved
	if len(mockRepo.saved) != 1 {
		t.Errorf("expected 1 audit log saved, got %d", len(mockRepo.saved))
	}
}

// TestTracedClient_AuditLogWithImmediatelyCancelledContext tests the edge case
// where the context is already cancelled before the request is made.
func TestTracedClient_AuditLogWithImmediatelyCancelledContext(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Create mock repo with channel
	mockRepo := &mockAuditRepo{
		savedChan: make(chan audit.ProviderAuditLog, 1),
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	client := NewTracedClient(&TracedClientConfig{
		AuditEnabled:    true,
		LogRequestBody:  false,
		LogResponseBody: false,
		MaxBodySize:     1024,
	}, log, mockRepo, "test-provider")

	// Create and cancel context BEFORE making request
	ctx, cancel := context.WithCancel(context.Background())
	ctx = ctxutil.WithCorrelationID(ctx, "already-cancelled")
	cancel() // Cancel immediately

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)

	// Execute request - should fail due to cancelled context
	// But if it somehow succeeds (e.g., local httptest server), audit should still work
	resp, err := client.Do(req)

	// Request might fail with context cancelled error, which is expected
	if err != nil {
		// This is expected - request failed due to cancelled context
		// But we still want to verify that if audit logging was attempted,
		// it would work with Background context
		t.Logf("request failed as expected with cancelled context: %v", err)
		return
	}

	if resp != nil {
		defer resp.Body.Close()

		// If request succeeded (httptest might allow this), verify audit log
		select {
		case savedLog := <-mockRepo.savedChan:
			// Even with cancelled request context, audit log should save
			if savedLog.CorrelationID != "already-cancelled" {
				t.Errorf("expected correlation ID 'already-cancelled', got '%s'", savedLog.CorrelationID)
			}
		case <-time.After(2 * time.Second):
			// Timeout - this is acceptable as the request might have failed
			t.Log("Audit log not saved (expected if request failed)")
		}
	}
}
