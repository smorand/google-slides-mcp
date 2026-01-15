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
- `SlidesService.BatchUpdate` used for modification operations (add/delete/reorder slides)

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

### search_presentations Tool (`search_presentations.go`)
Searches for Google Slides presentations in Google Drive.

**Input:**
```go
tools.SearchPresentationsInput{
    Query:      "quarterly report",  // Required - search term
    MaxResults: 10,                  // Optional, default 10, max 100
}
```

**Output:**
```go
tools.SearchPresentationsOutput{
    Presentations: []PresentationResult{...},
    TotalResults:  5,
    Query:         "quarterly report",
}

// Each result contains:
tools.PresentationResult{
    ID:           "presentation-id",
    Title:        "Q4 Report 2024",
    Owner:        "user@example.com",
    ModifiedDate: "2024-01-15T10:30:00Z",
    ThumbnailURL: "https://drive.google.com/thumbnail/...",
}
```

**Features:**
- Searches owned, shared, and shared drive presentations
- Only returns Google Slides files (filters by mime type)
- Supports advanced Drive search operators (name contains, modifiedTime, etc.)
- Simple queries are automatically wrapped in `fullText contains`
- Escapes special characters in queries

**Sentinel Errors:**
```go
tools.ErrDriveAPIError  // Drive API errors
tools.ErrInvalidQuery   // Empty or invalid query
tools.ErrAccessDenied   // 403 - no permission
```

**Advanced Query Examples:**
```go
// Simple text search (wrapped in fullText contains)
input := SearchPresentationsInput{Query: "budget report"}

// Search by name
input := SearchPresentationsInput{Query: "name contains 'Q4'"}

// Search by modification date
input := SearchPresentationsInput{Query: "modifiedTime > '2024-01-01'"}

// Combined search
input := SearchPresentationsInput{Query: "name contains 'Report' and modifiedTime > '2024-01-01'"}
```

### copy_presentation Tool (`copy_presentation.go`)
Copies a Google Slides presentation (useful for templates).

**Input:**
```go
tools.CopyPresentationInput{
    SourceID:            "source-presentation-id",  // Required
    NewTitle:            "My New Presentation",     // Required
    DestinationFolderID: "folder-id",               // Optional
}
```

**Output:**
```go
tools.CopyPresentationOutput{
    PresentationID: "new-presentation-id",
    Title:          "My New Presentation",
    URL:            "https://docs.google.com/presentation/d/new-presentation-id/edit",
    SourceID:       "source-presentation-id",
}
```

**Features:**
- Creates a copy of the source presentation using Drive API
- Preserves all formatting, themes, and masters (inherent to Drive copy)
- Optionally places copy in specified destination folder
- Returns direct edit URL for the new presentation

**Sentinel Errors:**
```go
tools.ErrSourceNotFound     // Source presentation not found
tools.ErrCopyFailed         // Generic copy failure
tools.ErrInvalidSourceID    // Empty source ID
tools.ErrInvalidTitle       // Empty title
tools.ErrDestinationInvalid // Destination folder not found
tools.ErrAccessDenied       // No permission to copy
```

**Usage Pattern:**
```go
// Copy a template presentation
output, err := tools.CopyPresentation(ctx, tokenSource, tools.CopyPresentationInput{
    SourceID: "template-id",
    NewTitle: "Q1 2024 Report",
})

// Copy to specific folder
output, err := tools.CopyPresentation(ctx, tokenSource, tools.CopyPresentationInput{
    SourceID:            "template-id",
    NewTitle:            "Q1 2024 Report",
    DestinationFolderID: "reports-folder-id",
})
```

### export_pdf Tool (`export_pdf.go`)
Exports a Google Slides presentation to PDF format.

**Input:**
```go
tools.ExportPDFInput{
    PresentationID: "presentation-id",  // Required
}
```

**Output:**
```go
tools.ExportPDFOutput{
    PDFBase64: "base64-encoded-pdf-content",
    PageCount: 10,      // Number of pages detected in PDF
    FileSize:  123456,  // Size in bytes
}
```

**Features:**
- Uses Drive API's export functionality
- Returns PDF as base64-encoded string for easy transfer
- Detects page count using PDF structure analysis
- Includes file size metadata

**Sentinel Errors:**
```go
tools.ErrExportFailed          // Generic export failure
tools.ErrInvalidPresentationID // Empty presentation ID
tools.ErrPresentationNotFound  // Presentation not found
tools.ErrAccessDenied          // No permission to export
tools.ErrDriveAPIError         // Drive API errors
```

**Usage Pattern:**
```go
// Export presentation to PDF
output, err := tools.ExportPDF(ctx, tokenSource, tools.ExportPDFInput{
    PresentationID: "abc123",
})

// Decode the PDF (example)
pdfData, _ := base64.StdEncoding.DecodeString(output.PDFBase64)
```

### create_presentation Tool (`create_presentation.go`)
Creates a new empty Google Slides presentation.

**Input:**
```go
tools.CreatePresentationInput{
    Title:    "My New Presentation",  // Required
    FolderID: "folder-id",            // Optional - destination folder
}
```

**Output:**
```go
tools.CreatePresentationOutput{
    PresentationID: "new-presentation-id",
    Title:          "My New Presentation",
    URL:            "https://docs.google.com/presentation/d/new-presentation-id/edit",
    FolderID:       "folder-id",  // Only if specified in input
}
```

**Features:**
- Creates presentation via Slides API (Presentations.Create)
- Optionally moves to specified folder via Drive API
- Returns direct edit URL for immediate access

**Sentinel Errors:**
```go
tools.ErrCreateFailed       // Generic creation failure
tools.ErrInvalidCreateTitle // Empty title provided
tools.ErrFolderNotFound     // Destination folder not found
tools.ErrAccessDenied       // No permission to create or access folder
tools.ErrSlidesAPIError     // Slides API errors
tools.ErrDriveAPIError      // Drive API errors (for folder operations)
```

**Usage Pattern:**
```go
// Create presentation in root
output, err := tools.CreatePresentation(ctx, tokenSource, tools.CreatePresentationInput{
    Title: "Q1 2024 Report",
})

// Create presentation in specific folder
output, err := tools.CreatePresentation(ctx, tokenSource, tools.CreatePresentationInput{
    Title:    "Q1 2024 Report",
    FolderID: "reports-folder-id",
})
```

### list_slides Tool (`list_slides.go`)
Lists all slides in a presentation with metadata and summary statistics.

**Input:**
```go
tools.ListSlidesInput{
    PresentationID:    "presentation-id",  // Required
    IncludeThumbnails: true,               // Optional, default false
}
```

**Output:**
```go
tools.ListSlidesOutput{
    PresentationID: "presentation-id",
    Title:          "Presentation Title",
    Slides:         []SlideListItem{...},
    Statistics:     SlidesStatistics{...},
}

// Each slide item contains:
tools.SlideListItem{
    Index:           1,                    // 1-based index
    SlideID:         "slide-object-id",
    Title:           "Slide Title",        // From TITLE placeholder
    LayoutType:      "TITLE_AND_BODY",     // Layout name
    ObjectCount:     5,                    // Number of page elements
    ThumbnailBase64: "...",               // Optional base64 thumbnail
}

// Statistics:
tools.SlidesStatistics{
    TotalSlides:      10,
    SlidesWithNotes:  5,
    SlidesWithVideos: 2,
}
```

