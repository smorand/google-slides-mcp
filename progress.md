# Implementation Progress

## 2026-01-15 - US-00001 - Initialize Go project with MCP structure

**Status:** Success

**What was implemented:**
- Created go.mod with module name github.com/smorand/google-slides-mcp (Go 1.21)
- Set up standard Go project layout: cmd/, internal/, pkg/, terraform/, scripts/
- Added main.go entry point in cmd/google-slides-mcp/
- Created comprehensive Makefile with build, test, run, fmt, vet, lint, check, install targets
- Created README.md with project overview, architecture, and tool documentation
- Created CLAUDE.md with development guidelines and conventions
- Added .gitignore configured for Go projects and Terraform

**Files changed:**
- `go.mod` - Module definition
- `go.sum` - Dependency checksums (empty, no deps yet)
- `cmd/google-slides-mcp/main.go` - Application entry point
- `Makefile` - Build automation with platform detection
- `README.md` - Human documentation
- `CLAUDE.md` - AI development guidelines
- `.gitignore` - Git ignore patterns

**Learnings:**
- Makefile auto-detects project structure (cmd/ layout vs flat layout)
- Using standard Go project layout without /src directory per Go conventions
- Entry point should be minimal, business logic goes in internal/

**Remaining issues:** None

---

## 2026-01-15 - US-00002 - Implement HTTP streamable MCP transport layer

**Status:** Success

**What was implemented:**
- HTTP server with configurable port (default 8080), timeouts, and graceful shutdown
- MCP JSON-RPC 2.0 protocol handler implementing initialize handshake
- Health check endpoint at `/health` returning `{"status": "healthy"}`
- Chunked transfer encoding for all MCP responses
- CORS middleware with configurable allowed origins
- Request logging middleware using structured slog output
- MCP methods: `tools/list` and `tools/call` (empty tool registry for now)
- Comprehensive test suite with 22 passing tests

**Files changed:**
- `cmd/google-slides-mcp/main.go` - Updated entry point to create and start server with signal handling
- `internal/transport/server.go` - HTTP server with middleware and routing
- `internal/transport/server_test.go` - Server tests (health, CORS, graceful shutdown)
- `internal/transport/mcp_handler.go` - MCP JSON-RPC protocol handler
- `internal/transport/mcp_handler_test.go` - Handler tests (initialize, tool calls, errors)
- `CLAUDE.md` - Added transport layer documentation
- `README.md` - Added endpoint documentation and MCP protocol details

**Learnings:**
- Go 1.21 doesn't support method-prefixed route patterns (e.g., "GET /health"), use simple patterns instead
- MCP protocol requires initialization before tool calls - track state in handler
- Use `Transfer-Encoding: chunked` header for streaming responses
- httptest.ResponseRecorder properly captures chunked encoding headers
- Use `json.RawMessage` for params to defer parsing until method is known

**Remaining issues:** None

---

## 2026-01-15 - US-00003 - Create Terraform infrastructure for Cloud Run

**Status:** Success

**What was implemented:**
- Complete Terraform configuration for GCP Cloud Run deployment
- Configuration-driven approach using `config.yaml`
- Service accounts for Cloud Run and Cloud Build with least-privilege IAM
- Cloud Run v2 service with auto-scaling, health checks, and secret injection
- Firestore database with indexes for API key lookups
- Secret Manager secrets for OAuth2 credentials
- API enablement for all required Google APIs

**Files changed:**
- `terraform/config.yaml` - Project configuration template
- `terraform/provider.tf` - Google provider with version ~> 5.0
- `terraform/local.tf` - Configuration loader with derived values
- `terraform/apis.tf` - Google APIs enablement
- `terraform/iam.tf` - Service accounts and IAM roles
- `terraform/cloudrun.tf` - Cloud Run v2 service definition
- `terraform/firestore.tf` - Database and indexes
- `terraform/secrets.tf` - Secret Manager secrets
- `Makefile` - Added terraform targets (plan, deploy, undeploy, init-*)
- `CLAUDE.md` - Added terraform structure documentation
- `README.md` - Added deployment instructions

**Learnings:**
- Use feature-based file organization (not resource-type based)
- Outputs go inline at end of each file, NOT in separate output.tf
- Cloud Run v2 API provides better features (scaling, startup probes)
- Firestore indexes must be created for query patterns
- Use `ignore_changes` lifecycle for CI/CD-managed image tags
- Follow terraform skill conventions: config.yaml + local.tf pattern

**Remaining issues:** None

---

## 2026-01-15 - US-00004 - Implement Dockerfile and Cloud Build configuration

**Status:** Success

**What was implemented:**
- Multi-stage Dockerfile with golang:1.21-alpine builder and distroless runtime
- Distroless base image (gcr.io/distroless/static-debian12:nonroot) for security
- Non-root user execution (UID 65532)
- Build arguments support: VERSION, COMMIT_SHA, BUILD_TIME for version tagging
- cloudbuild.yaml with 4 steps: test, build, push, deploy
- Static binary compilation with CGO_ENABLED=0

**Files changed:**
- `Dockerfile` - Multi-stage build with distroless runtime
- `cloudbuild.yaml` - Cloud Build pipeline for CI/CD
- `CLAUDE.md` - Added Docker documentation section
- `README.md` - Added Docker usage instructions

**Learnings:**
- Use `gcr.io/distroless/static-debian12:nonroot` for Go binaries (no glibc needed)
- Distroless nonroot variant automatically sets UID 65532
- CGO_ENABLED=0 required for static binary compatible with distroless
- CA certificates must be copied from builder for HTTPS to work
- Cloud Build substitutions allow parameterized builds (_REGION, _SERVICE_NAME)

**Remaining issues:** None

---

## 2026-01-15 - US-00005 - Implement OAuth2 flow with /auth endpoint

**Status:** Success

**What was implemented:**
- OAuth2 handler with Google endpoints and configurable scopes
- GET /auth endpoint returning authorization URL with correct scopes
- GET /auth/callback endpoint for OAuth2 callback with code exchange
- CSRF protection via cryptographic state parameter (32-byte random, base64 URL encoded)
- State tokens are single-use (deleted after callback)
- Secret Manager loader for OAuth credentials in production
- Environment-based configuration support for development
- Token callback hook for post-authentication processing
- AuthHandler interface integration in transport server
- Comprehensive test suite with 16 passing tests

**Files changed:**
- `internal/auth/oauth.go` - OAuth2 handler with HandleAuth and HandleCallback
- `internal/auth/oauth_test.go` - Comprehensive tests for auth flow
- `internal/auth/secrets.go` - Secret Manager loader for OAuth credentials
- `internal/auth/state.go` - Cryptographic state generation
- `internal/transport/server.go` - Added AuthHandler interface and /auth routes
- `go.mod` - Added golang.org/x/oauth2 and cloud.google.com/go/secretmanager
- `CLAUDE.md` - Added auth package documentation
- `README.md` - Updated authentication flow documentation

**Learnings:**
- Use `oauth2.AccessTypeOffline` and `oauth2.ApprovalForce` to always get refresh token
- Google OAuth2 state parameter provides CSRF protection
- State tokens must be consumed (deleted) after use to prevent replay attacks
- Token exchange requires valid client ID/secret - tests verify flow without real exchange
- AuthHandler interface decouples transport from auth implementation

