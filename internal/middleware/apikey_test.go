package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/smorand/google-slides-mcp/internal/auth"
)

func TestExtractAPIKey(t *testing.T) {
	tests := []struct {
		name        string
		authHeader  string
		wantAPIKey  string
		wantErr     error
	}{
		{
			name:        "valid bearer token",
			authHeader:  "Bearer test-api-key-12345",
			wantAPIKey:  "test-api-key-12345",
			wantErr:     nil,
		},
		{
			name:        "valid bearer token lowercase",
			authHeader:  "bearer test-api-key-12345",
			wantAPIKey:  "test-api-key-12345",
			wantErr:     nil,
		},
		{
			name:        "valid bearer token with extra spaces",
			authHeader:  "Bearer   test-api-key-12345  ",
			wantAPIKey:  "test-api-key-12345",
			wantErr:     nil,
		},
		{
			name:        "missing authorization header",
			authHeader:  "",
			wantAPIKey:  "",
			wantErr:     ErrMissingAuthHeader,
		},
		{
			name:        "invalid format - no bearer",
			authHeader:  "Basic test-api-key-12345",
			wantAPIKey:  "",
			wantErr:     ErrInvalidAuthHeader,
		},
		{
			name:        "invalid format - no token",
			authHeader:  "Bearer",
			wantAPIKey:  "",
			wantErr:     ErrInvalidAuthHeader,
		},
		{
			name:        "invalid format - only spaces after bearer",
			authHeader:  "Bearer   ",
			wantAPIKey:  "",
			wantErr:     ErrInvalidAuthHeader,
		},
		{
			name:        "invalid format - no space",
			authHeader:  "Bearertoken",
			wantAPIKey:  "",
			wantErr:     ErrInvalidAuthHeader,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			apiKey, err := extractAPIKey(req)

			if err != tt.wantErr {
				t.Errorf("extractAPIKey() error = %v, want %v", err, tt.wantErr)
			}

			if apiKey != tt.wantAPIKey {
				t.Errorf("extractAPIKey() = %v, want %v", apiKey, tt.wantAPIKey)
			}
		})
	}
}

func TestAPIKeyMiddleware_RequestWithoutAPIKey_Returns401(t *testing.T) {
	store := auth.NewMockAPIKeyStore()
	middleware := NewAPIKeyMiddleware(APIKeyMiddlewareConfig{
		Store:             store,
		OAuthClientID:     "test-client-id",
		OAuthClientSecret: "test-client-secret",
	})

	handler := middleware.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != ErrMissingAuthHeader.Error() {
		t.Errorf("expected error %q, got %q", ErrMissingAuthHeader.Error(), response["error"])
	}
}

func TestAPIKeyMiddleware_RequestWithInvalidAPIKey_Returns401(t *testing.T) {
	store := auth.NewMockAPIKeyStore()
	middleware := NewAPIKeyMiddleware(APIKeyMiddlewareConfig{
		Store:             store,
		OAuthClientID:     "test-client-id",
		OAuthClientSecret: "test-client-secret",
	})

	handler := middleware.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-api-key")
	rr := httptest.NewRecorder()

	handler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != ErrInvalidAPIKey.Error() {
		t.Errorf("expected error %q, got %q", ErrInvalidAPIKey.Error(), response["error"])
	}
}

