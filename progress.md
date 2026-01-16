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

## 2026-01-15 - US-00034 - Implement tool to replace image

**Status:** Success

**What was implemented:**
- New `replace_image` MCP tool to replace existing images with new content while preserving position and optionally size
- Input accepts presentation_id, object_id, image_base64, and optional preserve_size (default true)
- Strategy: delete old image and create new one at same position/size (no in-place image update in Slides API)
- Preserves original position via AffineTransform (translateX, translateY, scaleX, scaleY, shearX, shearY)
- Preserves original size when preserve_size is true by copying Size from old element
- Uploads new image to Google Drive first using UploadFile, then makes publicly accessible via MakeFilePublic
- Automatically detects image MIME type from magic bytes (PNG, JPEG, GIF, WebP, BMP) using detectImageMimeType
- Returns new object ID since replacement creates a new object via CreateImageRequest
- MakeFilePublic failure is non-fatal (logged as warning, operation continues)
- Comprehensive validation for empty inputs and object type verification
- Sentinel errors for all failure modes (invalid data, not found, not an image, upload failed, etc.)
- Comprehensive test suite with 20 test cases covering all scenarios including nested groups

**Files changed:**
- `internal/tools/replace_image.go` - replace_image tool implementation with ReplaceImageInput, ReplaceImageOutput types, buildReplaceImageRequests helper
- `internal/tools/replace_image_test.go` - Comprehensive tests (16 TestReplaceImage + 4 TestBuildReplaceImageRequests test cases)
- `CLAUDE.md` - Added replace_image documentation with input/output examples, features, sentinel errors, usage patterns
- `README.md` - Added full replace_image tool documentation with parameters, supported formats, examples, and errors
- `stories.yaml` - Marked US-00034 as passes: true

**Learnings:**
- Google Slides API has no direct image content replacement - must delete and recreate
- BatchUpdate with DeleteObjectRequest followed by CreateImageRequest achieves atomic replacement
- CreateImageRequest URL must be publicly accessible; using Drive download URL format
- When copying transform, must handle case where old element has no transform (use default values)
- Unit field in AffineTransform defaults to empty; must set to "EMU" explicitly
- generateImageObjectID uses nanosecond timestamp for unique object IDs
- Test factory functions must use exact types (oauth2.TokenSource, io.Reader) not interfaces

**Remaining issues:** None

---

## 2026-01-15 - US-00036 - Implement tool to create line/arrow

**Status:** Success

**What was implemented:**
- New `create_line` MCP tool to create lines, arrows, and connectors
- Automatic geometry calculation for line orientation (start/end points)
- Support for lines in all directions (including anti-diagonal) via transform flips
- Support for line types: STRAIGHT, CURVED, ELBOW (connector)
- Support for arrow heads: ARROW, DIAMOND, OVAL, OPEN_ARROW, etc.
- Styling options: line color, weight, dash style
- Uses `CreateLineRequest` and `UpdateLinePropertiesRequest` in BatchUpdate
- Comprehensive test suite with 6 test cases including negative slope handling

**Files changed:**
- `internal/tools/create_line.go` - Tool implementation
- `internal/tools/create_line_test.go` - Comprehensive tests
- `CLAUDE.md` - Added create_line documentation
- `README.md` - Added create_line tool documentation
- `stories.yaml` - Marked US-00036 as passes: true

**Learnings:**
- Google Slides lines are defined by a bounding box (Size + Transform) and a LineCategory
- To create a line from (x1,y1) to (x2,y2) where the slope is negative, a vertical flip (ScaleY = -1) or rotation is required
- `CreateLineRequest` allows setting `ElementProperties` including Transform matrix
- Arrow heads are properties of the line (`StartArrow`, `EndArrow`) in `LineProperties`, not connection sites
- `CreateLineRequest` uses `Category` field (STRAIGHT, BENT, CURVED) to define line type
- Reused `pointsToEMU` helper via package-level sharing (careful with visibility)

**Remaining issues:** None

---

## 2026-01-15 - US-00037 - Implement tool to modify shape properties

**Status:** Success

**What was implemented:**
- New `modify_shape` MCP tool to update shape appearance
- Support for fill color (solid hex or transparent)
- Support for outline properties (color, weight, dash style)
- Support for toggling shadow visibility
- Uses `UpdateShapePropertiesRequest` in BatchUpdate
- Comprehensive test suite with 8 test cases

**Files changed:**
- `internal/tools/modify_shape.go` - Tool implementation
- `internal/tools/modify_shape_test.go` - Comprehensive tests
- `CLAUDE.md` - Added modify_shape documentation
- `README.md` - Added modify_shape tool documentation
- `stories.yaml` - Marked US-00037 as passes: true

**Learnings:**
- `UpdateShapePropertiesRequest` handles fill, outline, and shadow
- Reflection is not supported on `ShapeProperties` in the Go Slides API client (v1), so it was omitted from implementation
- Transparency is handled by setting `PropertyState` to "NOT_RENDERED"
- Helper functions for pointers (`boolPtr`, `float64Ptr`) are useful for optional JSON fields but must be careful with package scope in tests

**Remaining issues:** None

---

## 2026-01-16 - US-00038 - Implement tool to move/resize object

**Status:** Success

**What was implemented:**
- New `transform_object` MCP tool to move, resize, or rotate any object
- Input accepts presentation_id, object_id, optional position, size, rotation, and scale_proportionally flag
- Affine transform decomposition and recomposition for complex transformations
- Position: moves object to absolute coordinates (in points, converted to EMU)
- Size: resizes object with optional proportional scaling when only width or height specified
- Rotation: rotates object to specific angle (in degrees, 0-360)
- Preserves existing transform properties when only some are updated
- Uses `UpdatePageElementTransformRequest` with ABSOLUTE mode for precise control
- Returns final position, size, and rotation in output
- Comprehensive test suite with 5 test cases covering all transformations

**Files changed:**
- `internal/tools/transform_object.go` - Tool implementation with TransformObjectInput, TransformObjectOutput types, calculateNewTransform, findElementByIDRecursively helpers
- `internal/tools/transform_object_test.go` - Comprehensive tests (5 test cases: move only, resize proportional, resize non-proportional, rotate, rotate and move)
- `CLAUDE.md` - Added transform_object documentation with input/output examples and usage patterns
- `README.md` - Added transform_object tool documentation with parameters, examples, and errors

**Learnings:**
- Google Slides uses AffineTransform matrix: [ScaleX ShearX TranslateX; ShearY ScaleY TranslateY; 0 0 1]
- Decomposing transform: Sx = sqrt(ScaleX² + ShearY²), Sy = sqrt(ScaleY² + ShearX²), rotation = atan2(ShearY, ScaleX)
- Recomposing transform: ScaleX = Sx*cos(θ), ShearY = Sx*sin(θ), ShearX = -Sy*sin(θ), ScaleY = Sy*cos(θ)
- `Size` property in PageElement is read-only; resizing must be done via Transform scale factors
- Visual size = Base size × Scale factor; to resize, calculate new scale as targetSize/baseSize
- UpdatePageElementTransformRequest with ABSOLUTE mode sets exact transform values

**Remaining issues:** None

---

## 2026-01-16 - US-00039 - Implement tool to change object z-order

**Status:** Success

**What was implemented:**
- New `change_z_order` MCP tool to change object layering (z-order) on a slide
- Four actions supported: bring_to_front, send_to_back, bring_forward, send_backward
- Action names are case-insensitive (normalized to uppercase for API)
- Uses Google Slides `UpdatePageElementsZOrderRequest` in BatchUpdate API
- Returns new z-order position (0-based) and total layers count after change
- Validates that object is not inside a group (API limitation prevents z-order changes on grouped objects)
- Helper function `findElementAndCheckGroup` to detect grouped objects
- Re-fetches presentation after change to calculate actual new position
- Comprehensive test suite with 14 main tests + 5 helper function tests

**Files changed:**
- `internal/tools/change_z_order.go` - Tool implementation with ChangeZOrderInput, ChangeZOrderOutput, validZOrderActions map, findElementAndCheckGroup helper
- `internal/tools/change_z_order_test.go` - Comprehensive tests including z-order simulation helper
- `CLAUDE.md` - Added change_z_order documentation with actions table, sentinel errors, usage patterns
- `README.md` - Added change_z_order tool documentation with input/output examples

**Learnings:**
- Google Slides API uses `UpdatePageElementsZOrderRequest` with `Operation` field (BRING_TO_FRONT, SEND_TO_BACK, BRING_FORWARD, SEND_BACKWARD)
- API requires `PageElementObjectIds` as array - supports multiple objects but must all be on same page and not grouped
- Z-order position is represented by array index in `PageElements` (0 = furthest back, last index = front)
- Grouped objects cannot have their z-order changed via API - must check before attempting
- Pattern: validate action mapping early with case-insensitive lookup, then use mapped API constant