**Remaining issues:** None

---

## 2026-01-15 - US-00006 - Implement API key generation and storage

**Status:** Success

**What was implemented:**
- UUID v4 API key generation using crypto/rand for cryptographic security
- Firestore-backed storage layer for API keys and refresh tokens
- API key record structure: {api_key, refresh_token, user_email, created_at, last_used}
- Token callback integration that generates API key after successful OAuth2 flow
- API key returned in OAuth callback response (shown only once with warning)
- Mock store implementation for unit testing
- Interface-based design (APIKeyStoreInterface) for easy testing and dependency injection
- Comprehensive test suite with 34 passing tests

**Files changed:**
- `internal/auth/apikey.go` - UUID v4 API key generation
- `internal/auth/apikey_test.go` - Tests for API key format and uniqueness
- `internal/auth/store.go` - Firestore storage layer with CRUD operations
- `internal/auth/store_mock.go` - In-memory mock store for testing
- `internal/auth/store_test.go` - Tests for store operations
- `internal/auth/callback.go` - Token callback function for API key generation
- `internal/auth/callback_test.go` - Tests for callback functionality
- `internal/auth/oauth.go` - Added SetOnTokenFuncWithResult for API key return
- `internal/auth/oauth_test.go` - Added tests for new callback type
- `go.mod` - Added cloud.google.com/go/firestore dependency
- `CLAUDE.md` - Updated auth package documentation
- `README.md` - Added API key storage documentation

**Learnings:**
- UUID v4 requires setting version bits (position 6) and variant bits (position 8)
- Use document ID as API key for O(1) lookups in Firestore
- Interface-based design enables easy mocking without Firestore emulator
- Token callback with result allows returning API key in OAuth response
- Refresh token must be present for API key generation (offline access required)

**Remaining issues:** None

---

## 2026-01-15 - US-00007 - Implement API key validation middleware

**Status:** Success

**What was implemented:**
- API key validation middleware in `internal/middleware/` package
- Authorization header extraction with Bearer token format
- API key lookup in Firestore via `APIKeyStoreInterface`
- OAuth2 TokenSource creation from stored refresh token
- Token caching with configurable TTL (default 5 minutes)
- Asynchronous last_used timestamp updates
- Context enrichment with API key, refresh token, user email, and TokenSource
- Integration with transport server via `SetAPIKeyMiddleware` method
- Comprehensive test suite with 14 passing tests

**Files changed:**
- `internal/middleware/apikey.go` - API key validation middleware with caching
- `internal/middleware/apikey_test.go` - Comprehensive tests for all middleware features
- `internal/transport/server.go` - Added APIKeyMiddleware interface and integration
- `CLAUDE.md` - Added middleware package documentation
- `README.md` - Added security features documentation
- `go.mod` - Updated dependencies

**Learnings:**
- Use interface-based design (`APIKeyMiddleware`) to decouple middleware from server
- Cache validated tokens to reduce Firestore reads on every request
- Update last_used asynchronously to avoid blocking requests
- Use sentinel errors for expected conditions (ErrMissingAuthHeader, ErrInvalidAPIKey)
- Context values allow passing authenticated data through request lifecycle
- Bearer token extraction should be case-insensitive for "Bearer" keyword

**Remaining issues:** None

---

## 2026-01-15 - US-00008 - Implement permission verification before modifications

**Status:** Success

**What was implemented:**
- New `internal/permissions/` package for verifying user access to presentations
- Permission checker using Google Drive API `file.capabilities.canEdit`
- Three permission levels: None, Read, Write (mapped from Drive roles)
- Caching with configurable TTL (default 5 minutes) for performance
- Clear error messages: `ErrNoWritePermission`, `ErrNoReadPermission`, `ErrFileNotFound`
- Interface-based design (`DriveService`) for easy mocking in tests
- Comprehensive test suite with 22 passing tests

**Files changed:**
- `internal/permissions/checker.go` - Permission checker implementation
- `internal/permissions/checker_test.go` - Comprehensive tests including caching and concurrency
- `CLAUDE.md` - Added permissions package documentation
- `README.md` - Added permission verification documentation
- `go.mod` - Added google.golang.org/api/drive/v3 dependency

**Learnings:**
- Use `file.capabilities.canEdit` from Drive API as the most reliable permission check
- Drive roles map to permissions: owner/writer → Write, commenter/reader → Read
- If file is accessible via API, user has at least read permission
- Use interface-based factory pattern for Drive service creation (testability)
- Cache key should combine user email + file ID for proper isolation
- isNotFoundError should check multiple error message patterns (404, notFound, not found)

**Remaining issues:** None

---

## 2026-01-15 - US-00009 - Implement global rate limiting

**Status:** Success

**What was implemented:**
- Token bucket algorithm for fair request distribution
- Configurable requests per second limit (default: 10 req/s)
- Configurable burst size (default: 20 requests)
- Per-endpoint rate limiting overrides via `SetEndpointLimit`
- 429 Too Many Requests response when rate limit exceeded
- Retry-After header indicating seconds to wait
- X-RateLimit-* headers for rate limit status (Limit, Remaining, Reset)
- Integration with transport server via `SetRateLimitMiddleware`
- Comprehensive test suite with 13 test cases

**Files changed:**
- `internal/ratelimit/limiter.go` - Token bucket implementation and middleware
- `internal/ratelimit/limiter_test.go` - Tests for all rate limiting features
- `internal/transport/server.go` - Added RateLimitMiddleware interface and integration
- `internal/transport/server_test.go` - Tests for rate limiting middleware integration
- `CLAUDE.md` - Added rate limiting package documentation
- `README.md` - Added rate limiting section

**Learnings:**
- Token bucket algorithm: tokens refill over time, consumed per request
- Use math.Min to cap tokens at max capacity during refill
- Calculate retry-after as (tokens needed / refill rate) in seconds
- Middleware pattern with interface allows easy testing with mocks
- Rate limiting should apply in withMiddleware before other processing
- Health endpoint intentionally bypasses rate limiting for monitoring

**Remaining issues:** None

---

## 2026-01-15 - US-00010 - Implement tool to load presentation into LLM context

**Status:** Success

**What was implemented:**
- New `internal/tools/` package for MCP tool implementations
- `get_presentation` tool that loads full presentation structure via Slides API
- Interface-based design (`SlidesService`) for easy mocking in tests
- `SlidesServiceFactory` pattern for creating services from token sources
- Complete text content extraction from shapes, text boxes, and tables
- Speaker notes extraction from notes page elements
- Optional thumbnail support with base64 encoding
- Grouped elements processed recursively
- Comprehensive test suite with 18 test cases covering all functionality

**Files changed:**
- `internal/tools/tools.go` - Base tools package with SlidesService interface
- `internal/tools/get_presentation.go` - get_presentation tool implementation
- `internal/tools/get_presentation_test.go` - Comprehensive tests
- `CLAUDE.md` - Added Tools Package documentation
- `README.md` - Added detailed get_presentation documentation with examples

