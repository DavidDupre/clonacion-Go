package cache

import (
	"sync"
	"time"
)

// TokenCache provides thread-safe caching for authentication tokens with TTL support.
type TokenCache struct {
	mu        sync.RWMutex
	token     string
	expiresAt time.Time
}

// NewTokenCache creates a new thread-safe token cache.
func NewTokenCache() *TokenCache {
	return &TokenCache{}
}

// Get returns the cached token if it's still valid, otherwise returns empty string.
func (c *TokenCache) Get() (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.token == "" {
		return "", false
	}

	if time.Now().After(c.expiresAt) {
		return "", false
	}

	return c.token, true
}

// Set stores a token with the specified TTL.
func (c *TokenCache) Set(token string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.token = token
	c.expiresAt = time.Now().Add(ttl)
}

// Clear removes the cached token.
func (c *TokenCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.token = ""
	c.expiresAt = time.Time{}
}

// IsExpired checks if the current token is expired without acquiring a read lock.
// This is useful for checking before attempting to refresh.
func (c *TokenCache) IsExpired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return time.Now().After(c.expiresAt) || c.token == ""
}
