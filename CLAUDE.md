# Google Slides MCP Server - Development Guidelines

## Project Overview

HTTP streamable MCP server for Google Slides API integration. Built in Go with deployment to Google Cloud Run.

## Quick Commands

```bash
make build    # Build for current platform
make test     # Run all tests
make run      # Build and run locally
make fmt      # Format code
make vet      # Run go vet
make check    # Run all checks (fmt, vet, lint, test)
make clean    # Remove build artifacts

# Docker commands
docker build -t google-slides-mcp .              # Build container image
docker run -p 8080:8080 google-slides-mcp        # Run container locally

# Terraform commands
make plan     # Plan infrastructure changes
make deploy   # Deploy infrastructure
make undeploy # Destroy infrastructure
```

## Project Structure

```
google-slides-mcp/
├── cmd/google-slides-mcp/    # Entry point only - minimal code
├── internal/                 # All implementation code goes here
│   ├── auth/                # OAuth2 flow, API key generation
│   ├── cache/               # In-memory caching with TTL
│   ├── middleware/          # API key validation, logging
│   ├── permissions/         # Drive permission checks
│   ├── ratelimit/           # Token bucket rate limiting
│   ├── retry/               # Exponential backoff retry
│   ├── tools/               # MCP tool implementations
│   └── transport/           # HTTP server, MCP protocol
├── pkg/                     # Public APIs (if any)
├── terraform/               # GCP infrastructure (Terraform)
│   ├── config.yaml         # Configuration file
│   ├── provider.tf         # Provider and backend config
│   ├── local.tf            # Local variables from config
│   ├── apis.tf             # API enablement
│   ├── iam.tf              # Service accounts and roles
│   ├── cloudrun.tf         # Cloud Run service
│   ├── firestore.tf        # Firestore database
│   └── secrets.tf          # Secret Manager secrets
└── scripts/                 # Utility scripts
```

## Code Conventions

### Go Standards
- Follow Go standard project layout (no `/src` directory)
- Use `internal/` for all private code
- Entry point in `cmd/` should only wire dependencies
- Package names are lowercase, single words
- Use `context.Context` as first parameter

### Error Handling
- Always handle errors explicitly
- Use `fmt.Errorf("context: %w", err)` for wrapping
- Define sentinel errors for expected conditions
- Log errors at the boundary, not in helper functions

### Testing
- Use table-driven tests
- Prefer standard library over testify
- Test files in same package as code
- Use `_test.go` suffix

## MCP Protocol

The server implements MCP (Model Context Protocol) over HTTP:
- Endpoint: `POST /mcp` for tool calls
- Format: JSON-RPC 2.0
- Transport: Chunked transfer encoding for streaming

### Tool Implementation Pattern

```go
// internal/tools/example.go
package tools

type ExampleInput struct {
    PresentationID string `json:"presentation_id"`
}

type ExampleOutput struct {
    Result string `json:"result"`
}

func (t *Tools) Example(ctx context.Context, input ExampleInput) (*ExampleOutput, error) {
    // Implementation
    return &ExampleOutput{}, nil
}
```

## Authentication Flow

1. `/auth` - Initiates OAuth2 flow
2. `/auth/callback` - Receives OAuth2 code
3. Exchange code for tokens
4. Generate API key, store in Firestore
5. Return API key to user

All subsequent requests require `Authorization: Bearer <api_key>` header.

## Google APIs Used

- **Slides API**: Presentation manipulation
- **Drive API**: File search, permissions, copy operations
- **Translate API**: Text translation

## Deployment

Terraform manages GCP infrastructure:
- Cloud Run service
- Firestore database
- Secret Manager secrets
- IAM roles

### Terraform Structure

The `terraform/` folder follows feature-based organization:
- `config.yaml` - Single source of configuration
- `provider.tf` - Google provider and backend
- `local.tf` - Loads config.yaml, defines derived values
- `apis.tf` - Enables required Google APIs
- `iam.tf` - Service accounts for Cloud Run and Cloud Build
- `cloudrun.tf` - MCP server deployment
- `firestore.tf` - Database for API keys and tokens
- `secrets.tf` - OAuth2 credentials storage