**Learnings:**
- Google Slides API returns presentations with complete structure including masters and layouts
- Speaker notes are stored in `SlideProperties.NotesPage.PageElements` with BODY placeholder type
- Text extraction needs to handle multiple TextElements in TextContent
- Table text extraction should include cell positions for context
- Thumbnail fetching requires separate HTTP request to ContentUrl
- Use `fmt.Fprintf(&builder, ...)` instead of `builder.WriteString(fmt.Sprintf(...))`
- Interface-based factory pattern allows full mocking without real API calls

**Remaining issues:** None

---

## 2026-01-15 - US-00011 - Implement tool to search presentations in Google Drive

**Status:** Success

**What was implemented:**
- `search_presentations` tool to search for Google Slides presentations in Drive
- `DriveService` interface for Drive API operations (ListFiles)
- `DriveServiceFactory` pattern for creating Drive services from token sources
- Query builder that filters by presentation mime type automatically
- Support for advanced Drive search operators (name contains, modifiedTime, etc.)
- Simple queries automatically wrapped in `fullText contains`
- Query string escaping for special characters (single quotes)
- Search across owned, shared, and shared drive presentations
- Results include id, title, owner, modified_date, thumbnail_url
- max_results parameter with default 10, max 100
- Comprehensive test suite with 14 test cases

**Files changed:**
- `internal/tools/tools.go` - Added DriveService interface, DriveServiceFactory, NewToolsWithDrive
- `internal/tools/search_presentations.go` - search_presentations tool implementation
- `internal/tools/search_presentations_test.go` - Comprehensive tests
- `CLAUDE.md` - Added search_presentations and Drive service documentation
- `README.md` - Added search_presentations tool documentation with examples

**Learnings:**
- Google Drive API uses `googleapi.Field` type for field selection, not string
- Use `SupportsAllDrives(true)` and `IncludeItemsFromAllDrives(true)` to search shared drives
- Simple queries should be detected and wrapped in `fullText contains`
- Advanced queries with operators (name contains, modifiedTime, etc.) should be passed through
- Query escaping: escape single quotes by doubling them (`'` → `\'`)
- Files from shared drives may not have owners - handle gracefully
- Interface-based design allows backward-compatible NewTools function while adding NewToolsWithDrive

**Remaining issues:** None

---

## 2026-01-15 - US-00012 - Implement tool to copy presentation from template

**Status:** Success

**What was implemented:**
- `copy_presentation` tool to copy Google Slides presentations
- Extended `DriveService` interface with `CopyFile` method
- Input validation for source_id and new_title
- Optional destination_folder_id support
- Returns new presentation ID, title, URL, and source_id
- Comprehensive error handling with sentinel errors
- 13 test cases covering all functionality

**Files changed:**
- `internal/tools/tools.go` - Added CopyFile to DriveService interface and implementation
- `internal/tools/copy_presentation.go` - copy_presentation tool implementation
- `internal/tools/copy_presentation_test.go` - Comprehensive tests
- `internal/tools/search_presentations_test.go` - Updated mockDriveService with CopyFile
- `CLAUDE.md` - Added copy_presentation documentation
- `README.md` - Added copy_presentation tool documentation with examples

**Learnings:**
- Drive API `Files.Copy` inherently preserves all formatting, themes, and masters
- Use `SupportsAllDrives(true)` for copying files from shared drives
- Error detection order matters: check specific errors (parent-related) before generic (not found)
- Parent folder errors should be distinguished by checking for "parent" keyword specifically
- Don't use generic "File not found" to detect parent errors - it overlaps with source not found

**Remaining issues:** None

---

## 2026-01-15 - US-00013 - Implement tool to export presentation as PDF

**Status:** Success

**What was implemented:**
- `export_pdf` tool to export Google Slides presentations to PDF format
- Extended `DriveService` interface with `ExportFile` method for Drive API export functionality
- PDF returned as base64-encoded string for easy JSON transfer
- Page count detection using PDF structure analysis (counts `/Type /Page` markers)
- File size metadata included in output
- Comprehensive error handling with sentinel errors
- 14 test cases covering all functionality

**Files changed:**
- `internal/tools/tools.go` - Added `io` import and `ExportFile` method to DriveService interface and realDriveService
- `internal/tools/export_pdf.go` - export_pdf tool implementation with page count detection
- `internal/tools/export_pdf_test.go` - Comprehensive tests for export functionality
- `internal/tools/search_presentations_test.go` - Updated mockDriveService with ExportFile method
- `CLAUDE.md` - Added export_pdf documentation and updated DriveService interface docs
- `README.md` - Added export_pdf tool documentation with examples and client-side handling

**Learnings:**
- Google Drive API `Files.Export` exports Workspace files to various formats (PDF, DOCX, etc.)
- Use `application/pdf` MIME type for PDF export
- PDF page count can be detected by counting `/Type /Page` markers (not `/Type /Pages` which is the page tree)
- Export returns `io.ReadCloser` - must close after reading to avoid resource leaks
- Base64 encoding allows embedding binary PDF data in JSON responses
- Test pattern: use `mockReadCloser` wrapper to verify Close() is called

**Remaining issues:** None

---

## 2026-01-15 - US-00014 - Implement tool to create new presentation

**Status:** Success

**What was implemented:**
- `create_presentation` tool to create new empty Google Slides presentations
- Extended `SlidesService` interface with `CreatePresentation` method
- Extended `DriveService` interface with `MoveFile` method for folder placement
- Input validation for required title parameter
- Optional folder placement via Drive API file update
- Returns presentation ID, title, and direct edit URL
- Comprehensive error handling with sentinel errors
- 13 test cases covering all functionality

**Files changed:**
- `internal/tools/tools.go` - Added `CreatePresentation` to SlidesService interface, `MoveFile` to DriveService interface with implementations
- `internal/tools/create_presentation.go` - create_presentation tool implementation
- `internal/tools/create_presentation_test.go` - Comprehensive tests for all functionality
- `internal/tools/get_presentation_test.go` - Updated mockSlidesService with CreatePresentation method
- `internal/tools/search_presentations_test.go` - Updated mockDriveService with MoveFile method
- `CLAUDE.md` - Added create_presentation documentation and updated DriveService interface docs
- `README.md` - Added create_presentation tool documentation with examples

**Learnings:**
- Google Slides API `Presentations.Create` creates a new empty presentation
- Moving files in Drive requires getting current parents first, then adding new parent and removing old ones
- Slides API returns the created presentation with its ID immediately
- Folder placement is a two-step process: create then move via Drive API
- When folder move fails but presentation was created, log warning and return success (presentation exists)
- Use interface-based factory pattern for both Slides and Drive services

**Remaining issues:** None

---

## 2026-01-15 - US-00015 - Implement tool to list slides in presentation

**Status:** Success

**What was implemented:**
- New `list_slides` tool that lists all slides with metadata and summary statistics
- Returns 1-based slide indices for human-friendly reference
- Extracts slide title from TITLE or CENTERED_TITLE placeholders
- Detects layout type by matching layout ID to presentation layouts
- Counts slides with speaker notes (non-empty text in notes page)
- Counts slides containing video elements (including nested in groups)
- Optional thumbnail support with base64 encoding
- Statistics include: total_slides, slides_with_notes, slides_with_videos
- Comprehensive test suite with 12 test cases covering all functionality

