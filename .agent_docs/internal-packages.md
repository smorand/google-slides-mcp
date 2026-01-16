# Internal Packages Reference

Detailed documentation for all internal packages in the `internal/` directory.

## transport/

The `internal/transport/` package provides HTTP server and MCP protocol handling.

### Server (`server.go`)
- HTTP server with configurable port, timeouts, and shutdown
- CORS middleware with configurable allowed origins
- Request logging middleware
- Graceful shutdown on context cancellation

### MCP Handler (`mcp_handler.go`)
- JSON-RPC 2.0 protocol implementation
- MCP initialize handshake with protocol version negotiation
- Tools list and call endpoints
- Chunked transfer encoding for streaming responses

### Key Types
```go
// Server configuration
transport.ServerConfig{
    Port:            8080,
    ReadTimeout:     30 * time.Second,
    WriteTimeout:    60 * time.Second,
    AllowedOrigins:  []string{"*"},
    Logger:          slog.Default(),
}

// JSON-RPC request/response
transport.JSONRPCRequest{
    JSONRPC: "2.0",
    ID:      1,
    Method:  "tools/call",
    Params:  json.RawMessage(`{...}`),
}
```

### Endpoints
- `GET /health` - Health check, returns `{"status": "healthy"}`
- `POST /mcp/initialize` - MCP handshake
- `POST /mcp` - Tool calls (tools/list, tools/call)
- `GET /auth` - Initiate OAuth2 flow, returns authorization URL
- `GET /auth/callback` - OAuth2 callback endpoint

---

## auth/

The `internal/auth/` package provides OAuth2 authentication and API key management.

### OAuth Handler (`oauth.go`)
- OAuth2 flow with Google endpoints
- CSRF protection via state parameter
- Configurable scopes (Slides, Drive, Translate APIs)
- Token callback hook for post-authentication processing
- API key generation and return on successful authentication

### API Key Generation (`apikey.go`)
- Generates UUID v4 format API keys
- Cryptographically secure random generation
- Format: `xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx`

### API Key Store (`store.go`)
- Firestore-backed storage for API keys and refresh tokens
- Document structure: `{api_key, refresh_token, user_email, created_at, last_used}`
- Fast lookups using API key as document ID
- Interface-based design for easy testing with `APIKeyStoreInterface`

### API Key Callback (`callback.go`)
- Creates callback function for OAuth flow
- Generates API key on successful token exchange
- Stores API key and refresh token in Firestore
- Returns API key to user (shown only once)

### Secret Loader (`secrets.go`)
- Load OAuth credentials from Google Secret Manager
- Support for environment-based configuration in development

### Key Types
```go
// OAuth configuration
auth.OAuthConfig{
    ClientID:     "your-client-id",
    ClientSecret: "your-client-secret",
    RedirectURI:  "http://localhost:8080/auth/callback",
    Scopes:       auth.DefaultScopes,  // Optional, uses default if empty
}

// Create handler
handler := auth.NewOAuthHandler(config, logger)

// Set callback for API key generation
store, _ := auth.NewAPIKeyStore(ctx, projectID, "api_keys")
callback := auth.NewAPIKeyCallback(auth.TokenCallbackConfig{
    Store:  store,
    Logger: logger,
})
handler.SetOnTokenFuncWithResult(callback)

// API key record structure
auth.APIKeyRecord{
    APIKey:       "uuid-format-key",
    RefreshToken: "oauth2-refresh-token",
    UserEmail:    "user@example.com",
    CreatedAt:    time.Now(),
    LastUsed:     time.Now(),
}
```

### Default Scopes
- `https://www.googleapis.com/auth/presentations` - Slides API
- `https://www.googleapis.com/auth/drive` - Drive API
- `https://www.googleapis.com/auth/cloud-translation` - Translate API

---

## middleware/

The `internal/middleware/` package provides API key validation for protected endpoints.

### API Key Middleware (`apikey.go`)
- Validates `Authorization: Bearer <api_key>` header
- Lookups API key in Firestore via `APIKeyStoreInterface`
- Creates OAuth2 `TokenSource` from stored refresh token
- Caches validated tokens with configurable TTL (default 5 min)
- Updates `last_used` timestamp asynchronously
- Adds authenticated data to request context