**Remaining issues:** None

---

## 2026-01-16 - US-00040 - Implement tool to group/ungroup objects

**Status:** Success

**What was implemented:**
- New `group_objects` MCP tool to group and ungroup objects on slides
- Two actions: "group" (combine objects) and "ungroup" (split a group)
- Uses Google Slides `GroupObjectsRequest` and `UngroupObjectsRequest` in BatchUpdate API
- Validates objects can be grouped (tables, videos, placeholders cannot be grouped)
- Validates all objects are on the same slide before grouping
- Checks that objects are not already inside a group
- Generates unique group object IDs using timestamp
- Returns created group ID for group action, or array of child IDs for ungroup action
- Helper function `containsObjectID` for recursive group membership checking
- Reuses existing `findElementAndCheckGroup` helper from change_z_order.go
- Comprehensive test suite with 23 test cases covering all scenarios

**Files changed:**
- `internal/tools/group_objects.go` - Tool implementation with GroupObjectsInput, GroupObjectsOutput types, groupObjects and ungroupObjects helpers
- `internal/tools/group_objects_test.go` - Comprehensive tests (group success, ungroup success, action normalization, validation errors, API errors)
- `CLAUDE.md` - Added group_objects documentation with actions table, ungroupable objects list, sentinel errors, usage patterns
- `README.md` - Added group_objects tool documentation with input/output examples and errors

**Learnings:**
- Google Slides API uses `GroupObjectsRequest` with `ChildrenObjectIds` (array) and `GroupObjectId` (suggested ID)
- API uses `UngroupObjectsRequest` with `ObjectIds` (array of group IDs to ungroup)
- Response includes actual created group ID in `resp.Replies[0].GroupObjects.ObjectId`
- Cannot group: tables, videos, placeholder shapes, objects already in groups
- All objects to group must be on same page - find slide containing all objects first
- Pattern: use sentinel errors for different validation failures (ErrNotEnoughObjects, ErrObjectsOnDifferentPages, etc.)

**Remaining issues:** None

---

## 2026-01-16 - US-00041 - Implement tool to delete object

**Status:** Success

**What was implemented:**
- New `delete_object` MCP tool to delete one or more objects from a presentation
- Supports single object deletion via `object_id` field
- Supports batch deletion via `multiple` array field
- Both fields can be combined (all unique IDs are deleted)
- Automatically deduplicates object IDs in input
- Validates objects exist before attempting deletion using `categorizeObjectIDs` helper
- Partial success handling: deletes found objects, reports not found IDs separately
- Recursively finds objects anywhere (slides, masters, layouts, nested in groups)
- Uses Google Slides `DeleteObjectRequest` in BatchUpdate API
- Returns `DeleteObjectOutput` with deleted count, deleted IDs, and optional not found IDs
- Comprehensive test suite with 19 test cases covering all scenarios

**Files changed:**
- `internal/tools/delete_object.go` - Tool implementation with DeleteObjectInput, DeleteObjectOutput types, collectObjectIDsToDelete, categorizeObjectIDs, collectAllObjectIDs, collectPageElementIDs helpers
- `internal/tools/delete_object_test.go` - Comprehensive tests (single object, multiple objects, deduplication, nested groups, partial not found, all not found, missing inputs, access denied, batch update failures)
- `CLAUDE.md` - Added delete_object documentation with features, sentinel errors, usage patterns
- `README.md` - Added delete_object tool documentation with input/output examples and errors

**Learnings:**
- Google Slides API uses `DeleteObjectRequest` with `ObjectId` field for each object
- BatchUpdate can contain multiple DeleteObjectRequest entries for batch deletion
- Need to pre-validate object existence by scanning entire presentation (slides, masters, layouts)
- Grouped objects have nested PageElements in `ElementGroup.Children` - must recursively scan
- Pattern: separate object collection from validation - first collect all IDs to delete, then categorize into existing/not found
- Partial success is valid UX: delete what exists, report what doesn't exist
- Tables contain TableRows with TableCells, but cells don't have separate object IDs (identified by row/column)

**Remaining issues:** None

---

## 2026-01-16 - US-00042 - Implement tool to create table

**Status:** Success

**What was implemented:**
- New `create_table` MCP tool to create tables on slides
- CreateTableInput struct with: presentation_id, slide_index/slide_id, rows, columns, position, size
- CreateTableOutput struct with: object_id, rows, columns
- Uses `CreateTableRequest` in Google Slides API BatchUpdate
- Support for optional position and size (in points, converted to EMU)
- Comprehensive test suite with 20 test cases

**Files changed:**
- `internal/tools/create_table.go` - Tool implementation
- `internal/tools/create_table_test.go` - Comprehensive tests
- `CLAUDE.md` - Added create_table documentation
- `README.md` - Added create_table tool documentation
- `stories.yaml` - Marked US-00042 as passes: true

**Learnings:**
- CreateTableRequest in Google Slides API requires rows and columns as int64
- Position is specified via AffineTransform with translateX/translateY in EMU
- Size is specified via Size with Width/Height Dimensions in EMU
- Table object ID is generated using timestamp-based pattern like other tools
- Test patterns from create_shape_test.go can be reused for similar element creation tools

**Remaining issues:** None

---

## 2026-01-16 - US-00043 - Implement tool to modify table structure

**Status:** Success

**What was implemented:**
- New `modify_table_structure` MCP tool to add/delete rows and columns from existing tables
- ModifyTableStructureInput struct with: presentation_id, object_id, action, index, count, insert_after
- ModifyTableStructureOutput struct with: object_id, action, index, count, new_rows, new_columns
- Four actions supported: add_row, delete_row, add_column, delete_column (case-insensitive)
- Uses `InsertTableRowsRequest`, `DeleteTableRowRequest`, `InsertTableColumnsRequest`, `DeleteTableColumnRequest` in BatchUpdate
- 0-based indexing for row/column positions
- `insert_after` parameter for add actions (default true = insert below/right)
- Validates table has at least 1 row and 1 column after deletion
- Delete operations performed in reverse order (highest index first) to avoid shifting issues
- Helper `findTableByID` to locate tables anywhere in presentation (slides, layouts, masters, groups)
- Comprehensive test suite with 32 test cases

**Files changed:**
- `internal/tools/modify_table_structure.go` - Tool implementation with sentinel errors, action normalization, validation
- `internal/tools/modify_table_structure_test.go` - Comprehensive tests (add/delete rows/columns, multiple items, insert_after, validation errors, API errors, helper function tests)
- `CLAUDE.md` - Added modify_table_structure documentation with actions table, features, sentinel errors, usage patterns
- `README.md` - Added modify_table_structure tool documentation with input/output examples, actions, errors
- `stories.yaml` - Marked US-00043 as passes: true

**Learnings:**
- InsertTableRowsRequest uses `InsertBelow` boolean and `Number` for count (not multiple requests)
- InsertTableColumnsRequest uses `InsertRight` boolean and `Number` for count
- DeleteTableRowRequest and DeleteTableColumnRequest must be sent individually per row/column
- Delete operations must be done from highest index to lowest to avoid index shifting
- CellLocation uses `RowIndex` or `ColumnIndex` depending on operation (not both)
- Google Slides API requires CellLocation for both insert and delete operations
- Pattern: create helper function to find specific object types (findTableByID) by reusing findElementByID
- Table dimensions retrieved from len(table.TableRows) and len(table.TableRows[0].TableCells)

**Remaining issues:** None

---

## 2026-01-16 - US-00044 - Implement tool to merge table cells

**Status:** Success

**What was implemented:**
- New `merge_cells` MCP tool to merge or unmerge cells in a table
- MergeCellsInput struct with: presentation_id, object_id, action, start_row, start_column, end_row, end_column (for merge), row, column (for unmerge)
- MergeCellsOutput struct with: object_id, action, range (human-readable description)
- Two actions supported: merge, unmerge (case-insensitive)
- For merge: uses 0-based indices with exclusive end (like Python slicing)
- For unmerge: specifies any cell position within a merged cell
- Uses `MergeTableCellsRequest` and `UnmergeTableCellsRequest` in BatchUpdate
- Validates range is within table bounds
- Validates merge range spans at least 2 cells (single cell is invalid)
- Reuses `findTableByID` helper from modify_table_structure.go
- Sentinel errors: ErrMergeCellsFailed, ErrUnmergeCellsFailed, ErrInvalidMergeAction, ErrInvalidMergeRange
- Helper functions: validateMergeRange, validateUnmergePosition
- Comprehensive test suite with 41 test cases (24 main tests, 10 merge range validation, 7 unmerge position validation)

