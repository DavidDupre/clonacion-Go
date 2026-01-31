package http

import (
	"net/http"
	"time"
)

// ClientConfig holds configuration for HTTP clients.
type ClientConfig struct {
	Timeout       time.Duration
	Transport     http.RoundTripper
	CheckRedirect func(req *http.Request, via []*http.Request) error
}

// NewClient creates a new HTTP client with standard configuration.
// If config is nil, uses sensible defaults (30s timeout).
func NewClient(config *ClientConfig) *http.Client {
	if config == nil {
		config = &ClientConfig{
			Timeout: 30 * time.Second,
		}
	}

	client := &http.Client{
		Timeout: config.Timeout,
	}

	if config.Transport != nil {
		client.Transport = config.Transport
	}

	if config.CheckRedirect != nil {
		client.CheckRedirect = config.CheckRedirect
	}

	return client
}