### Deployment Commands

```bash
# Initialize and deploy
cd terraform
terraform init
terraform plan
terraform apply

# Or use Makefile
make plan    # Preview changes
make deploy  # Apply changes
make undeploy # Destroy resources
```

### Configuration

Edit `terraform/config.yaml` to customize:
- `gcp.project_id` - Your GCP project ID
- `gcp.location` - Region (default: europe-west1)
- `gcp.resources.cloud_run.*` - CPU, memory, scaling
- `parameters.*` - Application environment variables

## Key Design Decisions

1. **HTTP Streamable vs SSE**: Using chunked transfer for better compatibility
2. **Firestore**: Chosen for API key storage due to low latency
3. **In-memory cache**: LRU cache for access tokens and permissions
4. **Rate limiting**: Token bucket algorithm for fairness

## Transport Layer

The `internal/transport/` package provides:

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

## Auth Package

The `internal/auth/` package provides OAuth2 authentication and API key management:

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

## Middleware Package

The `internal/middleware/` package provides API key validation for protected endpoints:

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
// Retrieve values from context
apiKey := middleware.GetAPIKey(ctx)
refreshToken := middleware.GetRefreshToken(ctx)
userEmail := middleware.GetUserEmail(ctx)
tokenSource := middleware.GetTokenSource(ctx)
```

### Configuration
```go
middleware.APIKeyMiddlewareConfig{
    Store:             store,                // APIKeyStoreInterface implementation
    OAuthClientID:     "client-id",          // For token refresh
    OAuthClientSecret: "client-secret",      // For token refresh
    CacheTTL:          5 * time.Minute,      // Token cache TTL
    UpdateLastUsed:    true,                 // Update last_used timestamp
    Logger:            slog.Default(),       // Logger instance
}
```

### Integration with Server
```go
// Create middleware
apiKeyMiddleware := middleware.NewAPIKeyMiddleware(config)

// Attach to server
server.SetAPIKeyMiddleware(apiKeyMiddleware)
```

### Error Responses
- `401 Unauthorized` - Missing/invalid Authorization header
- `401 Unauthorized` - API key not found in store
- `500 Internal Server Error` - Store lookup failure

### Cache Management
```go
// Invalidate specific key (e.g., after user logout)
apiKeyMiddleware.InvalidateCache(apiKey)

// Clear entire cache
apiKeyMiddleware.ClearCache()

// Check cache size
size := apiKeyMiddleware.CacheSize()
```

## Permissions Package

The `internal/permissions/` package verifies user permissions before modifying presentations:

### Permission Checker (`checker.go`)
- Calls Drive API to check file permissions via `file.capabilities.canEdit`
- Returns clear error messages when user lacks permission
- Caches permission checks with configurable TTL (default 5 minutes)
- Interface-based design for easy testing with `DriveService` interface

### Permission Levels
```go
permissions.PermissionNone   // No access
permissions.PermissionRead   // Read-only (viewer, commenter)
permissions.PermissionWrite  // Write access (writer, owner)
```

### Sentinel Errors
```go
permissions.ErrNoWritePermission  // "user does not have write permission on this presentation"
permissions.ErrNoReadPermission   // "user does not have read permission on this presentation"
permissions.ErrPermissionCheck    // "failed to check permissions"
permissions.ErrFileNotFound       // "presentation not found"
```

### Usage Pattern
```go
// Create checker
checker := permissions.NewChecker(permissions.DefaultCheckerConfig(), nil)

// Get token source from middleware context
tokenSource := middleware.GetTokenSource(ctx)
userEmail := middleware.GetUserEmail(ctx)

// Check write permission before modifications
if err := checker.CheckWrite(ctx, tokenSource, userEmail, presentationID); err != nil {
    if errors.Is(err, permissions.ErrNoWritePermission) {
        // Return 403 Forbidden with clear error message
    }
    // Handle other errors
}