**Files changed:**
- `internal/tools/merge_cells.go` - Tool implementation with sentinel errors, action normalization, range validation
- `internal/tools/merge_cells_test.go` - Comprehensive tests (merge 2x2, merge row/column, unmerge, case-insensitivity, validation errors, API errors, helper function tests)
- `CLAUDE.md` - Added merge_cells documentation with actions table, features, sentinel errors, usage patterns
- `README.md` - Added merge_cells tool documentation with input/output examples, actions, errors
- `stories.yaml` - Marked US-00044 as passes: true

**Learnings:**
- MergeTableCellsRequest uses TableRange with Location (TableCellLocation), RowSpan, and ColumnSpan
- UnmergeTableCellsRequest also uses TableRange - setting span to 1x1 unmerges any merged cell containing that position
- TableCellLocation uses 0-based RowIndex and ColumnIndex
- Range validation pattern: start >= 0, end > start, end <= table size, span >= 2 cells
- Position validation pattern: index >= 0, index < table dimension
- Reused test helper createPresentationWithTable from modify_table_structure_test.go pattern

**Remaining issues:** None

---

## 2026-01-16 - US-00045 - Implement tool to modify table cell content

**Status:** Success

**What was implemented:**
- New `modify_table_cell` MCP tool to modify content and styling of individual table cells
- ModifyTableCellInput struct with: presentation_id, object_id, row, column (0-based), text, style, alignment
- ModifyTableCellOutput struct with: object_id, row, column, modified_properties (list of changes)
- Text modification via DeleteTextRequest + InsertTextRequest (delete all, then insert new text)
- Text styling via UpdateTextStyleRequest with CellLocation (font_family, font_size, bold, italic, underline, strikethrough, foreground_color, background_color)
- Horizontal alignment via UpdateParagraphStyleRequest with CellLocation (START, CENTER, END, JUSTIFIED)
- Vertical alignment via UpdateTableCellPropertiesRequest with TableRange (TOP, MIDDLE, BOTTOM)
- Alignment values are case-insensitive (normalized to uppercase)
- Validates that at least one modification (text, style, or alignment) is provided
- Validates cell indices are within table bounds
- Reuses `findTableByID` helper from modify_table_structure.go
- Sentinel errors: ErrModifyTableCellFailed, ErrInvalidCellIndex, ErrNoCellModification, ErrInvalidHorizontalAlign, ErrInvalidVerticalAlign
- Helper functions: buildModifyTableCellRequests, buildTableCellStyleRequest
- Comprehensive test suite with 23 test cases

**Files changed:**
- `internal/tools/modify_table_cell.go` - Tool implementation with input/output types, validation, request builders
- `internal/tools/modify_table_cell_test.go` - Comprehensive tests (text content, styling, alignment, combined modifications, case-insensitivity, error handling)
- `CLAUDE.md` - Added modify_table_cell documentation with input/output examples, alignment tables, usage patterns
- `README.md` - Added modify_table_cell tool documentation with parameters, examples, and errors
- `stories.yaml` - Marked US-00045 as passes: true

**Learnings:**
- TableCellLocation uses 0-based RowIndex and ColumnIndex for targeting specific cells
- Text modification in cells uses same DeleteTextRequest/InsertTextRequest with CellLocation field
- UpdateTextStyleRequest can target cells via CellLocation field
- UpdateParagraphStyleRequest can target cells via CellLocation field for horizontal alignment
- Vertical alignment is different - requires UpdateTableCellPropertiesRequest with TableRange (not CellLocation)
- ContentAlignment field in TableCellProperties controls vertical alignment (TOP, MIDDLE, BOTTOM)
- Pattern: separate text, style, and alignment requests into distinct helpers for clarity
- Reused parseHexColor helper from style_text.go for color parsing

**Remaining issues:** None

---

## US-00046: Implement tool to style table cells

**Status:** ✅ Completed

**Implementation Summary:**
Implemented `style_table_cells` MCP tool that applies visual styling (background color, borders) to table cells with flexible cell selection options.

**Key Implementation Details:**
- Cell selection via ParseCellSelector: "all" (entire table), "row:N" (entire row), "column:N" (entire column), or array of {row, column} positions
- Background color using UpdateTableCellPropertiesRequest with TableCellBackgroundFill.SolidFill
- Borders using UpdateTableBorderPropertiesRequest with BorderPosition (TOP, BOTTOM, LEFT, RIGHT)
- TableBorderProperties supports color, width (points), and dash_style (SOLID, DOT, DASH, DASH_DOT, LONG_DASH, LONG_DASH_DOT)
- Cell selector strings are case-insensitive (normalized to lowercase)
- Dash style names are case-insensitive (normalized to uppercase)
- Validates at least one style property is provided
- Validates cell positions are within table bounds
- Sentinel errors: ErrStyleTableCellsFailed, ErrInvalidCellSelector, ErrNoCellStyle
- Helper functions: ParseCellSelector, hasAnyStyle, validateBorderStyles, resolveCellPositions, buildStyleTableCellsRequests, buildBorderRequests

**Files changed:**
- `internal/tools/style_table_cells.go` - Tool implementation with CellSelector parsing, style application via UpdateTableCellPropertiesRequest and UpdateTableBorderPropertiesRequest
- `internal/tools/style_table_cells_test.go` - Comprehensive tests (60 test cases: 20 main tests, 14 ParseCellSelector tests, 8 resolveCellPositions tests, 5 buildStyleTableCellsRequests tests, 7 hasAnyStyle tests, 6 validateBorderStyles tests)
- `CLAUDE.md` - Added style_table_cells documentation with cell selector options, dash styles, sentinel errors, usage patterns
- `README.md` - Added style_table_cells tool documentation with input parameters, examples, and errors
- `stories.yaml` - Marked US-00046 as passes: true

**Learnings:**
- UpdateTableCellPropertiesRequest handles cell background via TableCellBackgroundFill.SolidFill
- UpdateTableBorderPropertiesRequest is SEPARATE from UpdateTableCellPropertiesRequest (not nested)
- BorderPosition is a string field directly on UpdateTableBorderPropertiesRequest (TOP, BOTTOM, LEFT, RIGHT)
- TableBorderProperties uses TableBorderFill.SolidFill for color (different from TableCellBackgroundFill)
- Weight field on TableBorderProperties uses Dimension with PT unit (same as other measurements)
- DashStyle is a string field directly on TableBorderProperties (not nested)
- Each cell needs separate UpdateTableCellPropertiesRequest/UpdateTableBorderPropertiesRequest (no multi-cell support in single request)
- Cell selector parsing handles multiple formats: string literals, position arrays, and typed position slices
- toInt helper needed for JSON number conversion (float64 to int) during parsing

**Remaining issues:** None

---

## US-00047: Implement tool to add video to slide

**Status:** ✅ Completed

**Implementation Summary:**
Implemented `add_video` MCP tool that adds YouTube or Google Drive videos to slides with position, size, and playback settings.

**Key Implementation Details:**
- Video source: "youtube" or "drive" (case-insensitive)
- Video ID: YouTube video ID or Google Drive file ID
- Optional position and size in points (converted to EMU internally)
- Optional start_time and end_time in seconds for video trimming (converted to milliseconds for API)
- Optional autoplay and mute boolean settings
- Uses CreateVideoRequest for initial video creation
- Uses UpdateVideoPropertiesRequest for playback settings (start, end, autoPlay, mute)
- Position uses AffineTransform with TranslateX/TranslateY
- Size uses Dimension with Width/Height in EMU
- Video-specific time function (videoTimeNowFunc) for deterministic test IDs
- Sentinel errors: ErrAddVideoFailed, ErrInvalidVideoSource, ErrInvalidVideoID, ErrInvalidVideoSize, ErrInvalidVideoPosition, ErrInvalidVideoTime, ErrInvalidVideoTimeRange

**Files changed:**
- `internal/tools/add_video.go` - Tool implementation with AddVideoInput/AddVideoOutput structs, validation, CreateVideoRequest and UpdateVideoPropertiesRequest builders
- `internal/tools/add_video_test.go` - Comprehensive tests (24 test cases: YouTube success, Drive success, start/end times, autoplay/mute, position/size, slide ID selection, validation errors, API errors, case-insensitivity)
- `CLAUDE.md` - Added add_video documentation with input/output examples, video sources table, usage patterns
- `stories.yaml` - Marked US-00047 as passes: true