**Features:**
- Returns 1-based slide indices for easy human reference
- Extracts slide title from TITLE or CENTERED_TITLE placeholders
- Detects layout type from presentation layouts
- Counts slides with speaker notes
- Counts slides containing video elements (including in groups)
- Optional thumbnail support via base64 encoding

**Sentinel Errors:**
```go
tools.ErrInvalidPresentationID // Empty presentation ID
tools.ErrPresentationNotFound  // 404 - presentation does not exist
tools.ErrAccessDenied          // 403 - no permission to access
tools.ErrSlidesAPIError        // Other Slides API errors
```

**Usage Pattern:**
```go
// List slides without thumbnails
output, err := tools.ListSlides(ctx, tokenSource, tools.ListSlidesInput{
    PresentationID: "abc123",
})

// List slides with thumbnails
output, err := tools.ListSlides(ctx, tokenSource, tools.ListSlidesInput{
    PresentationID:    "abc123",
    IncludeThumbnails: true,
})

// Check statistics
fmt.Printf("Total: %d, With Notes: %d, With Videos: %d\n",
    output.Statistics.TotalSlides,
    output.Statistics.SlidesWithNotes,
    output.Statistics.SlidesWithVideos,
)
```

### describe_slide Tool (`describe_slide.go`)
Gets detailed human-readable description of a specific slide.

**Input:**
```go
tools.DescribeSlideInput{
    PresentationID: "presentation-id",  // Required
    SlideIndex:     1,                  // 1-based index (use this OR SlideID)
    SlideID:        "slide-object-id",  // Alternative to SlideIndex
}
```

**Output:**
```go
tools.DescribeSlideOutput{
    PresentationID:    "presentation-id",
    SlideID:           "slide-object-id",
    SlideIndex:        1,
    Title:             "Slide Title",
    LayoutType:        "TITLE_AND_BODY",
    PageSize:          &PageSize{...},
    Objects:           []ObjectDescription{...},
    LayoutDescription: "Title at top: \"My Title\". 2 element(s) in center...",
    ScreenshotBase64:  "base64-encoded-png",
    SpeakerNotes:      "Notes for this slide",
}

// Each object description contains:
tools.ObjectDescription{
    ObjectID:       "shape-123",
    ObjectType:     "TEXT_BOX",
    Position:       &Position{X: 100, Y: 50},    // In points
    Size:           &Size{Width: 300, Height: 100}, // In points
    ContentSummary: "First 100 characters of text...",
    ZOrder:         0,                            // Lower = further back
    Children:       []ObjectDescription{...},    // For groups
}
```

**Features:**
- Accepts either 1-based slide index OR slide ID for flexibility
- Returns position and size in points (converted from EMU)
- Content summary truncated to 100 characters for readability
- Generates human-readable layout description (e.g., "title at top center, image on left")
- Includes slide screenshot as base64 PNG
- Recursively describes grouped elements with children array

**Sentinel Errors:**
```go
tools.ErrInvalidPresentationID  // Empty presentation ID
tools.ErrInvalidSlideReference  // Neither slide_index nor slide_id provided
tools.ErrSlideNotFound          // Slide index out of range or ID not found
tools.ErrPresentationNotFound   // Presentation not found
tools.ErrAccessDenied           // No permission to access
tools.ErrSlidesAPIError         // Other Slides API errors
```

**Usage Pattern:**
```go
// Describe slide by index
output, err := tools.DescribeSlide(ctx, tokenSource, tools.DescribeSlideInput{
    PresentationID: "abc123",
    SlideIndex:     1,
})

// Describe slide by ID
output, err := tools.DescribeSlide(ctx, tokenSource, tools.DescribeSlideInput{
    PresentationID: "abc123",
    SlideID:        "g123456",
})

// Use the layout description for LLM context
fmt.Println(output.LayoutDescription)
// Output: "Title at top: \"Introduction\". 2 element(s) in center. Contains: 1 image, 1 text_box"
```

**Unit Conversion:**
- Positions are stored internally in EMU (English Metric Units)
- 1 point = 12700 EMU
- Standard slide size: 720 x 405 points

### add_slide Tool (`add_slide.go`)
Adds a new slide to a presentation.

**Input:**
```go
tools.AddSlideInput{
    PresentationID: "presentation-id",  // Required
    Position:       1,                  // 1-based position (0 or omitted = end)
    Layout:         "TITLE_AND_BODY",   // Required - layout type
}
```

**Output:**
```go
tools.AddSlideOutput{
    SlideIndex: 3,              // 1-based index of the new slide
    SlideID:    "slide-id",     // Object ID of the new slide
}
```

**Supported Layout Types:**
- `BLANK` - Empty slide
- `CAPTION_ONLY` - Caption only
- `TITLE` - Title slide
- `TITLE_AND_BODY` - Title with body text
- `TITLE_AND_TWO_COLUMNS` - Title with two columns
- `TITLE_ONLY` - Title only
- `ONE_COLUMN_TEXT` - Single column text
- `MAIN_POINT` - Main point layout
- `BIG_NUMBER` - Big number layout
- `SECTION_HEADER` - Section header
- `SECTION_TITLE_AND_DESCRIPTION` - Section title with description

**Features:**
- Position 0 or omitted inserts at end of presentation
- Position beyond slide count inserts at end
- Finds matching layout in presentation's layouts first
- Falls back to first available layout if exact match not found
- Falls back to predefined layout type if no layouts exist

**Sentinel Errors:**
```go
tools.ErrAddSlideFailed         // Generic slide creation failure
tools.ErrInvalidLayout          // Empty or unsupported layout type
tools.ErrInvalidPosition        // Invalid position (reserved for future validation)
tools.ErrInvalidPresentationID  // Empty presentation ID
tools.ErrPresentationNotFound   // Presentation not found
tools.ErrAccessDenied           // No permission to modify
tools.ErrSlidesAPIError         // Other Slides API errors
```

**Usage Pattern:**
```go
// Add slide at end with BLANK layout
output, err := tools.AddSlide(ctx, tokenSource, tools.AddSlideInput{
    PresentationID: "abc123",
    Layout:         "BLANK",
})

// Add slide at specific position
output, err := tools.AddSlide(ctx, tokenSource, tools.AddSlideInput{
    PresentationID: "abc123",
    Position:       2,  // Insert at position 2 (becomes second slide)
    Layout:         "TITLE_AND_BODY",
})

fmt.Printf("New slide: index=%d, id=%s\n", output.SlideIndex, output.SlideID)
```

### delete_slide Tool (`delete_slide.go`)
Deletes a slide from a presentation.