func TestAPIKeyMiddleware_RequestWithValidAPIKey_Succeeds(t *testing.T) {
	store := auth.NewMockAPIKeyStore()

	// Store a valid API key
	testRecord := &auth.APIKeyRecord{
		APIKey:       "valid-api-key-12345",
		RefreshToken: "test-refresh-token",
		UserEmail:    "test@example.com",
		CreatedAt:    time.Now(),
		LastUsed:     time.Now(),
	}
	if err := store.Store(context.Background(), testRecord); err != nil {
		t.Fatalf("failed to store test record: %v", err)
	}

	middleware := NewAPIKeyMiddleware(APIKeyMiddlewareConfig{
		Store:             store,
		OAuthClientID:     "test-client-id",
		OAuthClientSecret: "test-client-secret",
		UpdateLastUsed:    false, // Disable async update for test
	})

	var capturedAPIKey, capturedRefreshToken, capturedUserEmail string
	handler := middleware.Middleware(func(w http.ResponseWriter, r *http.Request) {
		capturedAPIKey = GetAPIKey(r.Context())
		capturedRefreshToken = GetRefreshToken(r.Context())
		capturedUserEmail = GetUserEmail(r.Context())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-api-key-12345")
	rr := httptest.NewRecorder()

	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	if capturedAPIKey != "valid-api-key-12345" {
		t.Errorf("expected API key %q, got %q", "valid-api-key-12345", capturedAPIKey)
	}

	if capturedRefreshToken != "test-refresh-token" {
		t.Errorf("expected refresh token %q, got %q", "test-refresh-token", capturedRefreshToken)
	}

	if capturedUserEmail != "test@example.com" {
		t.Errorf("expected user email %q, got %q", "test@example.com", capturedUserEmail)
	}
}

func TestAPIKeyMiddleware_TokenCaching(t *testing.T) {
	store := auth.NewMockAPIKeyStore()

	// Store a valid API key
	testRecord := &auth.APIKeyRecord{
		APIKey:       "cached-api-key-12345",
		RefreshToken: "test-refresh-token",
		UserEmail:    "test@example.com",
		CreatedAt:    time.Now(),
		LastUsed:     time.Now(),
	}
	if err := store.Store(context.Background(), testRecord); err != nil {
		t.Fatalf("failed to store test record: %v", err)
	}

	middleware := NewAPIKeyMiddleware(APIKeyMiddlewareConfig{
		Store:             store,
		OAuthClientID:     "test-client-id",
		OAuthClientSecret: "test-client-secret",
		CacheTTL:          5 * time.Minute,
		UpdateLastUsed:    false, // Disable async update for test
	})

	handler := middleware.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// First request - should hit the store
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer cached-api-key-12345")
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", rr.Code)
	}

	initialGetCalls := store.GetCalls

	// Second request - should use cache
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer cached-api-key-12345")
	rr = httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("second request: expected status 200, got %d", rr.Code)
	}

	// Store.Get should not have been called again
	if store.GetCalls != initialGetCalls {
		t.Errorf("expected %d Get calls (cached), got %d", initialGetCalls, store.GetCalls)
	}

	// Verify cache size
	if middleware.CacheSize() != 1 {
		t.Errorf("expected cache size 1, got %d", middleware.CacheSize())
	}
}

func TestAPIKeyMiddleware_CacheExpiration(t *testing.T) {
	store := auth.NewMockAPIKeyStore()

	// Store a valid API key
	testRecord := &auth.APIKeyRecord{
		APIKey:       "expiring-api-key-12345",
		RefreshToken: "test-refresh-token",
		UserEmail:    "test@example.com",
		CreatedAt:    time.Now(),
		LastUsed:     time.Now(),
	}
	if err := store.Store(context.Background(), testRecord); err != nil {
		t.Fatalf("failed to store test record: %v", err)
	}

	// Use a very short TTL
	middleware := NewAPIKeyMiddleware(APIKeyMiddlewareConfig{
		Store:             store,
		OAuthClientID:     "test-client-id",
		OAuthClientSecret: "test-client-secret",
		CacheTTL:          1 * time.Millisecond,
		UpdateLastUsed:    false,
	})

	handler := middleware.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// First request
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer expiring-api-key-12345")
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", rr.Code)
	}

	initialGetCalls := store.GetCalls

	// Wait for cache to expire
	time.Sleep(5 * time.Millisecond)

	// Second request - should hit store again due to cache expiration
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer expiring-api-key-12345")
	rr = httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("second request: expected status 200, got %d", rr.Code)
	}

	// Store.Get should have been called again
	if store.GetCalls <= initialGetCalls {
		t.Errorf("expected more Get calls after cache expiration, got %d", store.GetCalls)
	}
}

