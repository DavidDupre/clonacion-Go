package numrot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"3tcapital/ms_facturacion_core/internal/infrastructure/cache"
)

// HTTPClient interface allows using both standard and traced HTTP clients.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// AuthManager handles Numrot authentication with token caching.
type AuthManager struct {
	baseURL  string
	username string
	password string
	tokenTTL time.Duration
	cache    *cache.TokenCache
	client   HTTPClient
	log      *slog.Logger
	mu       sync.Mutex // Protects token refresh to avoid concurrent requests
}

// tokenRequest represents the authentication request payload.
type tokenRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}


// NewAuthManager creates a new Numrot authentication manager.
func NewAuthManager(baseURL, username, password string, tokenTTL time.Duration, client HTTPClient, log *slog.Logger) *AuthManager {
	return &AuthManager{
		baseURL:  baseURL,
		username: username,
		password: password,
		tokenTTL: tokenTTL,
		cache:    cache.NewTokenCache(),
		client:   client,
		log:      log,
	}
}

// GetToken returns a valid authentication token, refreshing if necessary.
func (a *AuthManager) GetToken(ctx context.Context) (string, error) {
	a.log.Debug("Getting Numrot token", "username", a.username)
	// Try to get from cache first
	if token, ok := a.cache.Get(); ok {
		return token, nil
	}

	// Token expired or not cached, need to refresh
	// Use mutex to prevent multiple concurrent refresh requests
	a.mu.Lock()
	defer a.mu.Unlock()

	// Double-check after acquiring lock (another goroutine might have refreshed)
	if token, ok := a.cache.Get(); ok {
		return token, nil
	}

	// Perform authentication
	token, err := a.authenticate(ctx)
	if err != nil {
		a.log.Error("Numrot authentication failed", "error", err)
		return "", fmt.Errorf("numrot authentication failed: %w", err)
	}

	// Cache the token
	a.cache.Set(token, a.tokenTTL)
	a.log.Debug("Numrot token refreshed and cached", "ttl", a.tokenTTL)

	return token, nil
}

// authenticate performs the actual authentication request to Numrot.
func (a *AuthManager) authenticate(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/v2/api/Token", a.baseURL)

	reqBody := tokenRequest{
		Username: a.username,
		Password: a.password,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read token as plain text from response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	token := strings.TrimSpace(string(body))
	if token == "" {
		return "", fmt.Errorf("empty token in response")
	}

	return token, nil
}

// ClearToken removes the cached token, forcing a refresh on next request.
func (a *AuthManager) ClearToken() {
	a.cache.Clear()
}