**Input:**
```go
tools.DeleteSlideInput{
    PresentationID: "presentation-id",  // Required
    SlideIndex:     2,                  // 1-based index (use this OR SlideID)
    SlideID:        "slide-object-id",  // Alternative to SlideIndex
}
```

**Output:**
```go
tools.DeleteSlideOutput{
    DeletedSlideID:     "slide-object-id",  // Object ID of the deleted slide
    RemainingSlideCount: 2,                  // Number of slides after deletion
}
```

**Features:**
- Accepts either 1-based slide index OR slide ID for identification
- If both provided, SlideID takes precedence
- Prevents deletion of last remaining slide (returns ErrLastSlideDelete)
- Returns updated slide count after deletion

**Sentinel Errors:**
```go
tools.ErrDeleteSlideFailed      // Generic deletion failure
tools.ErrLastSlideDelete        // Cannot delete the last remaining slide
tools.ErrInvalidSlideReference  // Neither slide_index nor slide_id provided
tools.ErrSlideNotFound          // Slide index out of range or ID not found
tools.ErrInvalidPresentationID  // Empty presentation ID
tools.ErrPresentationNotFound   // Presentation not found
tools.ErrAccessDenied           // No permission to modify
tools.ErrSlidesAPIError         // Other Slides API errors
```

**Usage Pattern:**
```go
// Delete slide by index
output, err := tools.DeleteSlide(ctx, tokenSource, tools.DeleteSlideInput{
    PresentationID: "abc123",
    SlideIndex:     2,  // Delete second slide
})

// Delete slide by ID
output, err := tools.DeleteSlide(ctx, tokenSource, tools.DeleteSlideInput{
    PresentationID: "abc123",
    SlideID:        "g123456",
})

fmt.Printf("Deleted: %s, Remaining: %d\n", output.DeletedSlideID, output.RemainingSlideCount)
```

### reorder_slides Tool (`reorder_slides.go`)
Moves slides to new positions within a presentation.

**Input:**
```go
tools.ReorderSlidesInput{
    PresentationID: "presentation-id",  // Required
    SlideIndices:   []int{2, 4},        // 1-based indices (use this OR SlideIDs)
    SlideIDs:       []string{"id1"},    // Alternative to SlideIndices
    InsertAt:       1,                  // 1-based position to move slides to
}
```

**Output:**
```go
tools.ReorderSlidesOutput{
    NewOrder: []SlidePosition{
        {Index: 1, SlideID: "slide-moved"},
        {Index: 2, SlideID: "slide-original-1"},
        // ... all slides in new order
    },
}
```

**Features:**
- Accepts either 1-based slide indices OR slide IDs for identification
- If both provided, SlideIDs takes precedence
- Moves multiple slides together maintaining their relative order
- insert_at beyond slide count is clamped to end
- Returns complete new slide order after reordering

**Sentinel Errors:**
```go
tools.ErrReorderSlidesFailed    // Generic reorder failure
tools.ErrNoSlidesToMove         // Neither slide_indices nor slide_ids provided
tools.ErrInvalidInsertAt        // insert_at must be at least 1
tools.ErrSlideNotFound          // Slide index out of range or ID not found
tools.ErrInvalidPresentationID  // Empty presentation ID
tools.ErrPresentationNotFound   // Presentation not found
tools.ErrAccessDenied           // No permission to modify
tools.ErrSlidesAPIError         // Other Slides API errors
```

**Usage Pattern:**
```go
// Move single slide by index
output, err := tools.ReorderSlides(ctx, tokenSource, tools.ReorderSlidesInput{
    PresentationID: "abc123",
    SlideIndices:   []int{3},  // Move third slide
    InsertAt:       1,         // To first position
})

// Move multiple slides by ID
output, err := tools.ReorderSlides(ctx, tokenSource, tools.ReorderSlidesInput{
    PresentationID: "abc123",
    SlideIDs:       []string{"slide-x", "slide-y"},
    InsertAt:       5,  // Move to position 5
})

// Print new order
for _, pos := range output.NewOrder {
    fmt.Printf("Position %d: %s\n", pos.Index, pos.SlideID)
}
```

### duplicate_slide Tool (`duplicate_slide.go`)
Duplicates an existing slide in a presentation.

**Input:**
```go
tools.DuplicateSlideInput{
    PresentationID: "presentation-id",  // Required
    SlideIndex:     2,                  // 1-based index (use this OR SlideID)
    SlideID:        "slide-object-id",  // Alternative to SlideIndex
    InsertAt:       3,                  // 1-based position (0 or omitted = after source slide)
}
```

**Output:**
```go
tools.DuplicateSlideOutput{
    SlideIndex: 3,              // 1-based index of the new duplicated slide
    SlideID:    "new-slide-id", // Object ID of the new duplicated slide
}
```

**Features:**
- Accepts either 1-based slide index OR slide ID for identification
- If both provided, SlideID takes precedence
- Default InsertAt (0 or omitted) places duplicate immediately after source slide
- InsertAt beyond slide count is clamped to end
- Uses `DuplicateObjectRequest` in BatchUpdate for duplication
- If move fails, returns slide in default position with warning logged

**Sentinel Errors:**
```go
tools.ErrDuplicateSlideFailed   // Generic duplication failure
tools.ErrInvalidSlideReference  // Neither slide_index nor slide_id provided
tools.ErrSlideNotFound          // Slide index out of range or ID not found
tools.ErrInvalidPresentationID  // Empty presentation ID
tools.ErrPresentationNotFound   // Presentation not found
tools.ErrAccessDenied           // No permission to modify
tools.ErrSlidesAPIError         // Other Slides API errors
```

**Usage Pattern:**
```go
// Duplicate slide by index (default position = after source)
output, err := tools.DuplicateSlide(ctx, tokenSource, tools.DuplicateSlideInput{
    PresentationID: "abc123",
    SlideIndex:     2,
})

// Duplicate slide by ID to specific position
output, err := tools.DuplicateSlide(ctx, tokenSource, tools.DuplicateSlideInput{
    PresentationID: "abc123",
    SlideID:        "g123456",
    InsertAt:       1,  // Move to first position
})

fmt.Printf("New slide: index=%d, id=%s\n", output.SlideIndex, output.SlideID)
```

### list_objects Tool (`list_objects.go`)
Lists all objects on slides with optional filtering.

**Input:**
```go
tools.ListObjectsInput{
    PresentationID: "presentation-id",           // Required
    SlideIndices:   []int{1, 3},                 // Optional - 1-based indices, default all slides
    ObjectTypes:    []string{"IMAGE", "VIDEO"},  // Optional - filter by type
}
```

**Output:**
```go
tools.ListObjectsOutput{
    PresentationID: "presentation-id",
    Objects:        []ObjectListing{...},
    TotalCount:     15,
    FilteredBy:     &FilterInfo{...},  // Present only if filters applied
}

// Each object listing contains:
tools.ObjectListing{
    SlideIndex:     1,                  // 1-based slide index
    ObjectID:       "shape-123",
    ObjectType:     "TEXT_BOX",
    Position:       &Position{X: 100, Y: 50},
    Size:           &Size{Width: 300, Height: 100},
    ZOrder:         0,                  // Lower = further back
    ContentPreview: "First 100 chars...",  // For text objects
}
```

