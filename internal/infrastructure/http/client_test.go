package http

import (
	"net/http"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name     string
		config   *ClientConfig
		validate func(t *testing.T, client *http.Client)
	}{
		{
			name:   "nil config uses defaults",
			config: nil,
			validate: func(t *testing.T, client *http.Client) {
				if client.Timeout != 30*time.Second {
					t.Errorf("expected default timeout 30s, got %v", client.Timeout)
				}
			},
		},
		{
			name: "custom timeout",
			config: &ClientConfig{
				Timeout: 10 * time.Second,
			},
			validate: func(t *testing.T, client *http.Client) {
				if client.Timeout != 10*time.Second {
					t.Errorf("expected timeout 10s, got %v", client.Timeout)
				}
			},
		},
		{
			name: "custom transport",
			config: &ClientConfig{
				Timeout:   5 * time.Second,
				Transport: http.DefaultTransport,
			},
			validate: func(t *testing.T, client *http.Client) {
				if client.Transport != http.DefaultTransport {
					t.Error("expected custom transport to be set")
				}
			},
		},
		{
			name: "custom check redirect",
			config: &ClientConfig{
				Timeout: 5 * time.Second,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			},
			validate: func(t *testing.T, client *http.Client) {
				if client.CheckRedirect == nil {
					t.Error("expected custom check redirect to be set")
				}
			},
		},
		{
			name: "all custom options",
			config: &ClientConfig{
				Timeout:   15 * time.Second,
				Transport: http.DefaultTransport,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return nil
				},
			},
			validate: func(t *testing.T, client *http.Client) {
				if client.Timeout != 15*time.Second {
					t.Errorf("expected timeout 15s, got %v", client.Timeout)
				}
				if client.Transport != http.DefaultTransport {
					t.Error("expected custom transport to be set")
				}
				if client.CheckRedirect == nil {
					t.Error("expected custom check redirect to be set")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.config)
			if client == nil {
				t.Fatal("expected client to be created, got nil")
			}
			tt.validate(t, client)
		})
	}
}
