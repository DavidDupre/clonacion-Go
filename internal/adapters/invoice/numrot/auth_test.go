package numrot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"3tcapital/ms_facturacion_core/internal/testutil"
)

func TestNewAuthManager(t *testing.T) {
	baseURL := "https://api.example.com"
	username := "testuser"
	password := "testpass"
	tokenTTL := 1 * time.Hour
	client := &http.Client{}
	logger := testutil.NewTestLogger()

	auth := NewAuthManager(baseURL, username, password, tokenTTL, client, logger)

	if auth == nil {
		t.Fatal("expected auth manager to be created, got nil")
	}

	if auth.baseURL != baseURL {
		t.Errorf("expected baseURL %q, got %q", baseURL, auth.baseURL)
	}

	if auth.username != username {
		t.Errorf("expected username %q, got %q", username, auth.username)
	}

	if auth.password != password {
		t.Errorf("expected password %q, got %q", password, auth.password)
	}

	if auth.tokenTTL != tokenTTL {
		t.Errorf("expected tokenTTL %v, got %v", tokenTTL, auth.tokenTTL)
	}

	if auth.client != client {
		t.Error("expected client to be set")
	}

	if auth.cache == nil {
		t.Error("expected cache to be initialized")
	}
}

func TestAuthManager_GetToken_Cached(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("cached-token"))
	}))
	defer server.Close()

	auth := NewAuthManager(server.URL, "user", "pass", 1*time.Hour, server.Client(), testutil.NewTestLogger())

	// First call should authenticate
	token1, err := auth.GetToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second call should use cache
	token2, err := auth.GetToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if token1 != token2 {
		t.Errorf("expected cached token, got different tokens: %q vs %q", token1, token2)
	}
}

func TestAuthManager_GetToken_Expired(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("token-" + string(rune(callCount+'0'))))
	}))
	defer server.Close()

	auth := NewAuthManager(server.URL, "user", "pass", 50*time.Millisecond, server.Client(), testutil.NewTestLogger())

	// First call
	_, err := auth.GetToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Second call should re-authenticate
	_, err = auth.GetToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 authentication calls, got %d", callCount)
	}
}

func TestAuthManager_GetToken_Concurrent(t *testing.T) {
	callCount := 0
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		time.Sleep(10 * time.Millisecond) // Simulate network delay
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("token"))
	}))
	defer server.Close()

	auth := NewAuthManager(server.URL, "user", "pass", 1*time.Hour, server.Client(), testutil.NewTestLogger())

	var wg sync.WaitGroup
	const numGoroutines = 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := auth.GetToken(context.Background())
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if token != "token" {
				t.Errorf("expected token 'token', got %q", token)
			}
		}()
	}

	wg.Wait()

	// Should only authenticate once due to mutex protection
	mu.Lock()
	actualCalls := callCount
	mu.Unlock()

	if actualCalls != 1 {
		t.Errorf("expected 1 authentication call, got %d", actualCalls)
	}
}

func TestAuthManager_authenticate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v2/api/Token" {
			t.Errorf("expected path /v2/api/Token, got %s", r.URL.Path)
		}

		var reqBody map[string]string
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		if reqBody["username"] != "testuser" {
			t.Errorf("expected username 'testuser', got %q", reqBody["username"])
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test-token-123"))
	}))
	defer server.Close()

	auth := NewAuthManager(server.URL, "testuser", "testpass", 1*time.Hour, server.Client(), testutil.NewTestLogger())

	token, err := auth.authenticate(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if token != "test-token-123" {
		t.Errorf("expected token 'test-token-123', got %q", token)
	}
}

func TestAuthManager_authenticate_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	auth := NewAuthManager(server.URL, "user", "pass", 1*time.Hour, server.Client(), testutil.NewTestLogger())

	_, err := auth.authenticate(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestAuthManager_authenticate_EmptyToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("   \n\t  ")) // Whitespace only
	}))
	defer server.Close()

	auth := NewAuthManager(server.URL, "user", "pass", 1*time.Hour, server.Client(), testutil.NewTestLogger())

	_, err := auth.authenticate(context.Background())
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestAuthManager_ClearToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("token"))
	}))
	defer server.Close()

	auth := NewAuthManager(server.URL, "user", "pass", 1*time.Hour, server.Client(), testutil.NewTestLogger())

	// Get and cache token
	_, err := auth.GetToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Clear token
	auth.ClearToken()

	// Next call should re-authenticate
	_, err = auth.GetToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