**Supported Object Types:**
- `TEXT_BOX`, `RECTANGLE`, `ELLIPSE`, etc. (shapes)
- `IMAGE`, `VIDEO`, `TABLE`, `LINE`
- `GROUP`, `SHEETS_CHART`, `WORD_ART`

**Features:**
- Lists objects from all slides when no filters applied
- `slide_indices` filter limits to specific slides (1-based)
- `object_types` filter limits to specific object types
- Position and size returned in points (converted from EMU)
- Content preview for text objects (first 100 characters)
- Z-order indicates layering position (lower = further back)
- Recursively includes objects within groups

**Sentinel Errors:**
```go
tools.ErrInvalidPresentationID  // Empty presentation ID
tools.ErrPresentationNotFound   // Presentation not found
tools.ErrAccessDenied           // No permission to access
tools.ErrSlidesAPIError         // Other Slides API errors
```

**Usage Pattern:**
```go
// List all objects
output, err := tools.ListObjects(ctx, tokenSource, tools.ListObjectsInput{
    PresentationID: "abc123",
})

// List only images and videos from slides 1 and 3
output, err := tools.ListObjects(ctx, tokenSource, tools.ListObjectsInput{
    PresentationID: "abc123",
    SlideIndices:   []int{1, 3},
    ObjectTypes:    []string{"IMAGE", "VIDEO"},
})

// Process results
for _, obj := range output.Objects {
    fmt.Printf("Slide %d: %s (%s) at (%f, %f)\n",
        obj.SlideIndex, obj.ObjectID, obj.ObjectType,
        obj.Position.X, obj.Position.Y)
}
```

### get_object Tool (`get_object.go`)
Gets detailed information about a specific object by ID.

**Input:**
```go
tools.GetObjectInput{
    PresentationID: "presentation-id",  // Required
    ObjectID:       "object-id",        // Required
}
```

**Output:**
```go
tools.GetObjectOutput{
    PresentationID: "presentation-id",
    ObjectID:       "object-id",
    ObjectType:     "TEXT_BOX",         // Shape type, IMAGE, VIDEO, TABLE, LINE, GROUP, SHEETS_CHART, WORD_ART
    SlideIndex:     1,                  // 1-based index of containing slide
    Position:       &Position{X: 100, Y: 50},
    Size:           &Size{Width: 300, Height: 100},
    Shape:          *ShapeDetails{...},  // Set for shapes
    Image:          *ImageDetails{...},  // Set for images
    Table:          *TableDetails{...},  // Set for tables
    Video:          *VideoDetails{...},  // Set for videos
    Line:           *LineDetails{...},   // Set for lines
    Group:          *GroupDetails{...},  // Set for groups
    Chart:          *ChartDetails{...},  // Set for Sheets charts
    WordArt:        *WordArtDetails{...},// Set for word art
}
```

**Type-Specific Details:**

For **shapes**:
```go
tools.ShapeDetails{
    ShapeType:       "TEXT_BOX",
    Text:            "Content text",
    TextStyle:       &TextStyleDetails{FontFamily: "Arial", FontSize: 24, Bold: true, Color: "#FF0000"},
    Fill:            &FillDetails{Type: "SOLID", SolidColor: "#007FFF"},
    Outline:         &OutlineDetails{Color: "#000000", Weight: 2.0, DashStyle: "SOLID"},
    PlaceholderType: "TITLE",  // For placeholder shapes
}
```

For **images**:
```go
tools.ImageDetails{
    ContentURL:   "https://...",
    SourceURL:    "https://...",
    Brightness:   0.5,
    Contrast:     0.3,
    Transparency: 0.1,
    Recolor:      "GRAYSCALE",
    Crop:         &CropDetails{Top: 0.1, Bottom: 0.2, Left: 0.05, Right: 0.15},
}
```

For **tables**:
```go
tools.TableDetails{
    Rows:    3,
    Columns: 4,
    Cells:   [][]CellDetails{...},  // 2D array of cell details with text, spans, background
}
```

For **videos**:
```go
tools.VideoDetails{
    VideoID:   "dQw4w9WgXcQ",
    Source:    "YOUTUBE",  // Or "DRIVE"
    URL:       "https://...",
    StartTime: 30.0,   // seconds
    EndTime:   60.0,   // seconds
    Autoplay:  true,
    Mute:      false,
}
```

For **lines**:
```go
tools.LineDetails{
    LineType:   "STRAIGHT_CONNECTOR_1",
    StartArrow: "ARROW",
    EndArrow:   "NONE",
    Color:      "#0000FF",
    Weight:     3.0,
    DashStyle:  "DASH",
}
```

For **groups**:
```go
tools.GroupDetails{
    ChildCount: 3,
    ChildIDs:   []string{"child-1", "child-2", "child-3"},
}
```