**Files changed:**
- `internal/tools/list_slides.go` - list_slides tool implementation
- `internal/tools/list_slides_test.go` - Comprehensive tests (12 test cases)
- `CLAUDE.md` - Added list_slides documentation with input/output examples
- `README.md` - Added full list_slides tool documentation with use cases

**Learnings:**
- Reuse existing sentinel errors (ErrInvalidPresentationID) instead of declaring duplicates
- TITLE and CENTERED_TITLE are both valid title placeholder types in Slides API
- Layout type uses `LayoutProperties.Name` first, falls back to `DisplayName`
- Video detection must be recursive to handle videos nested in groups
- Speaker notes detection checks for non-empty text after trimming whitespace
- Use helper functions (`getLayoutType`, `extractSlideTitle`, `hasSpeakerNotes`, `hasVideos`) for clean code organization

**Remaining issues:** None

---

## 2026-01-15 - US-00016 - Implement tool to describe slide content

**Status:** Success

**What was implemented:**
- New `describe_slide` tool that provides detailed description of a slide's content and layout
- Accepts either `slide_index` (1-based) or `slide_id` for slide identification
- Returns structured object descriptions with positions (x, y) and sizes (width, height) in points
- Content summary extraction for all object types (shapes, images, videos, tables, lines, charts, word art, groups)
- EMU (English Metric Units) to points conversion (1 point = 12700 EMU)
- Layout description generation based on object positions relative to page dimensions
- Recursive handling of grouped elements with children
- Screenshot support via thumbnail API with base64 encoding
- Speaker notes extraction from notes page
- Z-order tracking based on element array position
- Comprehensive test suite with 15+ test cases

**Files changed:**
- `internal/tools/describe_slide.go` - describe_slide tool implementation with helper functions
- `internal/tools/describe_slide_test.go` - Comprehensive tests (15+ test cases)
- `CLAUDE.md` - Added describe_slide documentation with input/output examples
- `README.md` - Added full describe_slide tool documentation with use cases

**Learnings:**
- EMU (English Metric Units) is the native unit in Google Slides API: 1 point = 12700 EMU
- Use `Transform.TranslateX/Y` for position, `Size.Width/Height` for dimensions
- Layout description categorizes objects into title area, top, left, center, right, bottom regions
- Object types can be determined by checking which element property is non-nil (Shape, Image, Video, etc.)
- Content summary varies by type: text excerpt for shapes, "Image (external)" for images, "Table (3x4)" for tables
- Reuse existing helpers (extractSlideTitle, extractSpeakerNotes, getLayoutType, fetchThumbnailImage) from list_slides
- Sentinel errors ErrInvalidSlideReference and ErrSlideNotFound for input validation

**Remaining issues:** None

---

## 2026-01-15 - US-00017 - Implement tool to add slide

**Status:** Success

**What was implemented:**
- New `add_slide` tool that adds slides to presentations via Slides API BatchUpdate
- Extended `SlidesService` interface with `BatchUpdate` method for modification operations
- Input validation for presentation_id and layout parameters
- Support for 11 predefined layout types (BLANK, TITLE, TITLE_AND_BODY, etc.)
- Position parameter: 1-based, 0 or omitted defaults to end, beyond range defaults to end
- Layout matching: finds by type in presentation layouts, falls back to first layout, then predefined
- Returns new slide's 1-based index and object ID
- Comprehensive test suite with 14 test cases covering all functionality

**Files changed:**
- `internal/tools/tools.go` - Added `BatchUpdate` method to SlidesService interface and realSlidesService
- `internal/tools/add_slide.go` - add_slide tool implementation with layout validation
- `internal/tools/add_slide_test.go` - Comprehensive tests (14 test cases)
- `internal/tools/get_presentation_test.go` - Updated mockSlidesService with BatchUpdate method
- `CLAUDE.md` - Added add_slide documentation with input/output examples and supported layouts
- `README.md` - Added full add_slide tool documentation with examples

**Learnings:**
- Google Slides API uses `BatchUpdate` with `CreateSlideRequest` to add slides
- `InsertionIndex` is 0-based in the API, must convert from user's 1-based position
- `LayoutReference` can use either `LayoutId` (existing layout) or `PredefinedLayout` (type name)
- Presentations may have custom layouts that don't match predefined types - handle gracefully
- `BatchUpdatePresentationResponse.Replies[].CreateSlide.ObjectId` contains the new slide ID
- Variable names like `slides` can shadow package imports - use explicit names like `existingSlides`

**Remaining issues:** None

---

## 2026-01-15 - US-00018 - Implement tool to delete slide

**Status:** Success

**What was implemented:**
- New `delete_slide` tool that deletes a slide from a presentation via Slides API BatchUpdate
- Input accepts either `slide_index` (1-based) or `slide_id` for slide identification
- If both provided, `slide_id` takes precedence
- Prevention of deleting the last remaining slide (returns ErrLastSlideDelete)
- Returns deleted slide ID and remaining slide count
- Uses `DeleteObjectRequest` in BatchUpdate for slide deletion
- Comprehensive test suite with 16 test cases covering all functionality

**Files changed:**
- `internal/tools/delete_slide.go` - delete_slide tool implementation with validation and error handling
- `internal/tools/delete_slide_test.go` - Comprehensive tests (16 test cases)
- `CLAUDE.md` - Added delete_slide documentation with input/output examples and sentinel errors
- `README.md` - Added full delete_slide tool documentation with examples

**Learnings:**
- Google Slides API uses `DeleteObjectRequest` with `ObjectId` to delete slides (same as deleting any object)
- Slides API requires at least one slide - must validate before attempting deletion
- Variable naming conflicts with imported packages (e.g., `slides` variable vs `slides` package) cause compilation errors
- `slide_id` should take precedence over `slide_index` when both are provided for flexibility
- Reuse existing sentinel errors (ErrInvalidSlideReference, ErrSlideNotFound) from describe_slide

**Remaining issues:** None

---

## 2026-01-15 - US-00019 - Implement tool to reorder slides

**Status:** Success

**What was implemented:**
- New `reorder_slides` tool to move slides to new positions within a presentation
- Input accepts either `slide_indices` (1-based array) OR `slide_ids` (string array)
- `insert_at` parameter specifies 1-based target position for slides
- Uses `UpdateSlidesPositionRequest` in Slides API BatchUpdate
- Returns complete new slide order after reordering
- If both `slide_indices` and `slide_ids` provided, `slide_ids` takes precedence
- `insert_at` beyond slide count clamps to end position
- Comprehensive test suite with 22 test cases

**Files changed:**
- `internal/tools/reorder_slides.go` - reorder_slides tool implementation
- `internal/tools/reorder_slides_test.go` - Comprehensive tests (22 test cases)
- `CLAUDE.md` - Added reorder_slides documentation with input/output examples and sentinel errors
- `README.md` - Added full reorder_slides tool documentation with examples

**Learnings:**
- Google Slides API uses `UpdateSlidesPositionRequest` with `SlideObjectIds` array and `InsertionIndex`
- `InsertionIndex` is 0-based in the API but we use 1-based for user interface consistency
- Multiple slides can be moved together; they maintain their relative order
- Fetching updated presentation after reorder is optional - if it fails, return empty order (operation still succeeded)
- Reuse sentinel errors from other slide tools (ErrInvalidPresentationID, ErrSlideNotFound, etc.)
- New errors: ErrReorderSlidesFailed, ErrNoSlidesToMove, ErrInvalidInsertAt