func TestAPIKeyMiddleware_InvalidateCache(t *testing.T) {
	store := auth.NewMockAPIKeyStore()

	// Store a valid API key
	testRecord := &auth.APIKeyRecord{
		APIKey:       "invalidate-api-key-12345",
		RefreshToken: "test-refresh-token",
		UserEmail:    "test@example.com",
		CreatedAt:    time.Now(),
		LastUsed:     time.Now(),
	}
	if err := store.Store(context.Background(), testRecord); err != nil {
		t.Fatalf("failed to store test record: %v", err)
	}

	middleware := NewAPIKeyMiddleware(APIKeyMiddlewareConfig{
		Store:             store,
		OAuthClientID:     "test-client-id",
		OAuthClientSecret: "test-client-secret",
		CacheTTL:          5 * time.Minute,
		UpdateLastUsed:    false,
	})

	handler := middleware.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// First request - populate cache
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalidate-api-key-12345")
	rr := httptest.NewRecorder()
	handler(rr, req)

	if middleware.CacheSize() != 1 {
		t.Errorf("expected cache size 1, got %d", middleware.CacheSize())
	}

	// Invalidate cache
	middleware.InvalidateCache("invalidate-api-key-12345")

	if middleware.CacheSize() != 0 {
		t.Errorf("expected cache size 0 after invalidation, got %d", middleware.CacheSize())
	}
}

func TestAPIKeyMiddleware_ClearCache(t *testing.T) {
	store := auth.NewMockAPIKeyStore()

	// Store multiple API keys
	for i := 0; i < 3; i++ {
		testRecord := &auth.APIKeyRecord{
			APIKey:       "clear-api-key-" + string(rune('1'+i)),
			RefreshToken: "test-refresh-token",
			UserEmail:    "test@example.com",
			CreatedAt:    time.Now(),
			LastUsed:     time.Now(),
		}
		if err := store.Store(context.Background(), testRecord); err != nil {
			t.Fatalf("failed to store test record: %v", err)
		}
	}

	middleware := NewAPIKeyMiddleware(APIKeyMiddlewareConfig{
		Store:             store,
		OAuthClientID:     "test-client-id",
		OAuthClientSecret: "test-client-secret",
		CacheTTL:          5 * time.Minute,
		UpdateLastUsed:    false,
	})

	handler := middleware.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Make requests to populate cache
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer clear-api-key-"+string(rune('1'+i)))
		rr := httptest.NewRecorder()
		handler(rr, req)
	}

	if middleware.CacheSize() != 3 {
		t.Errorf("expected cache size 3, got %d", middleware.CacheSize())
	}

	// Clear cache
	middleware.ClearCache()

	if middleware.CacheSize() != 0 {
		t.Errorf("expected cache size 0 after clear, got %d", middleware.CacheSize())
	}
}

func TestAPIKeyMiddleware_LastUsedUpdate(t *testing.T) {
	store := auth.NewMockAPIKeyStore()

	// Store a valid API key
	testRecord := &auth.APIKeyRecord{
		APIKey:       "lastused-api-key-12345",
		RefreshToken: "test-refresh-token",
		UserEmail:    "test@example.com",
		CreatedAt:    time.Now(),
		LastUsed:     time.Now(),
	}
	if err := store.Store(context.Background(), testRecord); err != nil {
		t.Fatalf("failed to store test record: %v", err)
	}

	middleware := NewAPIKeyMiddleware(APIKeyMiddlewareConfig{
		Store:             store,
		OAuthClientID:     "test-client-id",
		OAuthClientSecret: "test-client-secret",
		UpdateLastUsed:    true, // Enable last_used update
	})

	handler := middleware.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer lastused-api-key-12345")
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Wait a bit for async update
	time.Sleep(50 * time.Millisecond)

	// Check that UpdateLastUsed was called
	if store.UpdateLastUsedCall == 0 {
		t.Error("expected UpdateLastUsed to be called")
	}
}

