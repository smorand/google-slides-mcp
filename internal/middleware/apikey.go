package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/smorand/google-slides-mcp/internal/auth"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Context keys for storing authenticated data.
type contextKey string

const (
	// APIKeyContextKey is the context key for the API key.
	APIKeyContextKey contextKey = "api_key"
	// RefreshTokenContextKey is the context key for the refresh token.
	RefreshTokenContextKey contextKey = "refresh_token"
	// UserEmailContextKey is the context key for the user email.
	UserEmailContextKey contextKey = "user_email"
	// TokenSourceContextKey is the context key for the oauth2 token source.
	TokenSourceContextKey contextKey = "token_source"
)

// Sentinel errors for API key validation.
var (
	ErrMissingAuthHeader   = errors.New("missing Authorization header")
	ErrInvalidAuthHeader   = errors.New("invalid Authorization header format")
	ErrInvalidAPIKey       = errors.New("invalid API key")
	ErrAPIKeyLookupFailed  = errors.New("failed to lookup API key")
	ErrTokenRefreshFailed  = errors.New("failed to refresh token")
)

// CachedToken holds a cached access token with expiration.
type CachedToken struct {
	Record      *auth.APIKeyRecord
	TokenSource oauth2.TokenSource
	CachedAt    time.Time
}

// APIKeyMiddlewareConfig holds configuration for the API key middleware.
type APIKeyMiddlewareConfig struct {
	Store              auth.APIKeyStoreInterface
	OAuthClientID      string
	OAuthClientSecret  string
	CacheTTL           time.Duration // Default 5 minutes
	UpdateLastUsed     bool          // Whether to update last_used timestamp (default true)
	Logger             *slog.Logger
}

// DefaultAPIKeyMiddlewareConfig returns default configuration.
func DefaultAPIKeyMiddlewareConfig() APIKeyMiddlewareConfig {
	return APIKeyMiddlewareConfig{
		CacheTTL:       5 * time.Minute,
		UpdateLastUsed: true,
		Logger:         slog.Default(),
	}
}

// APIKeyMiddleware validates API keys and creates OAuth token sources.
type APIKeyMiddleware struct {
	config APIKeyMiddlewareConfig
	cache  map[string]*CachedToken
	mu     sync.RWMutex
}

// NewAPIKeyMiddleware creates a new API key middleware.
func NewAPIKeyMiddleware(config APIKeyMiddlewareConfig) *APIKeyMiddleware {
	if config.CacheTTL == 0 {
		config.CacheTTL = 5 * time.Minute
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	return &APIKeyMiddleware{
		config: config,
		cache:  make(map[string]*CachedToken),
	}
}

// Middleware returns an HTTP middleware function that validates API keys.
func (m *APIKeyMiddleware) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Extract API key from Authorization header
		apiKey, err := extractAPIKey(r)
		if err != nil {
			m.writeUnauthorized(w, err.Error())
			return
		}

		// Validate API key and get token source
		cachedToken, err := m.validateAndGetToken(ctx, apiKey)
		if err != nil {
			if errors.Is(err, ErrInvalidAPIKey) || errors.Is(err, ErrAPIKeyLookupFailed) {
				m.writeUnauthorized(w, err.Error())
				return
			}
			m.config.Logger.Error("failed to validate API key", slog.Any("error", err))
			m.writeError(w, http.StatusInternalServerError, "authentication failed")
			return
		}

		// Update last_used timestamp asynchronously if enabled
		if m.config.UpdateLastUsed {
			go m.updateLastUsed(context.Background(), apiKey)
		}

		// Add authenticated data to context
		ctx = context.WithValue(ctx, APIKeyContextKey, apiKey)
		ctx = context.WithValue(ctx, RefreshTokenContextKey, cachedToken.Record.RefreshToken)
		ctx = context.WithValue(ctx, UserEmailContextKey, cachedToken.Record.UserEmail)
		ctx = context.WithValue(ctx, TokenSourceContextKey, cachedToken.TokenSource)

		// Call the next handler with the enriched context
		next(w, r.WithContext(ctx))
	}
}

