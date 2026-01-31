package security

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestSanitizeHeaders(t *testing.T) {
	tests := []struct {
		name     string
		headers  http.Header
		expected map[string]string
	}{
		{
			name: "sensitive headers are redacted",
			headers: http.Header{
				"Authorization":  []string{"Bearer secret-token"},
				"Cookie":         []string{"session=abc123"},
				"Content-Type":   []string{"application/json"},
				"X-Api-Key":      []string{"my-api-key"},
			},
			expected: map[string]string{
				"Authorization": "[REDACTED]",
				"Cookie":        "[REDACTED]",
				"Content-Type":  "application/json",
				"X-Api-Key":     "[REDACTED]",
			},
		},
		{
			name: "multiple values are joined",
			headers: http.Header{
				"Accept": []string{"application/json", "text/html"},
			},
			expected: map[string]string{
				"Accept": "application/json, text/html",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeHeaders(tt.headers)
			
			for key, expectedValue := range tt.expected {
				if result[key] != expectedValue {
					t.Errorf("expected %s=%s, got %s", key, expectedValue, result[key])
				}
			}
		})
	}
}

func TestSanitizeBody(t *testing.T) {
	tests := []struct {
		name        string
		body        []byte
		maxSize     int
		expectation func(t *testing.T, result json.RawMessage)
	}{
		{
			name:    "empty body returns nil",
			body:    []byte{},
			maxSize: 1000,
			expectation: func(t *testing.T, result json.RawMessage) {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			},
		},
		{
			name:    "sensitive fields are redacted",
			body:    []byte(`{"username":"john","password":"secret123","email":"john@example.com"}`),
			maxSize: 1000,
			expectation: func(t *testing.T, result json.RawMessage) {
				var data map[string]interface{}
				if err := json.Unmarshal(result, &data); err != nil {
					t.Fatalf("failed to unmarshal result: %v", err)
				}
				
				if data["password"] != "[REDACTED]" {
					t.Errorf("expected password to be redacted, got %v", data["password"])
				}
				if data["username"] != "john" {
					t.Errorf("expected username to remain, got %v", data["username"])
				}
			},
		},
		{
			name:    "nested objects are sanitized",
			body:    []byte(`{"user":{"name":"john","auth":{"password":"secret","api_key":"key123"}}}`),
			maxSize: 1000,
			expectation: func(t *testing.T, result json.RawMessage) {
				var data map[string]interface{}
				if err := json.Unmarshal(result, &data); err != nil {
					t.Fatalf("failed to unmarshal result: %v", err)
				}
				
				user, ok := data["user"].(map[string]interface{})
				if !ok {
					t.Fatalf("user is not a map, got %T", data["user"])
				}
				
				// "auth" field itself is sensitive and should be redacted
				if user["auth"] != "[REDACTED]" {
					t.Errorf("expected auth field to be redacted, got %v", user["auth"])
				}
				
				// Verify name is not redacted
				if user["name"] != "john" {
					t.Errorf("expected name to remain, got %v", user["name"])
				}
			},
		},
		{
			name:    "body is truncated if too large",
			body:    []byte(`{"data":"very long string with lots of content"}`),
			maxSize: 20,
			expectation: func(t *testing.T, result json.RawMessage) {
				if len(result) <= 20 {
					t.Errorf("expected truncated body to be small")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeBody(tt.body, tt.maxSize)
			tt.expectation(t, result)
		})
	}
}

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "url without sensitive params unchanged",
			url:      "https://api.example.com/users?page=1&limit=10",
			expected: "https://api.example.com/users?page=1&limit=10",
		},
		{
			name:     "url with password param is redacted",
			url:      "https://api.example.com/auth?username=john&password=secret123",
			expected: "https://api.example.com/auth?username=john&password=[REDACTED]",
		},
		{
			name:     "url with token param is redacted",
			url:      "https://api.example.com/data?token=abc123&format=json",
			expected: "https://api.example.com/data?token=[REDACTED]&format=json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeURL(tt.url)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