// Read operations only need read permission
if err := checker.CheckRead(ctx, tokenSource, userEmail, presentationID); err != nil {
    // Handle permission error
}
```

### Configuration
```go
permissions.CheckerConfig{
    CacheTTL: 5 * time.Minute,    // Permission cache TTL
    Logger:   slog.Default(),     // Logger instance
}
```

### Cache Management
```go
// Invalidate cache for user/file combination
checker.InvalidateCache(userEmail, presentationID)

// Invalidate all cached permissions for a file
checker.InvalidateCacheForFile(presentationID)

// Clear entire cache
checker.ClearCache()

// Check cache size
size := checker.CacheSize()
```

### Integration with Tools
Tools that modify presentations should check write permission:
```go
func (t *Tools) ModifySlide(ctx context.Context, input ModifySlideInput) (*ModifySlideOutput, error) {
    tokenSource := middleware.GetTokenSource(ctx)
    userEmail := middleware.GetUserEmail(ctx)

    // Check write permission before modification
    if err := t.permissionChecker.CheckWrite(ctx, tokenSource, userEmail, input.PresentationID); err != nil {
        return nil, err
    }

    // Proceed with modification
    // ...
}
```

## Rate Limiting Package

The `internal/ratelimit/` package provides global rate limiting with per-endpoint support:

### Token Bucket Algorithm (`limiter.go`)
- Token bucket rate limiter for fair request distribution
- Configurable requests per second and burst size
- Per-endpoint rate limits override global defaults
- Automatic token refill based on elapsed time

### Rate Limit Headers
All responses include standard rate limit headers:
- `X-RateLimit-Limit` - Maximum requests allowed (burst size)
- `X-RateLimit-Remaining` - Remaining requests in current window
- `X-RateLimit-Reset` - Unix timestamp when limit resets

### 429 Response
When rate limit is exceeded:
- Returns `429 Too Many Requests` status
- Includes `Retry-After` header (seconds until next request allowed)
- JSON body: `{"error": "rate limit exceeded", "retry_after": N}`

### Configuration
```go
ratelimit.Config{
    RequestsPerSecond: 10.0,   // Tokens added per second
    BurstSize:         20,     // Maximum tokens (burst capacity)
    Logger:            slog.Default(),
}
```

### Usage Pattern
```go
// Create rate limiter
limiter := ratelimit.New(ratelimit.Config{
    RequestsPerSecond: 10.0,
    BurstSize:         20,
})

// Set per-endpoint limits
limiter.SetEndpointLimit("/api/heavy", 2.0, 5)   // 2 req/s, burst 5
limiter.SetEndpointLimit("/api/fast", 100.0, 50) // 100 req/s, burst 50

// Integrate with server
server.SetRateLimitMiddleware(limiter)
```

### Removing Endpoint Limits
```go
// Remove endpoint-specific limit (falls back to global)
limiter.RemoveEndpointLimit("/api/heavy")

// Get all configured endpoint limits
limits := limiter.GetEndpointLimits()
```

### Metrics
```go
// Check remaining tokens
remaining := limiter.GlobalRemaining()
limit := limiter.GlobalLimit()
rate := limiter.GlobalRate()
```

## Tools Package

The `internal/tools/` package implements MCP tools for Google Slides operations:

### Architecture
- Interface-based design (`SlidesService`) for easy mocking in tests
- `SlidesServiceFactory` pattern for creating services from token sources
- Tools receive `oauth2.TokenSource` from middleware context

### get_presentation Tool (`get_presentation.go`)
Loads a Google Slides presentation and returns its full structured content.

**Input:**
```go
tools.GetPresentationInput{
    PresentationID:    "presentation-id",  // Required
    IncludeThumbnails: true,               // Optional, default false
}
```

**Output:**
```go
tools.GetPresentationOutput{
    PresentationID: "presentation-id",
    Title:          "Presentation Title",
    Locale:         "en_US",
    SlidesCount:    10,
    PageSize:       &PageSize{Width: {720, "PT"}, Height: {405, "PT"}},
    Slides:         []SlideInfo{...},
    Masters:        []MasterInfo{...},
    Layouts:        []LayoutInfo{...},
}
```

**SlideInfo Structure:**
- `Index` - 1-based slide index
- `ObjectID` - Unique slide identifier
- `LayoutID`, `LayoutName` - Layout information
- `TextContent` - Array of `{object_id, object_type, text}`
- `SpeakerNotes` - Notes content from speaker notes page
- `ObjectCount` - Number of page elements
- `Objects` - Array of `{object_id, object_type}`
- `ThumbnailBase64` - Base64 encoded thumbnail (if requested)

**Sentinel Errors:**
```go
tools.ErrPresentationNotFound  // 404 - presentation does not exist
tools.ErrAccessDenied          // 403 - no permission to access
tools.ErrSlidesAPIError        // Other Slides API errors
```

**Usage Pattern:**
```go
// Create tools instance
tools := tools.NewTools(tools.DefaultToolsConfig(), nil)