// extractAPIKey extracts the API key from the Authorization header.
func extractAPIKey(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", ErrMissingAuthHeader
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", ErrInvalidAuthHeader
	}

	apiKey := strings.TrimSpace(parts[1])
	if apiKey == "" {
		return "", ErrInvalidAuthHeader
	}

	return apiKey, nil
}

// validateAndGetToken validates the API key and returns a cached token.
func (m *APIKeyMiddleware) validateAndGetToken(ctx context.Context, apiKey string) (*CachedToken, error) {
	// Check cache first
	m.mu.RLock()
	cached, ok := m.cache[apiKey]
	m.mu.RUnlock()

	if ok && time.Since(cached.CachedAt) < m.config.CacheTTL {
		m.config.Logger.Debug("cache hit for API key")
		return cached, nil
	}

	m.config.Logger.Debug("cache miss for API key, looking up in store")

	// Lookup in store
	record, err := m.config.Store.Get(ctx, apiKey)
	if err != nil {
		// Check if it's a "not found" error
		if strings.Contains(err.Error(), "not found") {
			return nil, ErrInvalidAPIKey
		}
		return nil, ErrAPIKeyLookupFailed
	}

	// Create token source from refresh token
	tokenSource, err := m.createTokenSource(ctx, record.RefreshToken)
	if err != nil {
		return nil, err
	}

	// Cache the result
	cachedToken := &CachedToken{
		Record:      record,
		TokenSource: tokenSource,
		CachedAt:    time.Now(),
	}

	m.mu.Lock()
	m.cache[apiKey] = cachedToken
	m.mu.Unlock()

	return cachedToken, nil
}

// createTokenSource creates an OAuth2 token source from a refresh token.
func (m *APIKeyMiddleware) createTokenSource(ctx context.Context, refreshToken string) (oauth2.TokenSource, error) {
	config := &oauth2.Config{
		ClientID:     m.config.OAuthClientID,
		ClientSecret: m.config.OAuthClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       auth.DefaultScopes,
	}

	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	return config.TokenSource(ctx, token), nil
}

// updateLastUsed updates the last_used timestamp in the store.
func (m *APIKeyMiddleware) updateLastUsed(ctx context.Context, apiKey string) {
	if err := m.config.Store.UpdateLastUsed(ctx, apiKey); err != nil {
		m.config.Logger.Error("failed to update last_used timestamp",
			slog.String("api_key", apiKey[:8]+"..."),
			slog.Any("error", err),
		)
	}
}

// InvalidateCache removes an API key from the cache.
func (m *APIKeyMiddleware) InvalidateCache(apiKey string) {
	m.mu.Lock()
	delete(m.cache, apiKey)
	m.mu.Unlock()
}

// ClearCache clears all cached tokens.
func (m *APIKeyMiddleware) ClearCache() {
	m.mu.Lock()
	m.cache = make(map[string]*CachedToken)
	m.mu.Unlock()
}

// CacheSize returns the number of cached tokens.
func (m *APIKeyMiddleware) CacheSize() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.cache)
}

// writeUnauthorized writes a 401 Unauthorized response.
func (m *APIKeyMiddleware) writeUnauthorized(w http.ResponseWriter, message string) {
	m.writeError(w, http.StatusUnauthorized, message)
}

// writeError writes a JSON error response.
func (m *APIKeyMiddleware) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

// GetAPIKey retrieves the API key from the request context.
func GetAPIKey(ctx context.Context) string {
	if v := ctx.Value(APIKeyContextKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetRefreshToken retrieves the refresh token from the request context.
func GetRefreshToken(ctx context.Context) string {
	if v := ctx.Value(RefreshTokenContextKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetUserEmail retrieves the user email from the request context.
func GetUserEmail(ctx context.Context) string {
	if v := ctx.Value(UserEmailContextKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetTokenSource retrieves the OAuth2 token source from the request context.
func GetTokenSource(ctx context.Context) oauth2.TokenSource {
	if v := ctx.Value(TokenSourceContextKey); v != nil {
		return v.(oauth2.TokenSource)
	}
	return nil
}