### Context Values
The middleware adds these values to the request context:
- `APIKeyContextKey` - The validated API key string
- `RefreshTokenContextKey` - The associated refresh token
- `UserEmailContextKey` - The user's email address
- `TokenSourceContextKey` - OAuth2 `TokenSource` for API calls

### Helper Functions
```go
// Get API key from context
apiKey := middleware.GetAPIKey(ctx)

// Get token source for API calls
tokenSource := middleware.GetTokenSource(ctx)

// Get authenticated user email
email := middleware.GetUserEmail(ctx)
```

### Configuration
```go
middleware.APIKeyConfig{
    Store:       store,                 // APIKeyStoreInterface
    CacheTTL:    5 * time.Minute,       // Token cache TTL
    Logger:      slog.Default(),
}
```

### Integration with Server
```go
// Wrap handler with API key middleware
apiKeyMiddleware := middleware.NewAPIKeyMiddleware(config)
protectedHandler := apiKeyMiddleware.Wrap(mcpHandler)
```

### Error Responses
- `401 Unauthorized` - Missing or invalid API key
- `403 Forbidden` - API key valid but expired/revoked

### Cache Management
- Token cache entries expire after `CacheTTL`
- Cache is cleared on server restart
- Individual entries can be invalidated via `InvalidateAPIKey(key)`

---

## permissions/

The `internal/permissions/` package handles Drive file permission checks.

### Permission Checker (`checker.go`)
- Verifies user permissions via Drive API `file.capabilities.canEdit`
- Caches permission results with configurable TTL
- Used before modification operations

### Permission Levels
```go
const (
    PermissionNone  = 0  // No access
    PermissionRead  = 1  // Can view
    PermissionWrite = 2  // Can edit
)
```

### Sentinel Errors
```go
permissions.ErrNoWritePermission  // User can read but not edit
permissions.ErrNoReadPermission   // User has no access
permissions.ErrFileNotFound       // File does not exist
```

### Usage Pattern
```go
// Create checker
checker := permissions.NewChecker(permissions.CheckerConfig{
    DriveServiceFactory: driveFactory,
    CacheTTL:            5 * time.Minute,
    Logger:              logger,
})

// Check permission before operation
perm, err := checker.Check(ctx, tokenSource, fileID)
if err != nil {
    return err
}
if perm < permissions.PermissionWrite {
    return permissions.ErrNoWritePermission
}
```

### Configuration
```go
permissions.CheckerConfig{
    DriveServiceFactory: func(ctx, ts) (DriveService, error) { ... },
    CacheTTL:            5 * time.Minute,
    MaxCacheSize:        1000,
    Logger:              slog.Default(),
}
```

### Cache Management
- Cache key: `userEmail:fileID`
- Entries expire after `CacheTTL`
- `Invalidate(email, fileID)` removes specific entry
- `InvalidateUser(email)` removes all entries for user

### Integration with Tools
```go
// In tool implementation
func (t *Tools) ModifySlide(ctx context.Context, ts oauth2.TokenSource, input ModifySlideInput) (*ModifySlideOutput, error) {
    // Check permission first
    perm, err := t.permChecker.Check(ctx, ts, input.PresentationID)
    if err != nil {
        return nil, err
    }
    if perm < permissions.PermissionWrite {
        return nil, ErrAccessDenied
    }
    // Proceed with modification...
}
```

---

## cache/

The `internal/cache/` package provides in-memory caching with LRU eviction and TTL.

### Core LRU Cache (`lru.go`)
- Thread-safe LRU (Least Recently Used) cache
- TTL-based expiration for entries
- Configurable max entries
- Hit/miss metrics tracking

### Configuration
```go
cache.LRUConfig{
    MaxEntries: 1000,           // Maximum entries before eviction
    DefaultTTL: 5 * time.Minute, // Default TTL for entries
}
```

### Typed Cache Wrappers

