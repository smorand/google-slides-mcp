package cache

import (
	"log/slog"
	"time"

	"golang.org/x/oauth2"
)

// CachedToken holds a cached OAuth2 token with metadata.
type CachedToken struct {
	APIKey       string
	RefreshToken string
	UserEmail    string
	TokenSource  oauth2.TokenSource
	AccessToken  string
	ExpiresAt    time.Time
	CachedAt     time.Time
}

// TokenCacheConfig holds configuration for the token cache.
type TokenCacheConfig struct {
	MaxEntries int           // Maximum number of tokens to cache
	TTL        time.Duration // TTL for token entries (should be less than token expiry)
	Logger     *slog.Logger
}

// DefaultTokenCacheConfig returns default configuration.
// Tokens typically expire in 60 minutes, so we use 55 minutes as default TTL.
func DefaultTokenCacheConfig() TokenCacheConfig {
	return TokenCacheConfig{
		MaxEntries: 500,
		TTL:        55 * time.Minute,
		Logger:     slog.Default(),
	}
}

// TokenCache caches OAuth2 tokens for faster access.
type TokenCache struct {
	lru    *LRU
	config TokenCacheConfig
}

// NewTokenCache creates a new token cache.
func NewTokenCache(config TokenCacheConfig) *TokenCache {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.TTL == 0 {
		config.TTL = 55 * time.Minute
	}
	if config.MaxEntries == 0 {
		config.MaxEntries = 500
	}

	return &TokenCache{
		lru: NewLRU(LRUConfig{
			MaxEntries: config.MaxEntries,
			DefaultTTL: config.TTL,
			Logger:     config.Logger,
		}),
		config: config,
	}
}

// Get retrieves a token from the cache by API key.
func (c *TokenCache) Get(apiKey string) (*CachedToken, bool) {
	val, ok := c.lru.Get(apiKey)
	if !ok {
		return nil, false
	}
	return val.(*CachedToken), true
}

// Set stores a token in the cache.
func (c *TokenCache) Set(token *CachedToken) {
	c.lru.SetWithTTL(token.APIKey, token, c.config.TTL)
}

// SetWithTTL stores a token in the cache with a specific TTL.
func (c *TokenCache) SetWithTTL(token *CachedToken, ttl time.Duration) {
	c.lru.SetWithTTL(token.APIKey, token, ttl)
}

// Invalidate removes a token from the cache.
func (c *TokenCache) Invalidate(apiKey string) {
	c.lru.Delete(apiKey)
}

// InvalidateByEmail removes all tokens for a specific email.
func (c *TokenCache) InvalidateByEmail(email string) int {
	count := 0
	for _, key := range c.lru.Keys() {
		val, ok := c.lru.Get(key)
		if ok {
			token := val.(*CachedToken)
			if token.UserEmail == email {
				c.lru.Delete(key)
				count++
			}
		}
	}
	return count
}

// Clear removes all tokens from the cache.
func (c *TokenCache) Clear() {
	c.lru.Clear()
}

// Size returns the number of cached tokens.
func (c *TokenCache) Size() int {
	return c.lru.Size()
}

// Metrics returns cache metrics.
func (c *TokenCache) Metrics() Metrics {
	return c.lru.Metrics()
}

// Cleanup removes expired entries.
func (c *TokenCache) Cleanup() int {
	return c.lru.Cleanup()
}
