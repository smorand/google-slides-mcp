package cache

import (
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestNewTokenCache(t *testing.T) {
	cache := NewTokenCache(TokenCacheConfig{
		MaxEntries: 10,
		TTL:        55 * time.Minute,
		Logger:     testLogger(),
	})

	if cache == nil {
		t.Fatal("expected cache to be created")
	}
	if cache.Size() != 0 {
		t.Errorf("expected size 0, got %d", cache.Size())
	}
}

func TestTokenCacheSetAndGet(t *testing.T) {
	cache := NewTokenCache(TokenCacheConfig{
		MaxEntries: 10,
		TTL:        55 * time.Minute,
		Logger:     testLogger(),
	})

	token := &CachedToken{
		APIKey:       "api-key-123",
		RefreshToken: "refresh-token-123",
		UserEmail:    "user@example.com",
		AccessToken:  "access-token-123",
		ExpiresAt:    time.Now().Add(55 * time.Minute),
		CachedAt:     time.Now(),
	}

	cache.Set(token)

	// Get the token
	retrieved, ok := cache.Get("api-key-123")
	if !ok {
		t.Fatal("expected token to be found")
	}
	if retrieved.UserEmail != "user@example.com" {
		t.Errorf("expected email 'user@example.com', got '%s'", retrieved.UserEmail)
	}
	if retrieved.AccessToken != "access-token-123" {
		t.Errorf("expected access token 'access-token-123', got '%s'", retrieved.AccessToken)
	}

	// Get non-existent token
	_, ok = cache.Get("nonexistent")
	if ok {
		t.Error("expected token to not be found")
	}
}

func TestTokenCacheExpiration(t *testing.T) {
	cache := NewTokenCache(TokenCacheConfig{
		MaxEntries: 10,
		TTL:        50 * time.Millisecond,
		Logger:     testLogger(),
	})

	token := &CachedToken{
		APIKey:    "api-key-123",
		UserEmail: "user@example.com",
	}

	cache.Set(token)

	// Should be found immediately
	_, ok := cache.Get("api-key-123")
	if !ok {
		t.Fatal("expected token to be found immediately")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	_, ok = cache.Get("api-key-123")
	if ok {
		t.Error("expected token to be expired")
	}
}

func TestTokenCacheSetWithTTL(t *testing.T) {
	cache := NewTokenCache(TokenCacheConfig{
		MaxEntries: 10,
		TTL:        55 * time.Minute,
		Logger:     testLogger(),
	})

	token := &CachedToken{
		APIKey:    "api-key-123",
		UserEmail: "user@example.com",
	}

	cache.SetWithTTL(token, 50*time.Millisecond)

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	_, ok := cache.Get("api-key-123")
	if ok {
		t.Error("expected token to be expired")
	}
}

func TestTokenCacheInvalidate(t *testing.T) {
	cache := NewTokenCache(TokenCacheConfig{
		MaxEntries: 10,
		TTL:        55 * time.Minute,
		Logger:     testLogger(),
	})

	token := &CachedToken{
		APIKey:    "api-key-123",
		UserEmail: "user@example.com",
	}

	cache.Set(token)
	cache.Invalidate("api-key-123")

	_, ok := cache.Get("api-key-123")
	if ok {
		t.Error("expected token to be invalidated")
	}
}

func TestTokenCacheInvalidateByEmail(t *testing.T) {
	cache := NewTokenCache(TokenCacheConfig{
		MaxEntries: 10,
		TTL:        55 * time.Minute,
		Logger:     testLogger(),
	})

	// Add tokens for different users
	cache.Set(&CachedToken{APIKey: "api-key-1", UserEmail: "user1@example.com"})
	cache.Set(&CachedToken{APIKey: "api-key-2", UserEmail: "user1@example.com"})
	cache.Set(&CachedToken{APIKey: "api-key-3", UserEmail: "user2@example.com"})

	// Invalidate all tokens for user1
	count := cache.InvalidateByEmail("user1@example.com")
	if count != 2 {
		t.Errorf("expected 2 tokens invalidated, got %d", count)
	}

	// Verify user1 tokens are gone
	_, ok := cache.Get("api-key-1")
	if ok {
		t.Error("expected api-key-1 to be invalidated")
	}
	_, ok = cache.Get("api-key-2")
	if ok {
		t.Error("expected api-key-2 to be invalidated")
	}

	// Verify user2 token is still there
	_, ok = cache.Get("api-key-3")
	if !ok {
		t.Error("expected api-key-3 to still exist")
	}
}

func TestTokenCacheClear(t *testing.T) {
	cache := NewTokenCache(TokenCacheConfig{
		MaxEntries: 10,
		TTL:        55 * time.Minute,
		Logger:     testLogger(),
	})

	cache.Set(&CachedToken{APIKey: "api-key-1", UserEmail: "user1@example.com"})
	cache.Set(&CachedToken{APIKey: "api-key-2", UserEmail: "user2@example.com"})

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("expected size 0, got %d", cache.Size())
	}
}

func TestTokenCacheMetrics(t *testing.T) {
	cache := NewTokenCache(TokenCacheConfig{
		MaxEntries: 10,
		TTL:        55 * time.Minute,
		Logger:     testLogger(),
	})

	cache.Set(&CachedToken{APIKey: "api-key-1", UserEmail: "user1@example.com"})

	// 1 hit
	cache.Get("api-key-1")

	// 1 miss
	cache.Get("api-key-2")

	metrics := cache.Metrics()
	if metrics.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", metrics.Hits)
	}
	if metrics.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", metrics.Misses)
	}
}