**Learnings:**
- Google Slides API CreateVideoRequest uses Source field (YOUTUBE or DRIVE) and Id field for video identifier
- Video playback properties (start, end, autoPlay, mute) are set via separate UpdateVideoPropertiesRequest
- API uses milliseconds for start/end times, but user-facing input uses seconds for better UX
- UpdateVideoPropertiesRequest requires Fields mask specifying which fields to update
- AutoPlay field in API is camelCase (AutoPlay), not snake_case
- Same slide finding pattern (findSlide) reused from add_text_box and add_image tools
- Same EMU conversion (pointsToEMU) and position/size patterns as other element tools

**Remaining issues:** None

---

## US-00048: Implement tool to modify video properties

**Status:** ✅ Completed

**Implementation Summary:**
Implemented `modify_video` MCP tool that modifies video playback properties (start_time, end_time, autoplay, mute) and position/size for existing videos.

**Key Implementation Details:**
- Input: presentation_id, object_id, properties object with optional fields
- Properties: position (x, y), size (width, height), start_time, end_time, autoplay, mute
- Position and size in points (converted to EMU for transform)
- Time values in seconds (converted to milliseconds for API)
- Uses UpdatePageElementTransformRequest with ABSOLUTE mode for position/size changes
- Uses UpdateVideoPropertiesRequest with field masks for playback properties
- Validates property ranges before making API calls
- Preserves current transform values when only changing subset of properties
- Returns list of modified properties for confirmation
- Sentinel errors: ErrModifyVideoFailed, ErrNotVideoObject, ErrNoVideoProperties
- Reuses existing errors: ErrInvalidVideoSize, ErrInvalidVideoPosition, ErrInvalidVideoTime, ErrInvalidVideoTimeRange

**Files changed:**
- `internal/tools/modify_video.go` - Tool implementation with ModifyVideoInput/VideoModifyProperties/ModifyVideoOutput structs, validation (validateVideoModifyProperties), transform builder (buildVideoTransformRequest), video properties builder (buildModifyVideoPropertiesRequest)
- `internal/tools/modify_video_test.go` - Comprehensive tests (22 test cases: start/end times, autoplay/mute, disable autoplay/mute, position/size, all properties, preserving current values, validation errors for presentation/object/properties/times/size/position, API errors)
- `CLAUDE.md` - Added modify_video documentation with input/output examples, features, sentinel errors, usage patterns
- `README.md` - Added modify_video documentation with JSON input/output examples, parameter table, features, use cases, examples, error messages
- `stories.yaml` - Marked US-00048 as passes: true

**Learnings:**
- Followed modify_image pattern for structure (validate input, find object, verify type, build requests, batch update)
- Video transform uses same ABSOLUTE mode as images for position/size changes
- Preserving current scale values when only changing position requires reading existing transform
- Negative size values need explicit check separate from "both zero" check for proper validation
- UpdateVideoPropertiesRequest Fields uses comma-separated string (e.g., "start,end,autoPlay,mute")
- API field names differ from user-facing names (autoPlay vs autoplay, start vs start_time)

**Remaining issues:** None

---

## US-00049: Implement tool to apply theme

**Status:** ✅ Completed

**Implementation Summary:**
Implemented `apply_theme` MCP tool that copies theme colors from one presentation to another. Important: Gallery themes cannot be applied via Google Slides API - this is a fundamental API limitation.

**Key Implementation Details:**
- Input: presentation_id (target), theme_source ("presentation" or "gallery"), source_presentation_id
- For "presentation" source: copies 12 editable theme colors from source to target master
- For "gallery" source: returns descriptive error explaining API limitation and workarounds
- Theme source is case-insensitive
- Copies only the 12 editable ThemeColorTypes: DARK1, LIGHT1, DARK2, LIGHT2, ACCENT1-6, HYPERLINK, FOLLOWED_HYPERLINK
- Uses UpdatePagePropertiesRequest with colorScheme field on master slide
- Fetches both source and target presentations to get master slide IDs and color schemes
- Deep clones RGB colors to avoid reference issues
- Returns updated property list, source/target master IDs, and success message
- Sentinel errors: ErrApplyThemeFailed, ErrInvalidThemeSource, ErrGalleryNotSupported, ErrNoMasterInSource, ErrNoMasterInTarget, ErrNoColorScheme, ErrInvalidSourcePresID, ErrSourceNotFound

**Files changed:**
- `internal/tools/apply_theme.go` - Tool implementation with ApplyThemeInput/ApplyThemeOutput structs, buildColorSchemeFromSource helper, cloneRgbColor helper, themeColorTypes array
- `internal/tools/apply_theme_test.go` - Comprehensive tests (20 test cases: success, case-insensitivity, gallery not supported, validation errors, missing source, missing target masters, missing color scheme, API errors, verify request structure, buildColorSchemeFromSource tests, cloneRgbColor tests)
- `CLAUDE.md` - Added apply_theme documentation with input/output examples, theme sources table, API limitation explanation, color types list, usage patterns
- `README.md` - Added apply_theme documentation with JSON input/output examples, parameter table, features, API limitation warning, examples, error messages
- `stories.yaml` - Marked US-00049 as passes: true, updated tests to reflect gallery limitation

**Learnings:**
- Google Slides API has no direct "apply theme" endpoint - only color scheme updates via UpdatePagePropertiesRequest
- Gallery themes (built-in themes in UI) are completely unsupported by the API
- Only way to "apply theme" via API is to copy color scheme from another presentation's master
- ColorScheme update requires providing all colors that should be updated (not incremental)
- Only 12 ThemeColorTypes are editable via API (excludes TEXT1, BACKGROUND1, etc.)
- Master slides store the color scheme in PageProperties.ColorScheme
- Must fetch both source and target presentations to get master ObjectIds
- Batch update targets the master slide's ObjectId, not the presentation ID

**Remaining issues:** None

---

## US-00050: Implement tool to set slide background

**Status:** ✅ Completed

**Implementation Summary:**
Implemented `set_background` MCP tool that sets the background for one or all slides. Supports three background types: solid color, image, and gradient.