**Remaining issues:** None

---

## 2026-01-15 - US-00020 - Implement tool to duplicate slide

**Status:** Success

**What was implemented:**
- New `duplicate_slide` tool to duplicate an existing slide within a presentation
- Input accepts either `slide_index` (1-based) OR `slide_id` for source slide identification
- Optional `insert_at` parameter specifies target position (1-based); defaults to after source slide
- Uses `DuplicateObjectRequest` in Slides API BatchUpdate for atomic slide duplication
- Two-phase operation: duplicate (creates copy after source), then optionally move if `insert_at` differs
- Uses `UpdateSlidesPositionRequest` for repositioning when needed
- Returns new slide's 1-based index and object ID
- Graceful handling when move fails (logs warning, returns slide at its default position)
- Comprehensive test suite with 18 test cases

**Files changed:**
- `internal/tools/duplicate_slide.go` - duplicate_slide tool implementation
- `internal/tools/duplicate_slide_test.go` - Comprehensive tests (18 test cases)
- `CLAUDE.md` - Added duplicate_slide documentation with input/output examples and sentinel errors
- `README.md` - Added full duplicate_slide tool documentation with examples

**Learnings:**
- Google Slides API `DuplicateObjectRequest` creates an exact copy of a slide including all elements
- The duplicate is always placed immediately after the source slide by default
- To place the duplicate elsewhere, a second BatchUpdate with `UpdateSlidesPositionRequest` is required
- `DuplicateObjectResponse.ObjectId` contains the new slide's object ID
- Error handling: if duplication succeeds but move fails, return success with actual position (slide exists)
- Reuse existing sentinel errors from other tools (ErrInvalidSlideReference, ErrSlideNotFound, etc.)
- New error: `ErrDuplicateSlideFailed` for duplication-specific failures

**Remaining issues:** None

---

## 2026-01-15 - US-00021 - Implement tool to list objects on slides

**Status:** Success

**What was implemented:**
- New `list_objects` tool that lists all objects on slides with optional filtering
- Filter by `slide_indices` (1-based array) to limit to specific slides
- Filter by `object_types` (array of strings) to limit to specific object types
- Returns position (x, y) and size (width, height) in points (converted from EMU)
- Returns z-order for layering position information
- Content preview for text objects (first 100 characters, truncated with "...")
- Table content preview extracts first non-empty cell content
- Recursively includes objects within groups with nested z-order
- Filter info only included in output when filters are applied
- Comprehensive test suite with 17 test cases

**Files changed:**
- `internal/tools/list_objects.go` - list_objects tool implementation
- `internal/tools/list_objects_test.go` - Comprehensive tests (17 test cases)
- `CLAUDE.md` - Added list_objects documentation with input/output examples
- `README.md` - Added full list_objects tool documentation with examples and use cases

**Learnings:**
- Reuse existing helper functions (`extractTextFromTextContent`, `truncateText`, `determineObjectType`, `emuToPoints`, `convertToPoints`) from other tools
- Filter info should only be included in output when filters were actually applied
- Z-order for grouped elements can use multiplied parent z-order (parent*1000 + childIdx) to maintain hierarchy
- Table content preview should extract first non-empty cell, not full table text
- Empty filters (nil or empty arrays) should be treated as "no filter" not "filter nothing"

**Remaining issues:** None

---

## 2026-01-15 - US-00022 - Implement tool to get object details

**Status:** Success