func TestTokenCacheCleanup(t *testing.T) {
	cache := NewTokenCache(TokenCacheConfig{
		MaxEntries: 10,
		TTL:        50 * time.Millisecond,
		Logger:     testLogger(),
	})

	cache.Set(&CachedToken{APIKey: "api-key-1", UserEmail: "user1@example.com"})
	cache.Set(&CachedToken{APIKey: "api-key-2", UserEmail: "user2@example.com"})

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	count := cache.Cleanup()
	if count != 2 {
		t.Errorf("expected 2 expired entries, got %d", count)
	}

	if cache.Size() != 0 {
		t.Errorf("expected size 0, got %d", cache.Size())
	}
}

func TestDefaultTokenCacheConfig(t *testing.T) {
	config := DefaultTokenCacheConfig()

	if config.MaxEntries != 500 {
		t.Errorf("expected max entries 500, got %d", config.MaxEntries)
	}
	if config.TTL != 55*time.Minute {
		t.Errorf("expected TTL 55 minutes, got %v", config.TTL)
	}
}

func TestTokenCacheWithTokenSource(t *testing.T) {
	cache := NewTokenCache(TokenCacheConfig{
		MaxEntries: 10,
		TTL:        55 * time.Minute,
		Logger:     testLogger(),
	})

	// Create a mock token source
	mockToken := &oauth2.Token{
		AccessToken:  "mock-access-token",
		RefreshToken: "mock-refresh-token",
		Expiry:       time.Now().Add(1 * time.Hour),
	}
	tokenSource := oauth2.StaticTokenSource(mockToken)

	token := &CachedToken{
		APIKey:       "api-key-123",
		RefreshToken: "refresh-token-123",
		UserEmail:    "user@example.com",
		TokenSource:  tokenSource,
		AccessToken:  "access-token-123",
		ExpiresAt:    time.Now().Add(55 * time.Minute),
		CachedAt:     time.Now(),
	}

	cache.Set(token)

	// Get the token
	retrieved, ok := cache.Get("api-key-123")
	if !ok {
		t.Fatal("expected token to be found")
	}
	if retrieved.TokenSource == nil {
		t.Error("expected token source to be cached")
	}

	// Verify the token source works
	tok, err := retrieved.TokenSource.Token()
	if err != nil {
		t.Fatalf("expected token source to return token: %v", err)
	}
	if tok.AccessToken != "mock-access-token" {
		t.Errorf("expected access token 'mock-access-token', got '%s'", tok.AccessToken)
	}
}