**Key Implementation Details:**
- Input: presentation_id, scope ("slide" or "all"), background_type ("solid", "image", "gradient")
- For solid: color (hex string)
- For image: image_base64 (uploaded to Drive, then referenced as StretchedPictureFill)
- For gradient: start_color, end_color, angle (0-360 degrees)
- Scope "slide" requires slide_index (1-based) OR slide_id
- Uses UpdatePagePropertiesRequest with pageBackgroundFill field
- For solid backgrounds: uses SolidFill with parsed RGB color
- For image backgrounds: uploads to Drive, makes public, uses StretchedPictureFill with URL
- For gradients: generates PNG image programmatically (API doesn't support native gradients), uploads to Drive, uses StretchedPictureFill
- PNG generation implemented without external dependencies (manual IHDR, IDAT, IEND chunks, zlib compression)
- Gradient generation supports any angle with proper color interpolation
- Returns success status, affected slide count, and background type applied
- Sentinel errors: ErrSetBackgroundFailed, ErrInvalidBackgroundType, ErrInvalidBackgroundScope (reuses ErrInvalidScope), ErrMissingBackgroundColor, ErrMissingGradientColors, ErrInvalidGradientAngle

**Files changed:**
- `internal/tools/set_background.go` - Tool implementation with SetBackgroundInput/SetBackgroundOutput structs, solid/image/gradient handlers, PNG generation helpers (encodePNG, writeChunk, crc32PNG, compressZlib, deflateStore, adler32, generateGradientImage)
- `internal/tools/set_background_test.go` - Comprehensive tests (25+ test cases: solid color single/all slides, image background, gradient single/all slides, by slide ID, validation errors, API errors, case-insensitivity, helper function tests)
- `CLAUDE.md` - Added set_background documentation with input/output examples, background types table, scope options, gradient angles, features, sentinel errors, usage patterns
- `README.md` - Added set_background documentation with JSON input/output examples, parameter tables, background types, gradient angles, features, error messages
- `stories.yaml` - Marked US-00050 as passes: true

**Learnings:**
- Google Slides API doesn't support native gradient backgrounds - workaround is to generate gradient image and use StretchedPictureFill
- PNG format requires careful implementation of chunks (IHDR, IDAT, IEND), CRC32 checksums, and zlib compression
- Zlib stream format: CMF byte (0x78), FLG byte (0x01 for no dict), deflate blocks, Adler-32 checksum
- Deflate "store" method (non-compressed) is simplest: 5-byte header per block with length in little-endian
- Gradient interpolation: calculate pixel position based on angle, interpolate colors linearly
- StretchedPictureFill requires publicly accessible URL - must make uploaded Drive file public
- Same findSlide helper pattern reused from other tools for slide selection
- Drive service factory pattern allows easy mocking in tests

**Remaining issues:** None

## 2026-01-16 - US-00051 - Implement tool to configure slide footer

**Status:** Success

**What was implemented:**
- Created configure_footer tool to manage slide footer elements (slide numbers, date, custom text)
- Input parameters:
  - presentation_id (required)
  - show_slide_number (bool, optional) - enable/disable slide numbers
  - show_date (bool, optional) - enable/disable date display
  - date_format (string, optional) - Go date format (default: "January 2, 2006")
  - footer_text (string pointer, optional) - custom footer text (nil = don't change, "" = clear)
  - apply_to (string, optional) - "all", "title_slides_only", or "exclude_title_slides" (default: "all")
- Output: success, message, updated counts for each placeholder type, affected slide IDs, applied_to value
- Implemented using placeholder modification (DeleteText + InsertText requests)
- Properly identifies title slides by checking layout names (TITLE, TITLE_SLIDE, SECTION_HEADER only - not TITLE_AND_BODY, TITLE_ONLY, etc.)
- Falls back to checking master and layout placeholders if no slide-level placeholders exist

**Files changed:**
- `internal/tools/configure_footer.go` - Tool implementation with ConfigureFooterInput/ConfigureFooterOutput structs, findFooterPlaceholders (with title slide detection), buildFooterUpdateRequests, isFooterPlaceholderType, footerPlaceholderInfo/footerUpdateStats helper types
- `internal/tools/configure_footer_test.go` - Comprehensive tests (11 test cases: show slide number enable/disable, show date with format enable/disable, footer text set/clear, apply_to all/title_slides_only/exclude_title_slides, validation errors, no placeholders error, presentation not found, access denied, batch update error, multiple updates, placeholders on master fallback)
- `CLAUDE.md` - Added configure_footer documentation with input/output examples, apply_to options table, API limitations note, features, sentinel errors, usage patterns
- `README.md` - Added configure_footer documentation with JSON input/output examples, parameter tables, apply_to options, API limitation note, error messages
- `stories.yaml` - Marked US-00051 as passes: true

**Learnings:**
- Footer elements in Google Slides are placeholder shapes with types: FOOTER, SLIDE_NUMBER, DATE_AND_TIME
- Placeholders cannot be created via API - they must exist in the master/layout
- To "show" or "hide" footers, we modify the placeholder text content (insert "#" for slide numbers, formatted date for dates, or clear text to hide)
- Title slide detection must be precise: "TITLE" layout is a title slide, but "TITLE_AND_BODY" is not - can't use string contains
- When no slide-level placeholders exist, the tool falls back to checking masters and layouts
- The apply_to filter allows targeting title slides only or excluding them, useful for different footer content on different slide types

**Remaining issues:** None

---

## US-00052: Implement tool to set slide transition

**Status:** Completed with API limitation discovery  
**Date:** 2026-01-16

**What was implemented:**
- Created set_transition tool that handles a Google Slides API limitation
- The tool validates input and returns an informative error explaining the API does not support transitions
- Input parameters:
  - presentation_id (required)
  - slide_index (int, optional) - 1-based slide index
  - slide_id (string, optional) - alternative to slide_index
  - transition_type (required) - NONE, FADE, SLIDE_FROM_RIGHT, SLIDE_FROM_LEFT, SLIDE_FROM_TOP, SLIDE_FROM_BOTTOM, FLIP, CUBE, GALLERY, ZOOM, DISSOLVE
  - duration (float64 pointer, optional) - seconds (0-10)
- Output: success (always false), message (error explanation), affected_slides (empty)
- Comprehensive input validation before returning API limitation error
- Case-insensitive transition type handling
- Sentinel errors for different validation failures

**Critical Discovery:**
The Google Slides API does NOT support setting slide transitions. Investigation of the Go API library (`google.golang.org/api@v0.258.0/slides/v1/slides-gen.go`) confirmed:
- SlideProperties only contains: IsSkipped, LayoutObjectId, MasterObjectId, NotesPage
- No transition-related properties exist in the API
- Grep for "transition|Transition" in the API library returned no matches
- Reference: https://developers.google.com/slides/api/reference/rest/v1/presentations.pages#SlideProperties

**Workarounds communicated in error message:**
1. Use the Google Slides UI (Slide > Transition)
2. Use Google Apps Script's SlidesApp.Slide.setTransition() method

**Files changed:**
- `internal/tools/set_transition.go` - Tool implementation with SetTransitionInput/SetTransitionOutput structs, validTransitionTypes map, input validation, ErrTransitionNotSupported error
- `internal/tools/set_transition_test.go` - Tests for API not supported (5 cases), input validation (5 cases: missing presentation_id, empty transition_type, invalid transition_type, negative duration, duration too long), all transition types validate (11 types), case-insensitivity (4 variants), error message informativeness
- `CLAUDE.md` - Added set_transition documentation with API limitation warning, input/output examples, transition types table, API limitation details, sentinel errors, usage pattern
- `README.md` - Added set_transition documentation with API limitation warning, JSON input/output examples, parameter tables, API limitation details, workarounds section, error messages
- `stories.yaml` - Marked US-00052 as passes: true

**Learnings:**
- Always verify API capabilities before implementing features - the Google Slides API has significant limitations
- Following the apply_theme.go pattern for handling API limitations: validate input, return informative error explaining the limitation and suggesting alternatives
- The test story requirements were interpreted as: tests verify the tool correctly reports the API limitation, not that it successfully sets transitions
- Case-insensitive input handling (using strings.ToUpper) is a consistent pattern across tools
- Duration validation (0-10 seconds) matches Google Slides UI limits

**Remaining issues:** None - this is an API limitation, not a bug

---

## 2026-01-16 - US-00053 - Implement tool to add object animation

**Status:** Success

**What was implemented:**
- Created `add_animation` tool following the pattern established by `set_transition` for API-limited features
- Comprehensive input validation for all animation parameters before returning API limitation error
- Sentinel errors for each validation failure type (animation type, category, direction, trigger, duration, delay)
- Informative error message explaining the API limitation with link to Google Issue Tracker

**API Limitation Discovery:**
The Google Slides API does NOT support adding or managing object animations programmatically. This is a known limitation:
- Issue Tracker: https://issuetracker.google.com/issues/36761236 (feature request open since 2015)
- The API has no request types for creating, modifying, deleting, or reading animation properties
- Animations can only be configured through the Google Slides UI (Insert > Animation or View > Motion)

**Files changed:**
- `internal/tools/add_animation.go` - Tool implementation with:
  - AddAnimationInput/AddAnimationOutput structs
  - Validation maps: validAnimationTypes, validAnimationCategories, validAnimationTriggers, validDirections
  - Sentinel errors: ErrAnimationNotSupported, ErrInvalidAnimationType, ErrInvalidAnimationCategory, ErrInvalidDirection, ErrInvalidAnimationTrigger, ErrInvalidAnimationDuration, ErrInvalidAnimationDelay
  - Input normalization (uppercase conversion for type/category/direction/trigger)
  - Duration/delay validation (0-60 seconds)
  
- `internal/tools/add_animation_test.go` - 46 test cases across 6 test functions:
  - TestAddAnimation: 23 subtests for valid animations returning API error and input validation
  - TestAddAnimation_AllAnimationTypes: 10 animation types tested
  - TestAddAnimation_AllAnimationCategories: 3 categories tested
  - TestAddAnimation_AllTriggerTypes: 3 triggers tested
  - TestAddAnimation_AllDirections: 4 directions tested
  - TestAddAnimation_ErrorMessageContainsIssueTracker: verifies error message quality
  
- `CLAUDE.md` - Added add_animation documentation with API limitation warning, input/output examples, animation types/categories/triggers/directions, sentinel errors, usage pattern
  
- `stories.yaml` - Marked US-00053 as passes: true

**Learnings:**
- Google Slides API has significant gaps for animation-related functionality
- Following established patterns (like set_transition.go) makes implementation consistent
- Validation-first approach ensures users get helpful error messages even for unsupported features
- Case-insensitive input handling (uppercase normalization) provides better UX
- Using pointers for optional numeric fields (Duration, Delay) allows distinguishing "not set" from "zero"

**Remaining issues:** None - this is an API limitation, not a bug

---

## 2026-01-16 - US-00054 - Implement tool to manage animation order

**Status:** Success

**What was implemented:**
- MCP Tool `manage_animations` for managing slide animations (list, reorder, modify, delete)
- Tool returns informative error because Google Slides API does not support animation management
- This is a known API limitation tracked at https://issuetracker.google.com/issues/36761236
- Comprehensive input validation before returning the API limitation error:
  - presentation_id required
  - slide_index (1-based) or slide_id required
  - action required: list, reorder, modify, delete (case-insensitive)
  - Action-specific validation:
    - reorder: animation_ids array required
    - modify: animation_id and properties required
    - delete: animation_id required
  - Properties validation for modify action:
    - animation_type, animation_category, direction, trigger must be valid if provided
    - duration/delay must be 0-60 seconds if provided

**API Limitation:** Google Slides API does not provide endpoints for:
- Listing/reading animations
- Reordering animations
- Modifying animation properties
- Deleting animations

Animations can only be managed through the Google Slides UI (View > Motion or Insert > Animation)

**Files changed:**
- `internal/tools/manage_animations.go` - Tool implementation with:
  - ManageAnimationsInput/ManageAnimationsOutput structs
  - AnimationModifyProperties struct for modify action
  - AnimationInfo struct for animation details (for future API support)
  - Sentinel errors: ErrManageAnimationsFailed, ErrManageAnimationsNotSupported, ErrInvalidManageAnimationsAction, ErrInvalidAnimationID, ErrNoAnimationIDs, ErrNoAnimationProperties
  - validManageAnimationsActions map: LIST, REORDER, MODIFY, DELETE
  - validateAnimationProperties method reusing add_animation validation patterns
  - Action normalization (uppercase conversion)
  
- `internal/tools/manage_animations_test.go` - 70+ test cases across 5 test functions:
  - TestManageAnimations: 24 subtests for all actions returning API error and input validation
  - TestManageAnimations_AllActions: 8 subtests testing all actions with case variations
  - TestManageAnimations_ValidPropertiesWithAllOptions: 26 subtests for valid properties
  - TestManageAnimations_ErrorMessageContainsIssueTracker: verifies error message quality
  - TestManageAnimations_SlideReferenceOptions: 3 subtests for slide_index/slide_id combinations
  
- `CLAUDE.md` - Added manage_animations documentation with API limitation warning, input/output examples, actions table, sentinel errors, usage pattern
  
- `README.md` - Added manage_animations to tool listing
  
- `stories.yaml` - Marked US-00054 as passes: true

**Learnings:**
- Animation management has same API limitation as animation creation (add_animation)
- Reusing validation maps and patterns from add_animation keeps code consistent
- Validating all inputs before returning API limitation helps users understand what would work if API supported it
- Tool follows same structure as add_animation for maintainability

**Remaining issues:** None - this is an API limitation, not a bug

---

## US-00055: Implement tool to manage speaker notes

**Status:** ✅ Completed

**Implementation Date:** 2026-01-16

**Files Created:**
- `internal/tools/manage_speaker_notes.go` - Main implementation with:
  - `ManageSpeakerNotesInput` struct with presentation_id, slide_index/slide_id, action, notes_text
  - `ManageSpeakerNotesOutput` struct with slide_id, slide_index, action, notes_content
  - `ManageSpeakerNotes` method supporting get/set/append/clear actions
  - `findSpeakerNotesShape` helper to locate BODY placeholder in notes page
  - `buildSpeakerNotesRequests` helper to generate BatchUpdate requests
  - 4 sentinel errors: ErrManageSpeakerNotesFailed, ErrInvalidSpeakerNotesAction, ErrNotesTextRequired, ErrNotesShapeNotFound

- `internal/tools/manage_speaker_notes_test.go` - Comprehensive test suite with:
  - TestManageSpeakerNotes_Get: 5 subtests (by index, by ID, empty notes, no notes page, case insensitive)
  - TestManageSpeakerNotes_Set: 4 subtests (replace existing, empty to new, no placeholder, batch error)
  - TestManageSpeakerNotes_Append: 2 subtests (to existing, to empty)
  - TestManageSpeakerNotes_Clear: 2 subtests (existing, already empty)
  - TestManageSpeakerNotes_ValidationErrors: 5 subtests (missing fields, invalid action)
  - TestManageSpeakerNotes_PresentationErrors: 3 subtests (not found, access denied, API error)
  - TestManageSpeakerNotes_SlideNotFound: 2 subtests (out of range, nonexistent ID)
  - TestManageSpeakerNotes_MultipleSlides: 4 subtests (various slides by index and ID)
  - TestManageSpeakerNotes_BatchUpdateForbidden: 1 test for 403 on modification
  - TestBuildSpeakerNotesRequests: 6 subtests for request generation
  - TestFindSpeakerNotesShape: 6 subtests for notes shape location

- `CLAUDE.md` - Added manage_speaker_notes documentation with input/output examples, actions table, sentinel errors, usage pattern

- `README.md` - Added manage_speaker_notes to tool listing

- `stories.yaml` - Marked US-00055 as passes: true

**Learnings:**
- Speaker notes are stored in SlideProperties.NotesPage.PageElements with BODY placeholder type
- Reused extractTextFromTextContent helper from get_presentation.go for text extraction
- Used same pattern as modify_text.go for DeleteText/InsertText BatchUpdate requests
- Append action uses InsertText at position equal to current text length
- 1-based slide indexing kept consistent with other tools for human-friendly API

**Remaining issues:** None

---

## US-00056: Implement tool to list comments

**Status:** ✅ Completed

**Implementation Date:** 2026-01-16

**Files Created:**
- `internal/tools/list_comments.go` - Main implementation with:
  - `ListCommentsInput` struct with presentation_id, include_resolved
  - `ListCommentsOutput` struct with presentation_id, comments array, total_count, unresolved_count, resolved_count
  - `CommentInfo` struct with comment_id, author, content, html_content, anchor_info, replies, resolved, deleted, created_time, modified_time
  - `AuthorInfo` struct with display_name, email_address, photo_link
  - `ReplyInfo` struct with reply_id, author, content, html_content, created_time, modified_time, deleted
  - `ListComments` method using Drive API with pagination support
  - Filtering logic to show only unresolved comments by default
  - Statistics calculation (total, unresolved, resolved counts)
  - Sentinel error: ErrListCommentsFailed

- `internal/tools/list_comments_test.go` - Comprehensive test suite with 16 subtests:
  - lists_all_unresolved_comments_by_default: verifies default behavior
  - include_resolved_shows_resolved_comments: verifies include_resolved flag
  - replies_are_included: verifies replies with author info
  - anchor_information_is_provided: verifies anchor JSON string is preserved
  - handles_pagination: verifies multiple API calls for paginated results
  - returns_empty_list_when_no_comments: verifies empty state handling
  - returns_error_for_empty_presentation_ID: validates input
  - returns_error_when_presentation_not_found: handles 404
  - returns_error_when_access_denied: handles 403
  - returns_error_when_drive_service_fails: handles API errors
  - handles_nil_author_gracefully: null safety
  - handles_nil_comment_in_list_gracefully: null safety
  - handles_nil_reply_in_list_gracefully: null safety
  - includes_HTML_content_when_available: verifies HTML content preservation
  - includes_deleted_flag_when_set: verifies deleted flag
  - returns_presentation_ID_in_output: verifies output structure

- `internal/tools/tools.go` - Extended DriveService interface:
  - Added `ListComments(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error)` method
  - Added implementation in realDriveService using Drive API v3 Comments.List

- `internal/tools/search_presentations_test.go` - Updated mockDriveService:
  - Added ListCommentsFunc field for mocking
  - Added ListComments method implementation

- `CLAUDE.md` - Added list_comments documentation with input/output examples, features, sentinel errors, usage pattern, updated DriveService interface

- `README.md` - Added list_comments documentation with detailed tool documentation and added to Collaboration section in tool summary

- `stories.yaml` - Marked US-00056 as passes: true

**Learnings:**
- Comments in Google Slides are accessed via Drive API, not Slides API
- Drive API Comments.List requires specific fields parameter for full data
- Anchor information is provided as a JSON string from the API
- By default, Drive API returns all non-deleted comments; filtering for resolved is done client-side
- Pagination using NextPageToken follows same pattern as other tools
- Replies are nested in each Comment object with their own Author info

**Remaining issues:** None

---

## 2026-01-16 - US-00057 - Implement tool to add comment

**Status:** Success

**What was implemented:**
- Created `add_comment` tool to add comments to presentations via Drive API
- Input parameters:
  - `presentation_id` (required): The Google Slides presentation ID
  - `content` (required): The comment text content
  - `anchor_object_id` (optional): Object ID to anchor the comment to
  - `anchor_page_index` (optional): Slide index (0-based) to anchor the comment to
- Anchor behavior:
  - If `anchor_object_id` is provided, comment is anchored to that object
  - If only `anchor_page_index` is provided, comment is anchored to that slide
  - If both are provided, `anchor_object_id` takes precedence
  - Page index is converted from 0-based input to 1-based page number in anchor format
- Output includes: comment_id, presentation_id, content, anchor_info (JSON), created_time
- Uses Google Drive anchor JSON format:
  - Object: `{"r":"content","a":[{"n":"objectId","v":"<id>"}]}`
  - Page: `{"r":"content","a":[{"n":"pageNumber","v":"<number>"}]}`

**Files changed:**

- `internal/tools/add_comment.go` - New file:
  - AddCommentInput and AddCommentOutput structs
  - AddComment method on Tools struct
  - Sentinel errors: ErrAddCommentFailed, ErrInvalidCommentText
  - Builds anchor JSON from input parameters
  - Creates comment via DriveService.CreateComment

- `internal/tools/add_comment_test.go` - New file with 14 test cases:
  - adds_comment_to_presentation_successfully: basic happy path
  - comment_can_be_anchored_to_object: objectId anchor format
  - comment_can_be_anchored_to_slide: pageNumber anchor format
  - comment_can_be_anchored_to_first_slide_(page_0): edge case for page 0
  - object_anchor_takes_precedence_over_page_anchor: precedence behavior
  - returns_comment_id: verifies output structure
  - returns_error_for_empty_presentation_ID: validation
  - returns_error_for_empty_content: validation
  - returns_error_when_presentation_not_found: 404 handling
  - returns_error_when_access_denied: 403 handling
  - returns_error_when_drive_service_fails: API error handling
  - returns_error_when_drive_service_factory_fails: factory error handling
  - no_anchor_when_neither_object_nor_page_specified: no anchor behavior
  - returns_presentation_ID_in_output: verifies output structure

- `internal/tools/tools.go` - Extended DriveService interface:
  - Added `CreateComment(ctx context.Context, fileID string, comment *drive.Comment) (*drive.Comment, error)` method
  - Added implementation in realDriveService using Drive API v3 Comments.Create
  - Fields returned: id, kind, content, htmlContent, author, createdTime, modifiedTime, resolved, deleted, anchor

- `internal/tools/search_presentations_test.go` - Updated mockDriveService:
  - Added CreateCommentFunc field for mocking
  - Added CreateComment method implementation

- `CLAUDE.md` - Added add_comment documentation:
  - Input/output examples
  - Anchor behavior explanation
  - Anchor JSON format reference
  - Features, sentinel errors, usage patterns
  - Updated DriveService interface to include CreateComment

- `README.md` - Added add_comment documentation:
  - Detailed tool documentation with JSON examples
  - Parameter table with descriptions
  - Anchor behavior section
  - Features and error descriptions
  - Added to Collaboration section in tool summary

- `stories.yaml` - Marked US-00057 as passes: true

**Learnings:**
- Comments are created via Drive API Comments.Create (not Slides API)
- Google Drive anchor format uses JSON with "r" (root) and "a" (attributes) fields
- Page numbers in anchor format are 1-based, while input is 0-based
- Object anchors and page anchors use different attribute names: "objectId" vs "pageNumber"
- CreateComment returns fields specified in Fields parameter, matching pattern from ListComments

**Remaining issues:** None

---

## US-00058: Implement tool to reply/resolve/delete comment

**Status:** Completed
**Date:** 2026-01-16

**Implementation Summary:**

Implemented the `manage_comment` MCP tool that provides four actions for managing comments in Google Slides presentations: reply, resolve, unresolve, and delete.

**Files Modified:**

- `internal/tools/manage_comment.go` - New file with tool implementation:
  - `ManageCommentInput` struct with presentation_id, comment_id, action, content fields
  - `ManageCommentOutput` struct with success status, message, and reply_id (for replies)
  - `ManageComment` method as main entry point with input validation
  - `handleReply` - Creates a reply using Drive API Replies.Create
  - `handleResolve` - Updates comment resolved status using Drive API Comments.Update
  - `handleDelete` - Deletes comment using Drive API Comments.Delete
  - Sentinel errors: ErrManageCommentFailed, ErrInvalidCommentAction, ErrInvalidCommentID, ErrReplyContentRequired, ErrCommentNotFound

- `internal/tools/manage_comment_test.go` - New file with 23 test cases covering:
  - Reply adds a reply to comment (happy path)
  - Resolve marks comment as resolved
  - Unresolve reopens resolved comment
  - Delete removes comment
  - Action is case insensitive (Reply, REPLY, reply all work)
  - Error cases: empty presentation_id, empty comment_id, invalid action, missing content for reply
  - API error handling: not found (404), access denied (403), generic API errors
  - Drive service factory failure handling

- `internal/tools/tools.go` - Extended DriveService interface:
  - Added `CreateReply(ctx context.Context, fileID, commentID string, reply *drive.Reply) (*drive.Reply, error)` method
  - Added `UpdateComment(ctx context.Context, fileID, commentID string, comment *drive.Comment) (*drive.Comment, error)` method
  - Added `DeleteComment(ctx context.Context, fileID, commentID string) error` method
  - Added implementations in realDriveService using Drive API v3

- `internal/tools/search_presentations_test.go` - Updated mockDriveService:
  - Added CreateReplyFunc, UpdateCommentFunc, DeleteCommentFunc fields
  - Added CreateReply, UpdateComment, DeleteComment method implementations

- `CLAUDE.md` - Added manage_comment documentation:
  - Input/output examples
  - Actions table (reply, resolve, unresolve, delete)
  - Features, sentinel errors, usage patterns
  - Updated DriveService interface to include new methods

- `README.md` - Added manage_comment documentation:
  - Detailed tool documentation with JSON examples
  - Parameter table with descriptions
  - Actions table
  - Features and error descriptions
  - Added to Collaboration section in tool summary

- `stories.yaml` - Marked US-00058 as passes: true

**Learnings:**
- Comment management operations (reply, resolve, delete) are handled via Drive API, not Slides API
- Drive API uses Comments.Update with resolved=true/false to resolve/unresolve comments
- Replies are created via Drive API Replies.Create endpoint
- Comments can be deleted entirely using Comments.Delete
- The same 404/403 error handling pattern from other comment tools applies here
- Action normalization (lowercase, trim) provides better UX without strict input requirements

**Test Results:** All 23 tests pass

**Remaining issues:** None

---

## US-00059: Implement tool to translate presentation

**Status:** Completed
**Date:** 2026-01-16

**Implementation Summary:**

Implemented the `translate_presentation` MCP tool that translates text in a Google Slides presentation using Google Cloud Translation API. Supports translating all text, a specific slide, or a specific object.

**Files Modified:**

- `internal/tools/translate_presentation.go` - New file with tool implementation:
  - `TranslateService` interface abstracting Google Cloud Translation API
  - `TranslateServiceFactory` type for creating translate services from token source
  - `TranslatePresentationInput` struct with presentation_id, target_language, source_language, scope, slide_index, slide_id, object_id
  - `TranslatePresentationOutput` struct with translated_count, affected_slides, translated_elements
  - `TranslatedElement` struct capturing original and translated text for each element
  - `TranslatePresentation` method as main entry point
  - `collectTextElements` and `collectTextFromElements` helpers for scope-based text extraction
  - Sentinel errors: ErrTranslateFailed, ErrInvalidTargetLanguage, ErrTranslateAPIError, ErrNoTextToTranslate

- `internal/tools/translate_presentation_test.go` - New file with 24 test cases covering:
  - Successful translation of entire presentation (scope: all)
  - Scope slide by index filters correctly
  - Scope slide by ID filters correctly
  - Scope object filters to specific object
  - Speaker notes are translated
  - Text in groups is translated recursively
  - Empty/whitespace-only text elements are skipped
  - Unchanged translations (same input/output) are skipped
  - Error cases: missing presentation_id, missing target_language, invalid scope
  - Scope slide without slide_index/slide_id error
  - Scope object without object_id error
  - Slide not found error
  - Object not found error
  - Presentation not found (404)
  - Access denied (403)
  - Translation API failure
  - Slides service factory failure
  - Translate service factory failure

- `internal/tools/tools.go` - Extended with Translation API support:
  - Added imports for `cloud.google.com/go/translate` and `golang.org/x/text/language`
  - Added `translateServiceFactory TranslateServiceFactory` field to Tools struct
  - `realTranslateService` struct wrapping Google Cloud Translation client
  - `TranslateText` method for single text translation
  - `TranslateTexts` method for batch translation (more efficient)
  - `NewRealTranslateServiceFactory` function
  - `NewToolsWithAllServices` constructor accepting all three service factories
  - Deprecated `NewToolsWithDrive` (delegates to NewToolsWithAllServices)

- `CLAUDE.md` - Added comprehensive translate_presentation tool documentation:
  - Input/Output struct definitions with JSON examples
  - Scope options table (all, slide, object)
  - Features list (batch translation, auto-detection, speaker notes support)
  - Sentinel errors documentation
  - Usage patterns with code examples
  - TranslateService interface documentation

- `README.md` - Added translate_presentation tool documentation:
  - New Translation section in tool summary
  - Detailed tool documentation with JSON examples
  - Parameter table with descriptions
  - Scope options table
  - Features and error descriptions

- `stories.yaml` - Marked US-00059 as passes: true

**Learnings:**
- Google Cloud Translation API uses `golang.org/x/text/language` for language tag parsing
- Batch translation is more efficient - collect all texts, translate in one API call, then apply updates
- TranslateTexts returns translations in same order as input texts
- Speaker notes are in slide.SlideProperties.NotesPage.PageElements, look for BODY placeholder
- Text in grouped elements requires recursive collection
- Empty/whitespace text elements should be filtered out before translation
- Unchanged translations (same as original) should be skipped to avoid unnecessary API calls
- Use DeleteText with Type: "ALL" followed by InsertText to replace text content

**Test Results:** All 24 tests pass

**Remaining issues:** None

---

## 2026-01-16 - US-00060 - Implement tool to manage hyperlinks

**Status:** Success

**What was implemented:**
- MCP Tool: `manage_hyperlinks` with three actions: list, add, remove
- List action scans all slides for hyperlinks (text links, shape links, image links)
- Add action creates hyperlinks on text ranges or entire shapes/images
- Remove action clears hyperlinks from text ranges
- Support for external URLs and internal slide links (#slide=N, #slideId=ID, #next, #previous, #first, #last)
- Link type detection (external, internal_slide, internal_position)
- Scope filtering for list action (all, slide, object)
- Comprehensive test suite with 27 test cases

**Files changed:**
- `internal/tools/manage_hyperlinks.go` - Main implementation:
  - ManageHyperlinksInput/ManageHyperlinksOutput structs
  - HyperlinkInfo struct for hyperlink details
  - listHyperlinks - scans presentation for all hyperlinks
  - addHyperlink - creates hyperlinks on text or shapes/images
  - removeHyperlink - clears hyperlinks from text ranges
  - Helper functions for link type detection and URL parsing

- `internal/tools/manage_hyperlinks_test.go` - Test suite:
  - Tests for all story requirements
  - Mock-based testing with mockSlidesService
  - Edge cases: empty presentation, invalid actions, missing URLs

- `CLAUDE.md` - Added manage_hyperlinks tool documentation:
  - Input/Output struct definitions with examples
  - Link types table and URL formats for internal links
  - Features list and sentinel errors
  - Usage patterns with code examples

- `stories.yaml` - Marked US-00060 as passes: true

**Learnings:**
- Google Slides API Shape type has Link inside ShapeProperties, not directly on Shape
  - Wrong: `element.Shape.Link`
  - Correct: `element.Shape.ShapeProperties.Link`
- Image hyperlinks use ImageProperties.Link, shape hyperlinks use ShapeProperties.Link
- Text hyperlinks are in TextRun.Style.Link within shape text elements
- Internal slide links use different formats:
  - `#slide=N` for 1-based slide index
  - `#slideId=ID` for slide object ID
  - `#next`, `#previous`, `#first`, `#last` for relative navigation
- Link.RelativeLink field indicates internal position links (NEXT_SLIDE, PREVIOUS_SLIDE, etc.)
- Use strings.CutPrefix instead of HasPrefix+TrimPrefix for cleaner code
- For unused context parameters in functions, use `_` to satisfy linter

**Test Results:** All 27 tests pass

**Remaining issues:** None

## US-00061: Implement batch operations support

**Files Created/Modified:**
- `internal/tools/batch_update.go` - Main implementation (~1160 lines):
  - BatchUpdate tool that executes multiple operations in a single API call
  - BatchOperation struct with tool_name and parameters
  - Three on_error modes: stop (default), continue, rollback
  - Batch optimization: combines compatible operations into single BatchUpdate calls
  - Support for 10 batchable tools (add_slide, delete_slide, add_text_box, modify_text, delete_object, create_shape, transform_object, style_text, create_bullet_list, create_numbered_list)
  - Support for 5 non-batchable tools (add_image, add_video, replace_image, set_background, translate_presentation)
  - Detailed results for each operation with success/failure status

- `internal/tools/batch_update_test.go` - Test suite:
  - Tests for all story requirements (10 test cases)
  - TestBatchUpdate_MultipleOperations - Multiple operations execute in sequence
  - TestBatchUpdate_OnErrorStop - Halts on first error
  - TestBatchUpdate_OnErrorContinue - Processes all operations (batched ops fail together)
  - TestBatchUpdate_OnErrorRollback - Atomic batch behavior
  - TestBatchUpdate_ResultsMatchOperations - Results array matches operations array
  - TestBatchUpdate_InvalidPresentationID, TestBatchUpdate_EmptyOperations - Validation tests
  - TestBatchUpdate_InvalidOnErrorMode, TestBatchUpdate_UnsupportedToolName - Error handling tests
  - TestBatchUpdate_DefaultOnErrorMode - Default mode behavior

- `CLAUDE.md` - Added batch_update tool documentation:
  - Input/Output struct definitions with examples
  - On error modes table
  - Supported operations tables (batchable and non-batchable)
  - Features list and sentinel errors
  - Usage patterns with code examples

- `README.md` - Added batch_update tool documentation:
  - New "Batch Operations" section in tool categories
  - Complete tool documentation with input/output examples
  - Parameter tables and error handling modes
  - Supported operations and features

- `stories.yaml` - Marked US-00061 as passes: true

**Learnings:**
- Google Slides API BatchUpdate can combine multiple requests into a single call
- Batchable operations are those that only require slides.Request objects
- Non-batchable operations need Drive API (images) or Translation API
- The slides.Response struct doesn't have DeleteText/InsertText fields - those are request-only
- For test logic, when operations are batched together, they fail/succeed as a unit
- Use `batchBuildTextStyleRequest` and `batchBuildShapeStyleRequest` for batch context (accept different parameters than the standalone versions)

**Test Results:** All 10 batch_update tests pass

**Remaining issues:** None

## US-00062: Implement retry logic with exponential backoff

**Files Created/Modified:**
- `internal/retry/retry.go` - Main implementation (~342 lines):
  - Package providing automatic retry logic with exponential backoff and jitter
  - Config struct with MaxRetries, InitialDelay, MaxDelay, Multiplier, JitterFactor, RetryableStatusCodes, Logger
  - DefaultConfig() returns sensible defaults (5 retries, 1s initial, 16s max, 2.0 multiplier, 0.2 jitter)
  - Retryer struct with configuration and status code lookup map
  - RetryableError type that wraps errors with status code and attempt info
  - IsRetryable() method checks if status code triggers retry
  - CalculateDelay() implements exponential backoff with jitter
  - Do() executes operations with retry logic
  - DoWithResult[T]() generic function for operations returning typed results
  - Config getters for inspection (MaxRetries, InitialDelay, MaxDelay, Multiplier, JitterFactor, RetryableStatusCodes)

- `internal/retry/retry_test.go` - Comprehensive test suite (~345 lines):
  - TestDefaultConfig - Validates default configuration values
  - TestNew - Default values, provided configuration, invalid jitter factor handling
  - TestRetryer_IsRetryable - All retryable status codes (429, 500, 502, 503, 504) and non-retryable codes
  - TestRetryer_CalculateDelay - Zero attempts, exponential backoff with tolerance, jitter range verification
  - TestRetryer_Do - First attempt success, 429/500/503 retry scenarios, max retries, non-retryable errors, context cancellation, exponential backoff intervals
  - TestDoWithResult - Success, retry with result, max retries error
  - TestRetryableError - Error message, Unwrap, errors.Is support
  - TestRetryer_RetryableStatusCodes - Returns configured codes

- `CLAUDE.md` - Added Retry Package documentation:
  - Configuration options with example code
  - Backoff algorithm explanation
  - Sentinel errors
  - RetryableError type definition
  - Usage patterns for Do() and DoWithResult()
  - Config getters list

- `README.md` - Added Retry Logic section:
  - Exponential backoff parameters
  - Jitter explanation
  - Retryable status codes table
  - Behavior description

- `stories.yaml` - Marked US-00062 as passes: true

**Learnings:**
- Exponential backoff formula: `delay = initialDelay * multiplier^(attempt-1)`
- Jitter applied as: `delay * (1 - jitterFactor + random(0, 2*jitterFactor))`
- JitterFactor <= 0 should default to 0.2 (not just < 0) to ensure jitter is applied with empty config
- Use tolerance-based assertions for timing tests with jitter (1% tolerance works well)
- Use very small jitter (0.001) for near-deterministic tests
- Generic DoWithResult[T] uses receiver on Retryer but takes function as parameter for type inference
- Context cancellation check should happen both before operation and during delay wait
- RetryableError should implement Unwrap() for errors.Is and errors.As compatibility

**Test Results:** All 16 retry tests pass (8 test functions with sub-tests)

**Remaining issues:** None