**Presentation Cache:**
```go
// Cache presentation data
presCache := cache.NewPresentationCache(cache.LRUConfig{
    MaxEntries: 100,
    DefaultTTL: 5 * time.Minute,
})

// Store presentation
presCache.Set(presentationID, presentationData)

// Retrieve (returns nil if not found or expired)
pres := presCache.Get(presentationID)

// Invalidate
presCache.Delete(presentationID)
```

**Token Cache:**
```go
// Cache OAuth tokens by API key
tokenCache := cache.NewTokenCache(cache.LRUConfig{
    MaxEntries: 500,
    DefaultTTL: 55 * time.Minute,  // Just under token expiry
})

// Store token
tokenCache.Set(apiKey, tokenSource)

// Retrieve
ts := tokenCache.Get(apiKey)
```

**Permission Cache:**
```go
// Cache permission check results
permCache := cache.NewPermissionCache(cache.LRUConfig{
    MaxEntries: 1000,
    DefaultTTL: 5 * time.Minute,
})

// Store permission (key format: email:fileID)
permCache.Set(email, fileID, permissionLevel)

// Retrieve
perm := permCache.Get(email, fileID)
```

### Cache Manager (`manager.go`)
Coordinates multiple cache types:

```go
// Create manager with all caches
manager := cache.NewManager(cache.ManagerConfig{
    PresentationConfig: cache.LRUConfig{MaxEntries: 100, DefaultTTL: 5 * time.Minute},
    TokenConfig:        cache.LRUConfig{MaxEntries: 500, DefaultTTL: 55 * time.Minute},
    PermissionConfig:   cache.LRUConfig{MaxEntries: 1000, DefaultTTL: 5 * time.Minute},
})

// Access individual caches
manager.Presentations.Get(id)
manager.Tokens.Get(apiKey)
manager.Permissions.Get(email, fileID)

// Invalidate all caches for a presentation
manager.InvalidatePresentation(presentationID)

// Invalidate all caches for a user
manager.InvalidateUser(email)
```

### Default TTL Values
| Cache Type | Default TTL | Rationale |
|------------|-------------|-----------|
| Presentations | 5 minutes | Balance freshness vs API calls |
| Tokens | 55 minutes | Just under OAuth token expiry (60 min) |
| Permissions | 5 minutes | Permission changes are infrequent |

### Cache Invalidation Strategy
- **On modification**: Invalidate presentation cache after successful update
- **On auth failure**: Invalidate token cache
- **On permission error**: Invalidate permission cache entry

### Metrics
```go
stats := manager.Stats()
fmt.Printf("Presentations: hits=%d, misses=%d, rate=%.1f%%\n",
    stats.Presentations.Hits,
    stats.Presentations.Misses,
    stats.Presentations.HitRate(),
)
```

---

## ratelimit/

The `internal/ratelimit/` package implements request rate limiting.

### Token Bucket Algorithm (`limiter.go`)
- Classic token bucket algorithm for rate limiting
- Per-user rate limits (by API key or IP)
- Burst capacity for handling spikes
- Configurable rates per endpoint

### Rate Limit Headers
Response headers included on rate-limited endpoints:
```
X-RateLimit-Limit: 100        # Max requests per window
X-RateLimit-Remaining: 95     # Remaining requests
X-RateLimit-Reset: 1642857600 # Unix timestamp when limit resets
```

### 429 Response
When limit exceeded:
```json
{
    "error": "rate limit exceeded",
    "retry_after": 30
}
```
Headers: `Retry-After: 30`

### Configuration
```go
ratelimit.Config{
    DefaultRate:  100,              // Requests per minute (default)
    DefaultBurst: 10,               // Burst capacity
    WindowSize:   time.Minute,      // Rate limit window
    PerEndpoint: map[string]EndpointConfig{
        "/mcp": {Rate: 200, Burst: 20},
        "/auth": {Rate: 10, Burst: 2},
    },
}
```

### Usage Pattern
```go
// Create limiter
limiter := ratelimit.NewLimiter(config)

// As middleware
limitedHandler := limiter.Wrap(handler)

// Manual check
if !limiter.Allow(apiKey) {
    limiter.WriteRateLimitResponse(w)
    return
}
```