// Get token source from middleware context
tokenSource := middleware.GetTokenSource(ctx)

// Call the tool
output, err := tools.GetPresentation(ctx, tokenSource, tools.GetPresentationInput{
    PresentationID:    "abc123",
    IncludeThumbnails: true,
})
```

### Text Content Extraction
- Extracts text from shapes (TEXT_BOX, RECTANGLE, etc.)
- Extracts text from tables with cell positions `[row,col]: content`
- Recursively extracts from grouped elements
- Trims whitespace from extracted text

### Object Type Detection
Supported object types:
- `TEXT_BOX`, `RECTANGLE`, `ELLIPSE`, etc. (shapes)
- `IMAGE`, `VIDEO`, `TABLE`, `LINE`
- `GROUP`, `SHEETS_CHART`, `WORD_ART`

### Speaker Notes Extraction
- Looks for BODY placeholder in notes page
- Falls back to any shape with text if no BODY placeholder
- Returns empty string if no notes exist

### Thumbnail Fetching
- Uses Slides API `GetThumbnail` with LARGE size
- Fetches image data via HTTP and encodes as base64
- Gracefully handles fetch failures (logs warning, continues)

## Testing Locally

1. Set up OAuth2 credentials in Secret Manager
2. Run `make run`
3. Visit `http://localhost:8080/auth` to authenticate
4. Use returned API key for tool calls

## Common Tasks

### Adding a New Tool

1. Create input/output structs in `internal/tools/`
2. Implement handler method
3. Register in tool registry
4. Add tests
5. Document in README.md

### Modifying Infrastructure

1. Edit files in `terraform/`
2. Run `make plan` to preview changes
3. Run `make deploy` to apply
4. Verify in GCP Console

### Adding New Terraform Resources

Follow the feature-based pattern:
1. Create new `<feature>.tf` file
2. Structure: locals -> resources -> permissions -> outputs
3. Use `local.resource_prefix` for naming
4. Apply `local.common_labels` for tracking
5. Run `terraform validate` before committing

## Docker

### Dockerfile Architecture

Multi-stage build for minimal image size and security:

1. **Builder stage** (`golang:1.21-alpine`):
   - Installs ca-certificates and git for module downloads
   - Copies and downloads Go dependencies
   - Builds static binary with CGO_ENABLED=0
   - Supports build args: VERSION, COMMIT_SHA, BUILD_TIME

2. **Runtime stage** (`gcr.io/distroless/static-debian12:nonroot`):
   - Distroless image for minimal attack surface
   - Runs as non-root user (UID 65532)
   - Contains only the binary and CA certificates

### Build Arguments

```bash
docker build \
  --build-arg VERSION=1.0.0 \
  --build-arg COMMIT_SHA=$(git rev-parse HEAD) \
  --build-arg BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  -t google-slides-mcp .
```

### Cloud Build

`cloudbuild.yaml` defines the CI/CD pipeline:

1. **test**: Run `go test -race` with coverage
2. **build**: Build Docker image with version tags
3. **push**: Push to Artifact Registry
4. **deploy**: Deploy to Cloud Run

Substitutions:
- `_REGION`: GCP region (default: europe-west1)
- `_SERVICE_NAME`: Cloud Run service name (default: google-slides-mcp)

Manual trigger:
```bash
gcloud builds submit --config=cloudbuild.yaml
```
