package context

import (
	"context"
	"testing"
)

func TestWithCorrelationID(t *testing.T) {
	tests := []struct {
		name          string
		correlationID string
	}{
		{
			name:          "adds correlation ID to context",
			correlationID: "test-correlation-123",
		},
		{
			name:          "handles empty correlation ID",
			correlationID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = WithCorrelationID(ctx, tt.correlationID)

			result := GetCorrelationID(ctx)
			if result != tt.correlationID {
				t.Errorf("expected %s, got %s", tt.correlationID, result)
			}
		})
	}
}

func TestGetCorrelationID(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "returns correlation ID when present",
			ctx:      WithCorrelationID(context.Background(), "test-123"),
			expected: "test-123",
		},
		{
			name:     "returns empty string when not present",
			ctx:      context.Background(),
			expected: "",
		},
		{
			name:     "returns empty string for nil context value",
			ctx:      context.WithValue(context.Background(), CorrelationIDKey, nil),
			expected: "",
		},
		{
			name:     "returns empty string for wrong type",
			ctx:      context.WithValue(context.Background(), CorrelationIDKey, 123),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCorrelationID(tt.ctx)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestCorrelationIDPropagation(t *testing.T) {
	// Test that correlation ID propagates through context chain
	ctx := context.Background()
	ctx = WithCorrelationID(ctx, "original-id")
	
	// Create derived context
	ctx2, cancel := context.WithCancel(ctx)
	defer cancel()
	
	// Should still have correlation ID
	if GetCorrelationID(ctx2) != "original-id" {
		t.Error("correlation ID should propagate to derived contexts")
	}
}