**Features:**
- Returns complete object properties based on object type
- Finds objects anywhere in the presentation (including nested in groups)
- Position and size returned in points (converted from EMU)
- Colors returned as hex strings (#RRGGBB) or theme references (theme:ACCENT1)
- Video times converted from milliseconds to seconds

**Sentinel Errors:**
```go
tools.ErrObjectNotFound         // Object ID not found in presentation
tools.ErrInvalidPresentationID  // Empty presentation ID
tools.ErrPresentationNotFound   // Presentation not found
tools.ErrAccessDenied           // No permission to access
tools.ErrSlidesAPIError         // Other Slides API errors
```

**Usage Pattern:**
```go
// Get details for any object by ID
output, err := tools.GetObject(ctx, tokenSource, tools.GetObjectInput{
    PresentationID: "abc123",
    ObjectID:       "shape-xyz",
})

// Check object type and access appropriate details
switch output.ObjectType {
case "TEXT_BOX", "RECTANGLE":
    fmt.Printf("Text: %s, Font: %s\n", output.Shape.Text, output.Shape.TextStyle.FontFamily)
case "IMAGE":
    fmt.Printf("Image URL: %s, Brightness: %.1f\n", output.Image.ContentURL, output.Image.Brightness)
case "TABLE":
    fmt.Printf("Table: %dx%d\n", output.Table.Rows, output.Table.Columns)
case "VIDEO":
    fmt.Printf("Video: %s (%s), Start: %.0fs\n", output.Video.VideoID, output.Video.Source, output.Video.StartTime)
}
```

### add_text_box Tool (`add_text_box.go`)
Adds a text box to a slide with optional styling.

**Input:**
```go
tools.AddTextBoxInput{
    PresentationID: "presentation-id",  // Required
    SlideIndex:     1,                  // 1-based index (use this OR SlideID)
    SlideID:        "slide-object-id",  // Alternative to SlideIndex
    Text:           "Hello World",      // Required - text content
    Position:       &PositionInput{X: 100, Y: 50},   // Position in points (default: 0, 0)
    Size:           &SizeInput{Width: 300, Height: 100},  // Required - size in points
    Style:          &TextStyleInput{...},  // Optional styling
}

// Position in points (standard slide is 720x405 points)
tools.PositionInput{
    X: 100,  // Points from left edge
    Y: 50,   // Points from top edge
}

// Size in points
tools.SizeInput{
    Width:  300,  // Width in points
    Height: 100,  // Height in points
}

// Optional text styling
tools.TextStyleInput{
    FontFamily: "Arial",       // Font family name
    FontSize:   24,            // Font size in points
    Bold:       true,          // Bold text
    Italic:     false,         // Italic text
    Color:      "#FF0000",     // Hex color string
}
```

**Output:**
```go
tools.AddTextBoxOutput{
    ObjectID: "textbox_1234567890",  // Unique ID of the created text box
}
```

**Features:**
- Accepts either 1-based slide index OR slide ID for identification
- Position defaults to (0, 0) if not specified
- Size is required with positive width and height
- Styling is optional - only specified style fields are applied
- Uses CreateShapeRequest with TEXT_BOX type
- Uses InsertTextRequest to add text content
- Uses UpdateTextStyleRequest for styling (if provided)
- 1 point = 12700 EMU (English Metric Units)

**Sentinel Errors:**
```go
tools.ErrAddTextBoxFailed       // Generic text box creation failure
tools.ErrInvalidText            // Text content is required (empty text)
tools.ErrInvalidSize            // Size is required with positive width and height
tools.ErrInvalidSlideReference  // Neither slide_index nor slide_id provided
tools.ErrSlideNotFound          // Slide index out of range or ID not found
tools.ErrInvalidPresentationID  // Empty presentation ID
tools.ErrPresentationNotFound   // Presentation not found
tools.ErrAccessDenied           // No permission to modify
tools.ErrSlidesAPIError         // Other Slides API errors
```

**Usage Pattern:**
```go
// Add simple text box by slide index
output, err := tools.AddTextBox(ctx, tokenSource, tools.AddTextBoxInput{
    PresentationID: "abc123",
    SlideIndex:     1,
    Text:           "Hello World",
    Size:           &tools.SizeInput{Width: 200, Height: 50},
})

// Add styled text box by slide ID
output, err := tools.AddTextBox(ctx, tokenSource, tools.AddTextBoxInput{
    PresentationID: "abc123",
    SlideID:        "g123456",
    Text:           "Important Title",
    Position:       &tools.PositionInput{X: 100, Y: 50},
    Size:           &tools.SizeInput{Width: 500, Height: 80},
    Style: &tools.TextStyleInput{
        FontFamily: "Arial",
        FontSize:   36,
        Bold:       true,
        Color:      "#0000FF",
    },
})

fmt.Printf("Created text box: %s\n", output.ObjectID)
```

### modify_text Tool (`modify_text.go`)
Modifies text content in an existing shape.

**Input:**
```go
tools.ModifyTextInput{
    PresentationID: "presentation-id",  // Required
    ObjectID:       "object-id",        // Required - ID of shape containing text
    Action:         "replace",          // Required: "replace", "append", "prepend", or "delete"
    Text:           "New text",         // Required for replace/append/prepend (not for delete)
    StartIndex:     &startIdx,          // Optional - for partial replacement
    EndIndex:       &endIdx,            // Optional - for partial replacement
}
```

**Actions:**
| Action | Description |
|--------|-------------|
| `replace` | Replace all text (or partial if indices provided) |
| `append` | Add text at the end of existing content |
| `prepend` | Add text at the beginning of existing content |
| `delete` | Remove all text from the shape |

**Output:**
```go
tools.ModifyTextOutput{
    ObjectID:    "object-id",     // The modified object's ID
    UpdatedText: "New text",      // The resulting text content
    Action:      "replace",       // The action that was performed
}
```

**Features:**
- Supports four actions: replace, append, prepend, delete
- Partial replacement via optional start_index/end_index
- Works with shapes containing text (TEXT_BOX, RECTANGLE, etc.)
- Validates that target object supports text modification
- Tables must be modified cell by cell (returns specific error)

**Sentinel Errors:**
```go
tools.ErrModifyTextFailed   // Generic modification failure
tools.ErrInvalidAction      // Action must be 'replace', 'append', 'prepend', or 'delete'
tools.ErrInvalidObjectID    // Object ID is required
tools.ErrTextRequired       // Text is required for this action (not delete)
tools.ErrInvalidTextRange   // Invalid start_index/end_index values
tools.ErrNotTextObject      // Object does not contain editable text
tools.ErrObjectNotFound     // Object not found in presentation
tools.ErrInvalidPresentationID  // Empty presentation ID
tools.ErrPresentationNotFound   // Presentation not found
tools.ErrAccessDenied           // No permission to modify
tools.ErrSlidesAPIError         // Other Slides API errors
```

**Usage Pattern:**
```go
// Replace all text in a shape
output, err := tools.ModifyText(ctx, tokenSource, tools.ModifyTextInput{
    PresentationID: "abc123",
    ObjectID:       "shape-xyz",
    Action:         "replace",
    Text:           "New content",
})

// Append text to existing content
output, err := tools.ModifyText(ctx, tokenSource, tools.ModifyTextInput{
    PresentationID: "abc123",
    ObjectID:       "shape-xyz",
    Action:         "append",
    Text:           " - additional text",
})

// Partial replacement (replace characters 5-10)
start := 5
end := 10
output, err := tools.ModifyText(ctx, tokenSource, tools.ModifyTextInput{
    PresentationID: "abc123",
    ObjectID:       "shape-xyz",
    Action:         "replace",
    Text:           "REPLACED",
    StartIndex:     &start,
    EndIndex:       &end,
})

// Delete all text
output, err := tools.ModifyText(ctx, tokenSource, tools.ModifyTextInput{
    PresentationID: "abc123",
    ObjectID:       "shape-xyz",
    Action:         "delete",
})

fmt.Printf("Updated text: %s\n", output.UpdatedText)
```

### style_text Tool (`style_text.go`)
Applies styling to text in a shape.

**Input:**
```go
tools.StyleTextInput{
    PresentationID: "presentation-id",  // Required
    ObjectID:       "object-id",        // Required - ID of shape containing text
    StartIndex:     &startIdx,          // Optional - style from this index
    EndIndex:       &endIdx,            // Optional - style until this index (whole text if omitted)
    Style:          &StyleTextStyleSpec{...},  // Required - style properties to apply
}

// Style specification:
tools.StyleTextStyleSpec{
    FontFamily:      "Arial",         // Optional - font family name
    FontSize:        24,              // Optional - font size in points
    Bold:            &true,           // Optional - bold (use pointer to distinguish false from unset)
    Italic:          &true,           // Optional - italic
    Underline:       &true,           // Optional - underline
    Strikethrough:   &true,           // Optional - strikethrough
    ForegroundColor: "#FF0000",       // Optional - text color (hex string)
    BackgroundColor: "#FFFF00",       // Optional - highlight color (hex string)
    LinkURL:         "https://...",   // Optional - create hyperlink
}
```

**Output:**
```go
tools.StyleTextOutput{
    ObjectID:      "object-id",      // The styled object's ID
    AppliedStyles: []string{...},    // List of style properties applied (e.g., "font_family=Arial", "bold=true")
    TextRange:     "ALL",            // Range type: "ALL" or "FIXED_RANGE (start-end)"
}
```

**Features:**
- Apply multiple style properties in a single call
- Partial styling via start_index/end_index for specific text ranges
- Boolean properties use pointers to distinguish false from unset
- Colors are specified as hex strings (#RRGGBB)
- Link URL creates hyperlinks on the text
- Invalid colors are silently ignored (other styles still apply)

**Sentinel Errors:**
```go
tools.ErrStyleTextFailed        // Generic styling failure
tools.ErrNoStyleProvided        // No style properties provided (empty style object)
tools.ErrInvalidObjectID        // Object ID is required
tools.ErrInvalidTextRange       // Invalid start_index/end_index values
tools.ErrNotTextObject          // Object does not contain text (tables must be styled cell by cell)
tools.ErrObjectNotFound         // Object not found in presentation
tools.ErrInvalidPresentationID  // Empty presentation ID
tools.ErrPresentationNotFound   // Presentation not found
tools.ErrAccessDenied           // No permission to modify
tools.ErrSlidesAPIError         // Other Slides API errors
```

**Usage Pattern:**
```go
// Apply font family to all text
output, err := tools.StyleText(ctx, tokenSource, tools.StyleTextInput{
    PresentationID: "abc123",
    ObjectID:       "shape-xyz",
    Style: &tools.StyleTextStyleSpec{
        FontFamily: "Arial",
    },
})

// Apply multiple styles (bold, italic, color)
bold := true
italic := true
output, err := tools.StyleText(ctx, tokenSource, tools.StyleTextInput{
    PresentationID: "abc123",
    ObjectID:       "shape-xyz",
    Style: &tools.StyleTextStyleSpec{
        Bold:            &bold,
        Italic:          &italic,
        ForegroundColor: "#FF0000",
    },
})

// Apply style to specific text range (characters 0-5)
start := 0
end := 5
output, err := tools.StyleText(ctx, tokenSource, tools.StyleTextInput{
    PresentationID: "abc123",
    ObjectID:       "shape-xyz",
    StartIndex:     &start,
    EndIndex:       &end,
    Style: &tools.StyleTextStyleSpec{
        Bold: &bold,
    },
})

// Create hyperlink
output, err := tools.StyleText(ctx, tokenSource, tools.StyleTextInput{
    PresentationID: "abc123",
    ObjectID:       "shape-xyz",
    Style: &tools.StyleTextStyleSpec{
        LinkURL: "https://example.com",
    },
})

fmt.Printf("Applied styles: %v\n", output.AppliedStyles)
```

### format_paragraph Tool (`format_paragraph.go`)
Sets paragraph formatting options like alignment, spacing, and indentation.

**Input:**
```go
tools.FormatParagraphInput{
    PresentationID: "presentation-id",  // Required
    ObjectID:       "object-id",        // Required - ID of shape containing text
    ParagraphIndex: &paragraphIdx,      // Optional - format specific paragraph (0-based), all if omitted
    Formatting:     &ParagraphFormattingOptions{...},  // Required - formatting options
}

// Formatting options:
tools.ParagraphFormattingOptions{
    Alignment:       "CENTER",      // Optional: START, CENTER, END, JUSTIFIED
    LineSpacing:     &150.0,        // Optional: percentage (100 = normal, 150 = 1.5 lines)
    SpaceAbove:      &12.0,         // Optional: points
    SpaceBelow:      &12.0,         // Optional: points
    IndentFirstLine: &36.0,         // Optional: points
    IndentStart:     &18.0,         // Optional: points (left indent)
    IndentEnd:       &18.0,         // Optional: points (right indent)
}
```

**Output:**
```go
tools.FormatParagraphOutput{
    ObjectID:          "object-id",                    // The formatted object's ID
    AppliedFormatting: []string{"alignment=CENTER"},   // List of formatting applied
    ParagraphScope:    "ALL",                          // "ALL" or "INDEX (N)"
}
```

**Alignment Values:**
| Value | Description |
|-------|-------------|
| `START` | Left-aligned (for LTR languages) |
| `CENTER` | Center-aligned |
| `END` | Right-aligned (for LTR languages) |
| `JUSTIFIED` | Justified text |

**Features:**
- Apply paragraph-level formatting (vs character-level in style_text)
- Format all paragraphs or target specific paragraph by index
- Alignment values are case-insensitive (normalized to uppercase)
- Spacing and indentation use points as unit
- Line spacing is percentage-based (100 = single, 200 = double)

**Sentinel Errors:**
```go
tools.ErrFormatParagraphFailed  // Generic formatting failure
tools.ErrNoFormattingProvided   // No formatting properties provided
tools.ErrInvalidAlignment       // Invalid alignment value
tools.ErrInvalidParagraphIndex  // Paragraph index negative or out of range
tools.ErrNotTextObject          // Object does not contain text
tools.ErrObjectNotFound         // Object not found in presentation
tools.ErrInvalidPresentationID  // Empty presentation ID
tools.ErrPresentationNotFound   // Presentation not found
tools.ErrAccessDenied           // No permission to modify
tools.ErrSlidesAPIError         // Other Slides API errors
```

**Usage Pattern:**
```go
// Center align all paragraphs
output, err := tools.FormatParagraph(ctx, tokenSource, tools.FormatParagraphInput{
    PresentationID: "abc123",
    ObjectID:       "shape-xyz",
    Formatting: &tools.ParagraphFormattingOptions{
        Alignment: "CENTER",
    },
})

// Apply multiple formatting options
lineSpacing := 150.0
spaceAbove := 12.0
output, err := tools.FormatParagraph(ctx, tokenSource, tools.FormatParagraphInput{
    PresentationID: "abc123",
    ObjectID:       "shape-xyz",
    Formatting: &tools.ParagraphFormattingOptions{
        Alignment:   "JUSTIFIED",
        LineSpacing: &lineSpacing,
        SpaceAbove:  &spaceAbove,
    },
})

// Format specific paragraph (0-based index)
paragraphIdx := 1
indentFirst := 36.0
output, err := tools.FormatParagraph(ctx, tokenSource, tools.FormatParagraphInput{
    PresentationID: "abc123",
    ObjectID:       "shape-xyz",
    ParagraphIndex: &paragraphIdx,
    Formatting: &tools.ParagraphFormattingOptions{
        IndentFirstLine: &indentFirst,
    },
})

fmt.Printf("Applied: %v, Scope: %s\n", output.AppliedFormatting, output.ParagraphScope)
```

### create_bullet_list Tool (`create_bullet_list.go`)
Converts text to a bullet list or adds bullets to existing text.

**Input:**
```go
tools.CreateBulletListInput{
    PresentationID:   "presentation-id",  // Required
    ObjectID:         "object-id",        // Required - ID of shape containing text
    ParagraphIndices: []int{0, 2},        // Optional - apply to specific paragraphs (0-based), all if omitted
    BulletStyle:      "DISC",             // Required - bullet style
    BulletColor:      "#FF0000",          // Optional - hex color for bullets
}
```

**Bullet Styles:**
| User-Friendly Name | API Preset |
|--------------------|------------|
| `DISC` | BULLET_DISC_CIRCLE_SQUARE |
| `CIRCLE` | BULLET_DISC_CIRCLE_SQUARE |
| `SQUARE` | BULLET_DISC_CIRCLE_SQUARE |
| `DIAMOND` | BULLET_DIAMOND_CIRCLE_SQUARE |
| `ARROW` | BULLET_ARROW_DIAMOND_DISC |
| `STAR` | BULLET_STAR_CIRCLE_SQUARE |
| `CHECKBOX` | BULLET_CHECKBOX |

Full preset names are also accepted (e.g., `BULLET_DISC_CIRCLE_SQUARE`).

**Output:**
```go
tools.CreateBulletListOutput{
    ObjectID:       "object-id",                  // The modified object's ID
    BulletPreset:   "BULLET_DISC_CIRCLE_SQUARE",  // The actual preset applied
    ParagraphScope: "ALL",                        // "ALL" or "INDICES [0, 2]"
    BulletColor:    "#FF0000",                    // The color applied, if any
}
```

**Features:**
- Apply bullets to all paragraphs or specific paragraphs by index
- Bullet style names are case-insensitive (normalized to uppercase)
- Supports both user-friendly names (DISC, ARROW) and full API preset names
- Bullet color is applied via UpdateTextStyleRequest (colors the bullet glyph)
- Invalid colors are silently ignored (bullets still created without color)

**Sentinel Errors:**
```go
tools.ErrCreateBulletListFailed // Generic bullet list creation failure
tools.ErrInvalidBulletStyle     // Invalid or empty bullet style
tools.ErrInvalidParagraphIndex  // Paragraph index negative or out of range
tools.ErrNotTextObject          // Object does not contain text (tables must be done cell by cell)
tools.ErrObjectNotFound         // Object not found in presentation
tools.ErrInvalidPresentationID  // Empty presentation ID
tools.ErrPresentationNotFound   // Presentation not found
tools.ErrAccessDenied           // No permission to modify
tools.ErrSlidesAPIError         // Other Slides API errors
```

**Usage Pattern:**
```go
// Apply bullets to all paragraphs with DISC style
output, err := tools.CreateBulletList(ctx, tokenSource, tools.CreateBulletListInput{
    PresentationID: "abc123",
    ObjectID:       "shape-xyz",
    BulletStyle:    "DISC",
})

// Apply colored star bullets
output, err := tools.CreateBulletList(ctx, tokenSource, tools.CreateBulletListInput{
    PresentationID: "abc123",
    ObjectID:       "shape-xyz",
    BulletStyle:    "STAR",
    BulletColor:    "#FFD700",  // Gold color
})

// Apply bullets to specific paragraphs only
output, err := tools.CreateBulletList(ctx, tokenSource, tools.CreateBulletListInput{
    PresentationID:   "abc123",
    ObjectID:         "shape-xyz",
    ParagraphIndices: []int{0, 2, 4},  // First, third, and fifth paragraphs
    BulletStyle:      "CHECKBOX",
})

fmt.Printf("Applied: %s, Scope: %s\n", output.BulletPreset, output.ParagraphScope)
```

### create_numbered_list Tool (`create_numbered_list.go`)
Converts text to a numbered list or adds numbering to existing text.

**Input:**
```go
tools.CreateNumberedListInput{
    PresentationID:   "presentation-id",  // Required
    ObjectID:         "object-id",        // Required - ID of shape containing text
    ParagraphIndices: []int{0, 2},        // Optional - apply to specific paragraphs (0-based), all if omitted
    NumberStyle:      "DECIMAL",          // Required - number style
    StartNumber:      1,                  // Optional - starting number (default 1)
}
```

**Number Styles:**
| User-Friendly Name | API Preset |
|--------------------|------------|
| `DECIMAL` | NUMBERED_DECIMAL_ALPHA_ROMAN |
| `ALPHA_UPPER` | NUMBERED_UPPERALPHA_ALPHA_ROMAN |
| `ALPHA_LOWER` | NUMBERED_ALPHA_ALPHA_ROMAN |
| `ROMAN_UPPER` | NUMBERED_UPPERROMAN_UPPERALPHA_DECIMAL |
| `ROMAN_LOWER` | NUMBERED_ROMAN_UPPERALPHA_DECIMAL |

Full preset names are also accepted (e.g., `NUMBERED_DECIMAL_NESTED`, `NUMBERED_DECIMAL_ALPHA_ROMAN_PARENS`).

**Output:**
```go
tools.CreateNumberedListOutput{
    ObjectID:       "object-id",                    // The modified object's ID
    NumberPreset:   "NUMBERED_DECIMAL_ALPHA_ROMAN", // The actual preset applied
    ParagraphScope: "ALL",                          // "ALL" or "INDICES [0, 2]"
    StartNumber:    1,                              // The start number applied
}
```

**Features:**
- Apply numbering to all paragraphs or specific paragraphs by index
- Number style names are case-insensitive (normalized to uppercase)
- Supports both user-friendly names (DECIMAL, ROMAN_UPPER) and full API preset names
- Start number is stored but API creates list starting from 1 (limitation)

**Sentinel Errors:**
```go
tools.ErrCreateNumberedListFailed // Generic numbered list creation failure
tools.ErrInvalidNumberStyle       // Invalid or empty number style
tools.ErrInvalidStartNumber       // Start number less than 1
tools.ErrInvalidParagraphIndex    // Paragraph index negative or out of range
tools.ErrNotTextObject            // Object does not contain text (tables must be done cell by cell)
tools.ErrObjectNotFound           // Object not found in presentation
tools.ErrInvalidPresentationID    // Empty presentation ID
tools.ErrPresentationNotFound     // Presentation not found
tools.ErrAccessDenied             // No permission to modify
tools.ErrSlidesAPIError           // Other Slides API errors
```

**Usage Pattern:**
```go
// Apply decimal numbering to all paragraphs
output, err := tools.CreateNumberedList(ctx, tokenSource, tools.CreateNumberedListInput{
    PresentationID: "abc123",
    ObjectID:       "shape-xyz",
    NumberStyle:    "DECIMAL",
})

// Apply uppercase roman numerals
output, err := tools.CreateNumberedList(ctx, tokenSource, tools.CreateNumberedListInput{
    PresentationID: "abc123",
    ObjectID:       "shape-xyz",
    NumberStyle:    "ROMAN_UPPER",
})

// Apply numbering to specific paragraphs only
output, err := tools.CreateNumberedList(ctx, tokenSource, tools.CreateNumberedListInput{
    PresentationID:   "abc123",
    ObjectID:         "shape-xyz",
    ParagraphIndices: []int{0, 2, 4},  // First, third, and fifth paragraphs
    NumberStyle:      "ALPHA_LOWER",
})

fmt.Printf("Applied: %s, Scope: %s\n", output.NumberPreset, output.ParagraphScope)
```

### search_text Tool (`search_text.go`)
Searches for text across all slides in a presentation.

**Input:**
```go
tools.SearchTextInput{
    PresentationID: "presentation-id",  // Required
    Query:          "search term",      // Required - text to search for
    CaseSensitive:  false,              // Optional, default false
}
```

**Output:**
```go
tools.SearchTextOutput{
    PresentationID: "presentation-id",
    Query:          "search term",
    CaseSensitive:  false,
    TotalMatches:   5,
    Results:        []SearchTextResult{...},
}

// Results grouped by slide:
tools.SearchTextResult{
    SlideIndex: 1,            // 1-based slide index
    SlideID:    "slide-id",
    Matches:    []TextMatch{...},
}

// Each match contains:
tools.TextMatch{
    ObjectID:    "shape-id",      // ID of object containing match
    ObjectType:  "TEXT_BOX",      // Type of object (TEXT_BOX, TABLE_CELL, SPEAKER_NOTES:TEXT_BOX, etc.)
    StartIndex:  10,              // Character position of match in text
    TextContext: "...before MATCH after...",  // Surrounding text (50 chars before/after)
}
```

**Features:**
- Searches across all slides, text shapes, tables, and speaker notes
- Case-insensitive search by default (configurable)
- Includes surrounding context (50 characters before/after) for each match
- Groups results by slide for easy navigation
- Searches recursively through grouped elements
- Table cell matches include position in ObjectID (e.g., "table-1[0,2]")
- Speaker notes matches are prefixed with "SPEAKER_NOTES:" in ObjectType

**Sentinel Errors:**
```go
tools.ErrSearchTextFailed       // Generic search failure
tools.ErrInvalidQuery           // Query is required (empty query)
tools.ErrInvalidPresentationID  // Empty presentation ID
tools.ErrPresentationNotFound   // Presentation not found
tools.ErrAccessDenied           // No permission to access
tools.ErrSlidesAPIError         // Other Slides API errors
```

**Usage Pattern:**
```go
// Case-insensitive search (default)
output, err := tools.SearchText(ctx, tokenSource, tools.SearchTextInput{
    PresentationID: "abc123",
    Query:          "important keyword",
})

// Case-sensitive search
output, err := tools.SearchText(ctx, tokenSource, tools.SearchTextInput{
    PresentationID: "abc123",
    Query:          "Important",
    CaseSensitive:  true,
})

// Process results
fmt.Printf("Found %d matches across %d slides\n", output.TotalMatches, len(output.Results))
for _, slideResult := range output.Results {
    fmt.Printf("Slide %d (%s):\n", slideResult.SlideIndex, slideResult.SlideID)
    for _, match := range slideResult.Matches {
        fmt.Printf("  - %s at index %d: %s\n", match.ObjectID, match.StartIndex, match.TextContext)
    }
}
```

### replace_text Tool (`replace_text.go`)
Finds and replaces text across a presentation.

**Input:**
```go
tools.ReplaceTextInput{
    PresentationID: "presentation-id",  // Required
    Find:           "old text",         // Required - text to find
    ReplaceWith:    "new text",         // Required - replacement text (empty string to delete)
    CaseSensitive:  false,              // Optional, default false
    Scope:          "all",              // Optional: "all" | "slide" | "object" - default "all"
    SlideID:        "slide-id",         // Required when scope is "slide"
    ObjectID:       "object-id",        // Required when scope is "object"
}
```

**Output:**
```go
tools.ReplaceTextOutput{
    PresentationID:   "presentation-id",
    Find:             "old text",
    ReplaceWith:      "new text",
    CaseSensitive:    false,
    Scope:            "all",
    ReplacementCount: 5,                    // Number of replacements made
    AffectedObjects:  []AffectedObject{...}, // List of objects that were modified
}

// Each affected object contains:
tools.AffectedObject{
    SlideIndex: 1,           // 1-based slide index
    SlideID:    "slide-id",
    ObjectID:   "shape-id",
    ObjectType: "TEXT_BOX",
}
```

**Features:**
- Uses Google Slides `ReplaceAllTextRequest` API for efficient bulk replacement
- Scope `all` replaces across entire presentation
- Scope `slide` limits to a specific slide (requires `slide_id`)
- Scope `object` limits to slide containing object (requires `object_id`)
- Case-insensitive by default (configurable)
- Reports affected objects with slide and type information
- Empty `replace_with` effectively deletes matched text

**Sentinel Errors:**
```go
tools.ErrReplaceTextFailed     // Generic replacement failure
tools.ErrInvalidFind           // Find text is empty
tools.ErrInvalidScope          // Invalid scope or missing scope-specific parameter
tools.ErrSlideNotFound         // Slide ID not found (when scope is "slide")
tools.ErrObjectNotFound        // Object ID not found (when scope is "object")
tools.ErrInvalidPresentationID // Empty presentation ID
tools.ErrPresentationNotFound  // Presentation not found
tools.ErrAccessDenied          // No permission to modify
tools.ErrSlidesAPIError        // Other Slides API errors
```

**Usage Pattern:**
```go
// Replace all occurrences in entire presentation
output, err := tools.ReplaceText(ctx, tokenSource, tools.ReplaceTextInput{
    PresentationID: "abc123",
    Find:           "{{name}}",
    ReplaceWith:    "John Doe",
})

// Case-sensitive replacement on specific slide
output, err := tools.ReplaceText(ctx, tokenSource, tools.ReplaceTextInput{
    PresentationID: "abc123",
    Find:           "OLD",
    ReplaceWith:    "NEW",
    CaseSensitive:  true,
    Scope:          "slide",
    SlideID:        "slide-xyz",
})

// Delete text (replace with empty string)
output, err := tools.ReplaceText(ctx, tokenSource, tools.ReplaceTextInput{
    PresentationID: "abc123",
    Find:           "[DRAFT]",
    ReplaceWith:    "",
})

// Process results
fmt.Printf("Replaced %d occurrences in %d objects\n",
    output.ReplacementCount, len(output.AffectedObjects))
for _, obj := range output.AffectedObjects {
    fmt.Printf("  Slide %d: %s (%s)\n", obj.SlideIndex, obj.ObjectID, obj.ObjectType)
}
```

### Drive Service Interface
The tools package uses a `DriveService` interface for Drive API operations:

```go
// Interface for mocking
type DriveService interface {
    ListFiles(ctx context.Context, query string, pageSize int64, fields googleapi.Field) (*drive.FileList, error)
    CopyFile(ctx context.Context, fileID string, file *drive.File) (*drive.File, error)
    ExportFile(ctx context.Context, fileID string, mimeType string) (io.ReadCloser, error)
    MoveFile(ctx context.Context, fileID string, folderID string) error
}

// Factory pattern
type DriveServiceFactory func(ctx context.Context, tokenSource oauth2.TokenSource) (DriveService, error)

// Create tools with Drive support
tools := tools.NewToolsWithDrive(config, slidesFactory, driveFactory)
```

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