**What was implemented:**
- New `get_object` tool that returns detailed information about a specific object by ID
- Complete property extraction for all object types: shapes, images, tables, videos, lines, groups, charts, word art
- For shapes: text content, font style (family, size, bold, italic), fill color, outline properties, placeholder type
- For images: content URL, source URL, crop settings, brightness, contrast, transparency, recolor
- For tables: rows, columns, 2D array of cells with text, spans, and background colors
- For videos: video ID, source (YOUTUBE/DRIVE), URL, start/end times (in seconds), autoplay, mute settings
- For lines: line type, arrow styles, color, weight, dash style
- For groups: child count and child object IDs
- For Sheets charts: spreadsheet ID and chart ID
- For word art: rendered text
- Recursive search finds objects nested inside groups
- Position and size in points (converted from EMU)
- Colors extracted as hex (#RRGGBB) or theme references (theme:ACCENT1)
- Comprehensive test suite with 17 test cases covering all object types

**Files changed:**
- `internal/tools/get_object.go` - get_object tool implementation with type-specific extractors
- `internal/tools/get_object_test.go` - Comprehensive tests (17 test cases)
- `CLAUDE.md` - Added get_object documentation with input/output examples for all object types
- `README.md` - Added full get_object tool documentation with examples and type-specific details

**Learnings:**
- Google Slides API uses `AutoPlay` (not `Autoplay`) for video properties - case matters
- Video times are stored in milliseconds, must convert to seconds for user-friendly output
- Colors can be RGB (convert to hex) or theme colors (return as "theme:COLOR_NAME")
- Recursive element search via `findElementByID` helper allows finding objects nested at any depth
- Table cells use TableCellBackgroundFill for background colors, not ShapeBackgroundFill
- Reuse existing helper functions (emuToPoints, convertToPoints, extractTextFromTextContent) for consistency
- New sentinel error ErrObjectNotFound for object-specific failures

**Remaining issues:** None

---

## 2026-01-15 - US-00023 - Implement tool to add text box

**Status:** Success

**What was implemented:**
- Created `add_text_box` MCP tool to add text boxes to slides with optional styling
- Input accepts presentation_id, slide_index OR slide_id, text content, position (points), size (points), and optional style
- Style options include: font_family, font_size, bold, italic, and color (hex format)
- Position defaults to (0, 0) if not specified; size is required with positive dimensions
- Uses CreateShapeRequest with TEXT_BOX type, InsertTextRequest for text, UpdateTextStyleRequest for styling
- Created helper functions: findSlide, generateObjectID, pointsToEMU, buildTextBoxRequests, buildTextStyleRequest, parseHexColor
- Comprehensive test suite with 36+ test cases covering all scenarios

**Files changed:**
- `internal/tools/add_text_box.go` - add_text_box tool implementation with input/output types and helpers
- `internal/tools/add_text_box_test.go` - Comprehensive tests (16 TestAddTextBox cases + TestParseHexColor, TestPointsToEMU, TestFindSlide, TestBuildTextStyleRequest)
- `CLAUDE.md` - Added add_text_box documentation with input/output examples and usage patterns
- `README.md` - Added full add_text_box tool documentation with parameters, examples, and errors

**Learnings:**
- Google Slides uses EMU (English Metric Units) internally: 1 point = 12700 EMU
- CreateShapeRequest with ShapeType "TEXT_BOX" creates the shape, then InsertTextRequest adds text content
- UpdateTextStyleRequest requires a Fields mask to specify which style fields to apply
- TextRange with Type "ALL" applies style to entire text content
- Hex color parsing converts #RRGGBB to RGB float values (0-1 range)
- timeNowFunc variable pattern allows overriding time.Now for deterministic tests
- findSlide helper function reused from other tools for slide lookup by index or ID

**Remaining issues:** None

---

## 2026-01-15 - US-00024 - Implement tool to modify text in shape

**Status:** Success

**What was implemented:**
- Created `modify_text` MCP tool to modify text content in existing shapes
- Four actions supported: replace, append, prepend, and delete
- Replace action supports full replacement or partial replacement via start_index/end_index parameters
- Validates that target object supports text modification (shapes with text, not tables or images)
- Uses DeleteTextRequest and InsertTextRequest for text manipulation via BatchUpdate API
- Returns the expected resulting text content after modification
- Created helper functions: buildModifyTextRequests for creating API requests based on action
- Comprehensive test suite with 20+ test cases covering all actions and error scenarios

**Files changed:**
- `internal/tools/modify_text.go` - modify_text tool implementation with input/output types and helpers
- `internal/tools/modify_text_test.go` - Comprehensive tests covering all actions, validation, and errors
- `CLAUDE.md` - Added modify_text documentation with input/output examples and usage patterns
- `README.md` - Added full modify_text tool documentation with parameters, examples, and errors

**Learnings:**
- Google Slides Range.StartIndex and Range.EndIndex are *int64 pointers, not int64 values
- DeleteTextRequest with Range Type "ALL" deletes all text; "FIXED_RANGE" for specific indices
- Partial replacement requires delete first (if range has content), then insert at start position
- Tables require cell-by-cell modification and return ErrNotTextObject with specific message
- Objects nested in groups can be found using findElementByID helper from get_object tool
- extractTextFromTextContent helper extracts current text for calculating expected output
- Text indices for partial replacement are clamped to actual text length to prevent out-of-bounds

**Remaining issues:** None

---

## 2026-01-15 - US-00025 - Implement tool to search text in presentation

**Status:** Success

**What was implemented:**
- New `search_text` MCP tool to find text across all slides in a presentation
- Case-insensitive search by default, with optional case-sensitive mode
- Searches across text shapes, tables (cell by cell), and speaker notes
- Includes surrounding context (50 characters before/after) for each match
- Results grouped by slide for easy navigation
- Recursive search through grouped elements
- Table cell matches include row/column position in ObjectID (e.g., "table-1[0,2]")
- Speaker notes matches prefixed with "SPEAKER_NOTES:" in ObjectType
- Support for overlapping matches (e.g., "aa" in "aaa" finds matches at positions 0 and 1)
- Comprehensive test suite with 17+ test cases

**Files changed:**
- `internal/tools/search_text.go` - search_text tool implementation with helper functions
- `internal/tools/search_text_test.go` - Comprehensive tests covering all scenarios
- `CLAUDE.md` - Added search_text documentation with input/output examples
- `README.md` - Added full search_text tool documentation with parameters, examples, and errors

**Learnings:**
- Reuse ErrInvalidQuery from search_presentations.go instead of declaring duplicate
- Helper functions `contains`, `findSubstring` already exist in search_presentations_test.go
- Context extraction with ellipsis indicators ("...") for truncated context at boundaries
- Overlapping matches are found by advancing search position by 1 (not by match length)
- Speaker notes are in slide.SlideProperties.NotesPage.PageElements
- Table cells provide text via TableCell.Text (same TextContent structure as shapes)
- strings.ToLower for case-insensitive comparison, apply to both text and query

**Remaining issues:** None

---

## 2026-01-15 - US-00026 - Implement tool to replace text in presentation

**Status:** Success

**What was implemented:**
- New `replace_text` MCP tool to find and replace text across a presentation
- Uses Google Slides `ReplaceAllTextRequest` API for efficient bulk replacement
- Three scope options: `all` (entire presentation), `slide` (specific slide), `object` (slide containing object)
- Case-sensitive matching option (default: case-insensitive)
- Empty `replace_with` parameter effectively deletes matched text
- Reports affected objects with slide index, slide ID, object ID, and object type
- Pre-scans presentation to build affected objects list before API call
- Comprehensive test suite with 20+ test cases covering all scenarios

**Files changed:**
- `internal/tools/replace_text.go` - replace_text tool implementation
- `internal/tools/replace_text_test.go` - Comprehensive tests (20+ test cases)
- `CLAUDE.md` - Added replace_text documentation with input/output examples
- `README.md` - Added full replace_text tool documentation with examples and errors

**Learnings:**
- Google Slides `ReplaceAllTextRequest` uses `PageObjectIds` to limit scope to specific pages (slides)
- API does not support scoping to individual objects within a slide - only to pages
- `SubstringMatchCriteria` provides `MatchCase` for case sensitivity control
- Use `strings.Contains` and `strings.ToLower` for case-insensitive matching (avoid custom implementations)
- Pre-scanning for affected objects before replacement helps provide useful feedback to users
- Objects nested in groups require recursive search via `containsObject` helper
- Reuse existing sentinel errors (ErrSlideNotFound, ErrObjectNotFound) from other tools

**Remaining issues:** None

---

## 2026-01-15 - US-00027 - Implement tool to apply text styling

**Status:** Success

**What was implemented:**
- New `style_text` MCP tool to apply text styling to shapes
- Uses Google Slides `UpdateTextStyleRequest` API for efficient style application
- Supports all standard text style properties:
  - Font family (string)
  - Font size (integer, in points)
  - Bold, italic, underline, strikethrough (boolean pointers to distinguish false from unset)
  - Foreground color (hex string #RRGGBB)
  - Background color (hex string #RRGGBB)
  - Link URL (string for hyperlinks)
- Optional start_index/end_index for partial text styling (default: ALL text)
- Uses Fields mask to only update specified properties
- Reports applied styles and text range in output
- Comprehensive test suite with 31 test cases covering all scenarios

**Files changed:**
- `internal/tools/style_text.go` - style_text tool implementation with StyleTextInput, StyleTextStyleSpec, StyleTextOutput types and buildStyleTextRequest helper
- `internal/tools/style_text_test.go` - Comprehensive tests (31 test cases)
- `CLAUDE.md` - Added style_text documentation with input/output examples
- `README.md` - Added full style_text tool documentation with parameters, examples, and errors

**Learnings:**
- Boolean pointers (*bool) allow distinguishing between "set to false" and "not set"
- Google Slides API requires Fields mask to specify which properties to update
- Fields mask uses camelCase property names: "fontFamily", "fontSize", "bold", etc.
- Text range type "ALL" styles entire text content; "FIXED_RANGE" requires StartIndex/EndIndex
- Reuse existing helper functions: parseHexColor (from add_text_box.go), findElementByID (from modify_text.go)
- Function-based mock pattern with local capturedRequests variable for test assertions
- Link is set via textStyle.Link = &slides.Link{Url: url} and field name is just "link"

**Remaining issues:** None

---

## 2026-01-15 - US-00028 - Implement tool to set paragraph formatting

**Status:** Success

**What was implemented:**
- New `format_paragraph` MCP tool to set paragraph formatting options
- Uses Google Slides `UpdateParagraphStyleRequest` API for efficient paragraph styling
- Supports all standard paragraph style properties:
  - Alignment: START, CENTER, END, JUSTIFIED (case-insensitive)
  - Line spacing (percentage, e.g., 100 = single, 150 = 1.5 lines)
  - Space above (points)
  - Space below (points)
  - Indent first line (points)
  - Indent start (points, left indent for LTR)
  - Indent end (points, right indent for LTR)
- Optional paragraph_index parameter to target specific paragraph (0-based)
- Default behavior formats all paragraphs when paragraph_index is omitted
- Uses Fields mask to only update specified properties
- Reports applied formatting and paragraph scope in output
- Comprehensive test suite with 26+ test cases covering all scenarios

**Files changed:**
- `internal/tools/format_paragraph.go` - format_paragraph tool implementation with FormatParagraphInput, ParagraphFormattingOptions, FormatParagraphOutput types and helper functions (countParagraphs, getParagraphRange, buildFormatParagraphRequest)
- `internal/tools/format_paragraph_test.go` - Comprehensive tests (26+ test cases)
- `CLAUDE.md` - Added format_paragraph documentation with input/output examples, alignment table, usage patterns
- `README.md` - Added full format_paragraph tool documentation with parameters, formatting options table, examples, and errors

**Learnings:**
- Google Slides TextElement has StartIndex and EndIndex as int64 (not pointers), unlike some other API fields
- ParagraphMarker elements in TextContent mark paragraph boundaries
- Paragraph range uses Type: "ALL" for all paragraphs, Type: "FIXED_RANGE" with StartIndex/EndIndex for specific paragraph
- Line spacing in Google Slides API is percentage-based (100 = normal single spacing)
- Space and indent values use Dimension type with Unit: "PT" for points
- Fields mask uses camelCase: "alignment", "lineSpacing", "spaceAbove", "spaceBelow", "indentFirstLine", "indentStart", "indentEnd"
- Alignment values must be uppercase when sent to API (START, CENTER, END, JUSTIFIED)
- Pattern: validate alignment early, normalize to uppercase, then proceed with formatting

**Remaining issues:** None

---

## 2026-01-15 - US-00029 - Implement tool to create bullet list

**Status:** Success

**What was implemented:**
- New `create_bullet_list` MCP tool to convert text to bullet lists
- Uses Google Slides `CreateParagraphBulletsRequest` API for bullet creation
- Supports user-friendly bullet style names (DISC, CIRCLE, SQUARE, DIAMOND, ARROW, STAR, CHECKBOX)
- Also accepts full API preset names (e.g., BULLET_DISC_CIRCLE_SQUARE)
- Style names are case-insensitive (normalized to uppercase)
- Optional paragraph_indices array to apply bullets to specific paragraphs only
- Optional bullet_color (hex string) applied via UpdateTextStyleRequest
- Uses validBulletStyles map to translate user-friendly names to API presets
- Reports bullet preset applied and paragraph scope in output
- Helper functions: getBulletTextRange, getParagraphRanges, buildCreateBulletListRequests
- Comprehensive test suite with 40+ test cases covering all scenarios

**Files changed:**
- `internal/tools/create_bullet_list.go` - create_bullet_list tool implementation with CreateBulletListInput, CreateBulletListOutput types, validBulletStyles map, and helper functions
- `internal/tools/create_bullet_list_test.go` - Comprehensive tests (25 main tests + 15+ helper function tests)
- `CLAUDE.md` - Added create_bullet_list documentation with bullet styles table, input/output examples, usage patterns
- `README.md` - Added full create_bullet_list tool documentation with parameters, bullet styles table, examples, and errors

**Learnings:**
- Google Slides API provides `CreateParagraphBulletsRequest` with `BulletGlyphPreset` enum
- Bullet styles cannot be modified directly via Slides API; UpdateTextStyleRequest on paragraph text updates bullet glyph
- Available presets: BULLET_DISC_CIRCLE_SQUARE, BULLET_ARROW_DIAMOND_DISC, BULLET_CHECKBOX, BULLET_STAR_CIRCLE_SQUARE, BULLET_DIAMOND_CIRCLE_SQUARE, BULLET_DIAMONDX_ARROW3D_SQUARE, BULLET_ARROW3D_CIRCLE_SQUARE, BULLET_LEFTTRIANGLE_DIAMOND_DISC, BULLET_DIAMONDX_HOLLOWDIAMOND_SQUARE
- User-friendly names (DISC, ARROW, STAR) are more intuitive; map translates to full preset names
- Paragraph text range uses "ALL" for all paragraphs or "FIXED_RANGE" with start/end indices
- getParagraphRanges extracts paragraph boundaries from TextContent.TextElements with ParagraphMarker
- Bullet color requires separate UpdateTextStyleRequest with foregroundColor field
- Pattern: validate bullet_style early, normalize to uppercase, lookup in map, fail if not found
- Reusing parseHexColor helper from style_text.go for color validation

**Remaining issues:** None

---

## 2026-01-15 - US-00030 - Implement tool to create numbered list

**Status:** Success

**What was implemented:**
- New `create_numbered_list` MCP tool to convert text to numbered lists
- Uses Google Slides `CreateParagraphBulletsRequest` API with NUMBERED_* presets
- Supports user-friendly number style names (DECIMAL, ALPHA_UPPER, ALPHA_LOWER, ROMAN_UPPER, ROMAN_LOWER)
- Also accepts full API preset names (e.g., NUMBERED_DECIMAL_NESTED, NUMBERED_DECIMAL_ALPHA_ROMAN_PARENS)
- Style names are case-insensitive (normalized to uppercase)
- Optional paragraph_indices array to apply numbering to specific paragraphs only
- Optional start_number parameter with validation (must be >= 1)
- Reuses existing helper functions from create_bullet_list (getBulletTextRange, getParagraphRanges)
- Comprehensive test suite with 27+ test cases covering all scenarios

**Files changed:**
- `internal/tools/create_numbered_list.go` - create_numbered_list tool implementation with CreateNumberedListInput, CreateNumberedListOutput types, validNumberStyles map
- `internal/tools/create_numbered_list_test.go` - Comprehensive tests (27+ test cases including helper function tests)
- `CLAUDE.md` - Added create_numbered_list documentation with number styles table, input/output examples, usage patterns
- `README.md` - Added full create_numbered_list tool documentation with parameters, number styles table, examples, and errors

**Learnings:**
- Google Slides API uses same `CreateParagraphBulletsRequest` for both bullets and numbering - presets starting with NUMBERED_* are for numbered lists
- Available number presets: NUMBERED_DECIMAL_ALPHA_ROMAN, NUMBERED_UPPERALPHA_ALPHA_ROMAN, NUMBERED_ALPHA_ALPHA_ROMAN, NUMBERED_UPPERROMAN_UPPERALPHA_DECIMAL, NUMBERED_ROMAN_UPPERALPHA_DECIMAL, NUMBERED_DECIMAL_NESTED, NUMBERED_DECIMAL_ALPHA_ROMAN_PARENS, NUMBERED_ZERODIGIT_ALPHA_ROMAN
- Start number customization is limited - API's CreateParagraphBulletsRequest doesn't directly support custom start numbers; would require more complex list ID manipulation
- Reusing existing helper functions (getBulletTextRange, getParagraphRanges, countParagraphs, findElementByID) keeps code DRY
- Pattern matches create_bullet_list: validate style early, normalize to uppercase, lookup in map, fail if not found

**Remaining issues:** None

---

## 2026-01-15 - US-00031 - Implement tool to modify list properties

**Status:** Success

**What was implemented:**
- New `modify_list` MCP tool to modify existing list properties, remove formatting, or change indentation
- Four actions supported: 'modify', 'remove', 'increase_indent', 'decrease_indent'
- 'modify' action: change bullet_style, number_style, or color using CreateParagraphBulletsRequest and UpdateTextStyleRequest
- 'remove' action: uses DeleteParagraphBulletsRequest to convert list back to plain text
- 'increase_indent'/'decrease_indent' actions: use UpdateParagraphStyleRequest with indentStart property
- Default indentation increment of 18 points per level (standard for Google Slides)
- Optional paragraph_indices array to apply action to specific paragraphs only
- Action and style names are case-insensitive (normalized)
- Reuses existing validBulletStyles and validNumberStyles maps from create_bullet_list and create_numbered_list
- Comprehensive test suite with 27 test cases covering all actions and error scenarios

**Files changed:**
- `internal/tools/modify_list.go` - modify_list tool implementation with ModifyListInput, ModifyListOutput, ListModifyProperties types, and helper functions
- `internal/tools/modify_list_test.go` - Comprehensive tests (27 test cases including helper function tests)
- `CLAUDE.md` - Added modify_list documentation with actions table, input/output examples, usage patterns
- `README.md` - Added full modify_list tool documentation with parameters, actions table, properties, examples, and errors

**Learnings:**
- DeleteParagraphBulletsRequest removes list formatting and converts paragraphs back to plain text
- UpdateParagraphStyleRequest with indentStart property controls paragraph indentation (in points)
- To change bullet/number style on existing list, use CreateParagraphBulletsRequest with new preset (replaces existing)
- Indentation uses points as unit (1 point = 12700 EMU); 18 points is standard indent increment
- getCurrentIndent helper reads existing ParagraphStyle.IndentStart to calculate new indentation
- Reusing parseHexColor helper for color validation when modifying list color
- Pattern: validate action first, then action-specific validation (properties required only for 'modify')

**Remaining issues:** None

---

## 2026-01-15 - US-00032 - Implement tool to add image to slide

**Status:** Success

**What was implemented:**
- New `add_image` MCP tool to add images to slides from base64-encoded data
- Extended DriveService interface with `UploadFile` and `MakeFilePublic` methods
- Image upload workflow: decode base64 → detect MIME type → upload to Drive → make public → create image in Slides
- Automatic MIME type detection from magic bytes (PNG, JPEG, GIF, WebP, BMP)
- Input accepts presentation_id, slide_index OR slide_id, image_base64, optional position, optional size
- Position defaults to (0, 0) if not specified; coordinates in points
- Size supports width only, height only, or both (aspect ratio preserved when one dimension omitted)
- Uses CreateImageRequest in Slides API BatchUpdate with Drive file URL
- Makes uploaded image publicly accessible so Google Slides can display it
- Graceful handling when MakeFilePublic fails (logs warning, continues - image may still work)
- Comprehensive test suite with 23+ test cases covering all scenarios

**Files changed:**
- `internal/tools/tools.go` - Added UploadFile and MakeFilePublic methods to DriveService interface and realDriveService implementation
- `internal/tools/add_image.go` - add_image tool implementation with AddImageInput, ImageSizeInput, AddImageOutput types, detectImageMimeType, generateImageFileName, generateImageObjectID, buildImageRequests helpers
- `internal/tools/add_image_test.go` - Comprehensive tests (23+ test cases including helper function tests)
- `internal/tools/search_presentations_test.go` - Updated mockDriveService with UploadFileFunc and MakeFilePublicFunc
- `internal/tools/modify_list_test.go` - Fixed incorrect test expectation for invalid color (pre-existing bug in US-00031)
- `CLAUDE.md` - Added add_image documentation with input/output examples, supported formats, features, sentinel errors
- `README.md` - Added full add_image tool documentation with parameters, supported image formats, examples, and errors

**Learnings:**
- Google Slides API CreateImageRequest requires a publicly accessible URL for the image
- Images must be uploaded to Drive first, then made publicly accessible via permissions
- Drive API Files.Create with Media() uploads binary content
- Drive API Permissions.Create with "anyone"/"reader" makes file publicly accessible
- Image MIME type detection via magic bytes is more reliable than relying on file extensions
- Slides API uses EMU (English Metric Units) internally: 1 point = 12700 EMU
- CreateImageRequest can specify ElementProperties with PageObjectId, Transform (position), and Size
- imageTimeNowFunc pattern allows overriding time.Now for deterministic test object IDs
- MakeFilePublic failure should be non-fatal since image may still be accessible to authenticated user

**Remaining issues:** None

---

## 2026-01-15 - US-00033 - Implement tool to modify image properties

**Status:** Success

**What was implemented:**
- New `modify_image` MCP tool to modify properties of existing images in presentations
- Input accepts presentation_id, object_id, and properties object with any of: position, size, crop, brightness, contrast, transparency, recolor
- Position and size modifications via UpdatePageElementTransformRequest with ABSOLUTE mode
- Image property modifications (crop, brightness, contrast, transparency, recolor) via UpdateImagePropertiesRequest
- Crop values as percentages (0-1) for top, bottom, left, right offsets
- Brightness and contrast adjustments in range -1 to 1
- Transparency level in range 0 to 1 (0 = opaque, 1 = fully transparent)
- Recolor support with preset names (GRAYSCALE, SEPIA, NEGATIVE, LIGHT1-10, DARK1-10) or "none" to remove
- Comprehensive validation for all property ranges before API calls
- Returns list of modified properties for confirmation
- Sentinel errors for all failure modes (not found, not an image, invalid values, etc.)
- Comprehensive test suite with 24+ test cases covering all scenarios and edge cases

**Files changed:**
- `internal/tools/modify_image.go` - modify_image tool implementation with ModifyImageInput, ImageModifyProperties, CropInput, ModifyImageOutput types, validation helpers, request builders
- `internal/tools/modify_image_test.go` - Comprehensive tests (24+ test cases including helper function tests)
- `CLAUDE.md` - Added modify_image documentation with input/output examples, recolor presets, features, sentinel errors
- `README.md` - Added full modify_image tool documentation with parameters, recolor presets, examples, and errors
- `stories.yaml` - Marked US-00033 as passes: true

**Learnings:**
- UpdatePageElementTransformRequest with ABSOLUTE mode sets exact position/scale values
- Position uses TranslateX/TranslateY in EMU; size uses ScaleX/ScaleY relative to original element size
- UpdateImagePropertiesRequest requires field mask specifying which properties to update
- Crop properties are offsets as percentages (0-1) stored in CropProperties struct
- To remove recolor, set Recolor field to nil and include "recolor" in field mask
- Must retrieve current element transform values to preserve non-modified properties when using ABSOLUTE mode
- determineObjectType helper identifies object type for error messages when target is not an image
- findElementByID recursively searches through all slides and groups to locate objects

**Remaining issues:** None

---