func TestAPIKeyMiddleware_StoreLookupError(t *testing.T) {
	store := auth.NewMockAPIKeyStore()
	store.GetError = ErrAPIKeyLookupFailed

	middleware := NewAPIKeyMiddleware(APIKeyMiddlewareConfig{
		Store:             store,
		OAuthClientID:     "test-client-id",
		OAuthClientSecret: "test-client-secret",
	})

	handler := middleware.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer some-api-key")
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestGetContextHelpers(t *testing.T) {
	ctx := context.Background()

	// Test with empty context
	if GetAPIKey(ctx) != "" {
		t.Error("expected empty API key from empty context")
	}
	if GetRefreshToken(ctx) != "" {
		t.Error("expected empty refresh token from empty context")
	}
	if GetUserEmail(ctx) != "" {
		t.Error("expected empty user email from empty context")
	}
	if GetTokenSource(ctx) != nil {
		t.Error("expected nil token source from empty context")
	}

	// Test with populated context
	ctx = context.WithValue(ctx, APIKeyContextKey, "test-api-key")
	ctx = context.WithValue(ctx, RefreshTokenContextKey, "test-refresh-token")
	ctx = context.WithValue(ctx, UserEmailContextKey, "test@example.com")

	if GetAPIKey(ctx) != "test-api-key" {
		t.Errorf("expected API key %q, got %q", "test-api-key", GetAPIKey(ctx))
	}
	if GetRefreshToken(ctx) != "test-refresh-token" {
		t.Errorf("expected refresh token %q, got %q", "test-refresh-token", GetRefreshToken(ctx))
	}
	if GetUserEmail(ctx) != "test@example.com" {
		t.Errorf("expected user email %q, got %q", "test@example.com", GetUserEmail(ctx))
	}
}

func TestDefaultAPIKeyMiddlewareConfig(t *testing.T) {
	config := DefaultAPIKeyMiddlewareConfig()

	if config.CacheTTL != 5*time.Minute {
		t.Errorf("expected CacheTTL %v, got %v", 5*time.Minute, config.CacheTTL)
	}

	if !config.UpdateLastUsed {
		t.Error("expected UpdateLastUsed to be true by default")
	}

	if config.Logger == nil {
		t.Error("expected Logger to be non-nil by default")
	}
}

func TestNewAPIKeyMiddleware_DefaultValues(t *testing.T) {
	store := auth.NewMockAPIKeyStore()

	// Create with zero-value config
	middleware := NewAPIKeyMiddleware(APIKeyMiddlewareConfig{
		Store: store,
	})

	// Verify defaults are applied
	if middleware.config.CacheTTL != 5*time.Minute {
		t.Errorf("expected default CacheTTL %v, got %v", 5*time.Minute, middleware.config.CacheTTL)
	}

	if middleware.config.Logger == nil {
		t.Error("expected default Logger to be set")
	}
}

func TestAPIKeyMiddleware_TokenSourceInContext(t *testing.T) {
	store := auth.NewMockAPIKeyStore()

	// Store a valid API key
	testRecord := &auth.APIKeyRecord{
		APIKey:       "tokensource-api-key-12345",
		RefreshToken: "test-refresh-token",
		UserEmail:    "test@example.com",
		CreatedAt:    time.Now(),
		LastUsed:     time.Now(),
	}
	if err := store.Store(context.Background(), testRecord); err != nil {
		t.Fatalf("failed to store test record: %v", err)
	}

	middleware := NewAPIKeyMiddleware(APIKeyMiddlewareConfig{
		Store:             store,
		OAuthClientID:     "test-client-id",
		OAuthClientSecret: "test-client-secret",
		UpdateLastUsed:    false,
	})

	var hasTokenSource bool
	handler := middleware.Middleware(func(w http.ResponseWriter, r *http.Request) {
		ts := GetTokenSource(r.Context())
		hasTokenSource = ts != nil
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tokensource-api-key-12345")
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	if !hasTokenSource {
		t.Error("expected token source to be present in context")
	}
}