### Removing Endpoint Limits
```go
// Exclude health check from rate limiting
config.ExcludePaths = []string{"/health"}
```

### Metrics
```go
stats := limiter.Stats()
fmt.Printf("Total requests: %d, Rejected: %d\n",
    stats.TotalRequests,
    stats.RejectedRequests,
)
```

---

## retry/

The `internal/retry/` package provides exponential backoff retry logic.

### Retryer (`retry.go`)
- Exponential backoff with jitter
- Configurable max retries and delays
- Retryable error detection (transient errors only)
- Context-aware (respects cancellation)

### Configuration
```go
retry.Config{
    MaxRetries:      3,
    InitialDelay:    100 * time.Millisecond,
    MaxDelay:        5 * time.Second,
    Multiplier:      2.0,
    JitterFraction:  0.2,  // 20% jitter
    RetryableErrors: []error{ErrTemporary},
    RetryableCodes:  []int{429, 500, 502, 503, 504},
}
```

### Backoff Algorithm
```
delay = min(InitialDelay * (Multiplier ^ attempt), MaxDelay)
delay = delay * (1 + random(-JitterFraction, +JitterFraction))
```

### Sentinel Errors
```go
retry.ErrMaxRetriesExceeded  // All retries exhausted
retry.ErrNotRetryable        // Error is not retryable
retry.ErrContextCanceled     // Context was canceled
```

### RetryableError Type
```go
// Wrap an error as retryable
err := retry.Retryable(originalErr)

// Check if error is retryable
if retry.IsRetryable(err) {
    // Will be retried
}
```

### Usage Pattern
```go
// Create retryer
retryer := retry.NewRetryer(config)

// Retry a function
err := retryer.Do(ctx, func() error {
    return apiCall()
})

// With typed result
result, err := retry.DoWithResult(ctx, retryer, func() (*Result, error) {
    return apiCallWithResult()
})
```

### Config Getters
```go
// Use default config
retryer := retry.NewRetryer(retry.DefaultConfig())

// Customize from default
config := retry.DefaultConfig()
config.MaxRetries = 5
retryer := retry.NewRetryer(config)
```

### Default Values
```go
DefaultConfig() = Config{
    MaxRetries:           5,
    InitialDelay:         1 * time.Second,
    MaxDelay:             16 * time.Second,
    Multiplier:           2.0,
    JitterFactor:         0.2,
    RetryableStatusCodes: []int{429, 500, 502, 503, 504},
}
```

### Key Learnings
- Exponential backoff formula: `delay = initialDelay * multiplier^(attempt-1)`
- Jitter applied as: `delay * (1 - jitterFactor + random(0, 2*jitterFactor))`
- `JitterFactor <= 0` defaults to 0.2 to ensure jitter is applied
- `RetryableError` implements `Unwrap()` for `errors.Is` and `errors.As` compatibility
- Context cancellation check happens both before operation and during delay wait

---

## tools/

The `internal/tools/` package implements all MCP tool handlers.

### Interface-Based Design
- `SlidesService`, `DriveService`, `TranslateService` interfaces
- Factory pattern: `SlidesServiceFactory func(ctx, tokenSource) (SlidesService, error)`
- All tools receive `oauth2.TokenSource` from middleware context

### Key Helper Functions
```go
// Slide lookup (used by many tools)
findSlide(presentation, slideIndex, slideID) (*slides.Page, error)

// Element search (recursive, searches groups)
findElementByID(presentation, objectID) (*slides.PageElement, error)

// Text extraction
extractTextFromTextContent(textContent) string

// EMU conversion
pointsToEMU(points float64) int64  // 1 point = 12700 EMU
emuToPoints(emu int64) float64

// Color parsing
parseHexColor(hex string) (*slides.RgbColor, error)  // "#RRGGBB" -> RGB
```

### Common Patterns
- Validate inputs first, return sentinel errors early
- Use `findSlide` for slide_index/slide_id flexibility
- Use `findElementByID` to find objects anywhere (slides, masters, groups)
- Delete operations from highest index to lowest (avoid shifting)
- Boolean pointers (`*bool`) distinguish "false" from "not set"
