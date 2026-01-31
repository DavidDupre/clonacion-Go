package cache

import (
	"sync"
	"testing"
	"time"
)

func TestNewTokenCache(t *testing.T) {
	cache := NewTokenCache()
	if cache == nil {
		t.Fatal("expected cache to be created, got nil")
	}
}

func TestTokenCache_Get(t *testing.T) {
	tests := []struct {
		name        string
		setupCache  func() *TokenCache
		expectedOk  bool
		expectedTok string
	}{
		{
			name: "empty cache",
			setupCache: func() *TokenCache {
				return NewTokenCache()
			},
			expectedOk:  false,
			expectedTok: "",
		},
		{
			name: "valid token",
			setupCache: func() *TokenCache {
				cache := NewTokenCache()
				cache.Set("test-token", 1*time.Hour)
				return cache
			},
			expectedOk:  true,
			expectedTok: "test-token",
		},
		{
			name: "expired token",
			setupCache: func() *TokenCache {
				cache := NewTokenCache()
				cache.Set("test-token", -1*time.Hour) // Already expired
				return cache
			},
			expectedOk:  false,
			expectedTok: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := tt.setupCache()
			token, ok := cache.Get()

			if ok != tt.expectedOk {
				t.Errorf("expected ok=%v, got %v", tt.expectedOk, ok)
			}

			if token != tt.expectedTok {
				t.Errorf("expected token %q, got %q", tt.expectedTok, token)
			}
		})
	}
}

func TestTokenCache_Set(t *testing.T) {
	cache := NewTokenCache()
	token := "test-token-123"
	ttl := 1 * time.Hour

	cache.Set(token, ttl)

	retrieved, ok := cache.Get()
	if !ok {
		t.Fatal("expected token to be retrievable after Set")
	}

	if retrieved != token {
		t.Errorf("expected token %q, got %q", token, retrieved)
	}
}

func TestTokenCache_Clear(t *testing.T) {
	cache := NewTokenCache()
	cache.Set("test-token", 1*time.Hour)

	cache.Clear()

	token, ok := cache.Get()
	if ok {
		t.Errorf("expected token to be cleared, but got %q", token)
	}
}

func TestTokenCache_IsExpired(t *testing.T) {
	tests := []struct {
		name       string
		setupCache func() *TokenCache
		expected   bool
	}{
		{
			name: "empty cache",
			setupCache: func() *TokenCache {
				return NewTokenCache()
			},
			expected: true,
		},
		{
			name: "expired token",
			setupCache: func() *TokenCache {
				cache := NewTokenCache()
				cache.Set("token", -1*time.Hour)
				return cache
			},
			expected: true,
		},
		{
			name: "valid token",
			setupCache: func() *TokenCache {
				cache := NewTokenCache()
				cache.Set("token", 1*time.Hour)
				return cache
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := tt.setupCache()
			result := cache.IsExpired()

			if result != tt.expected {
				t.Errorf("expected IsExpired=%v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTokenCache_ConcurrentAccess(t *testing.T) {
	cache := NewTokenCache()
	const numGoroutines = 100
	const numOps = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				cache.Set("token", 1*time.Hour)
				cache.Get()
				cache.IsExpired()
			}
		}(i)
	}

	wg.Wait()

	// Verify final state
	token, ok := cache.Get()
	if !ok {
		t.Error("expected token to be set after concurrent operations")
	}
	if token != "token" {
		t.Errorf("expected token 'token', got %q", token)
	}
}

func TestTokenCache_ConcurrentReadWrite(t *testing.T) {
	cache := NewTokenCache()
	const numReaders = 50
	const numWriters = 10

	var wg sync.WaitGroup

	// Writers
	wg.Add(numWriters)
	for i := 0; i < numWriters; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				cache.Set("token", 1*time.Hour)
				time.Sleep(1 * time.Millisecond)
				cache.Clear()
			}
		}(i)
	}

	// Readers
	wg.Add(numReaders)
	for i := 0; i < numReaders; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				cache.Get()
				cache.IsExpired()
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
	// Test passes if no race conditions detected
}

func TestTokenCache_TTLExpiration(t *testing.T) {
	cache := NewTokenCache()
	cache.Set("token", 50*time.Millisecond)

	// Should be valid immediately
	if cache.IsExpired() {
		t.Error("token should not be expired immediately after setting")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	if !cache.IsExpired() {
		t.Error("token should be expired after TTL")
	}

	token, ok := cache.Get()
	if ok {
		t.Errorf("expected token to be expired, but got %q", token)
	}
}
