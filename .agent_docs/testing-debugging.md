# Testing & Debugging Guide

## Running Tests

```bash
# Run all unit tests
make test

# Run integration tests (requires credentials)
make test-integration

# Run tests with verbose output
go test -v ./...

# Run specific package tests
go test ./internal/tools/... -v

# Run specific test function
go test ./internal/tools/... -run TestGetPresentation -v

# Run tests with race detection
go test -race ./...

# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

---

## Integration Tests

Integration tests verify end-to-end functionality with real Google APIs. They are located in `internal/integration/`.

### Running Integration Tests

```bash
# Using Makefile target
make test-integration

# Or manually
INTEGRATION_TEST=1 go test -v -timeout 10m ./internal/integration/...
```

### Required Environment Variables

| Variable | Description |
|----------|-------------|
| `INTEGRATION_TEST` | Set to "1" to enable integration tests |
| `GOOGLE_CLIENT_ID` | OAuth2 client ID |
| `GOOGLE_CLIENT_SECRET` | OAuth2 client secret |
| `GOOGLE_REFRESH_TOKEN` | Valid refresh token for testing |
| `TEST_PRESENTATION_ID` | (Optional) Existing presentation ID for read-only tests |
| `GOOGLE_PROJECT_ID` | (Optional) GCP project ID for Firestore tests |

### Test Categories

| File | Tests |
|------|-------|
| `auth_test.go` | OAuth2 flow, token refresh |
| `presentation_test.go` | Create, get, copy, search, export presentations |
| `slide_test.go` | Add, delete, duplicate, reorder slides |
| `object_test.go` | Text boxes, tables, transforms, styling |

### Test Fixtures

The `Fixtures` struct manages test resources:

```go
func TestSomething(t *testing.T) {
    SkipIfNoIntegration(t)
    config := LoadConfig(t)
    fixtures := NewFixtures(t, config)

    // Create test presentation (auto-cleaned up)
    pres := fixtures.CreateTestPresentation("Test - My Feature")

    // Use fixtures.TokenSource() for API calls
    // Cleanup happens automatically via t.Cleanup()
}
```

### Skipping Without Credentials

Tests automatically skip when credentials are missing:

```
=== RUN   TestGetPresentation_LoadsExistingPresentation
    presentation_test.go:14: Missing required environment variables
--- SKIP: TestGetPresentation_LoadsExistingPresentation (0.00s)
```

---

## Test Organization

### Table-Driven Tests

Standard pattern for all tests:

```go
func TestSomething(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        want    OutputType
        wantErr error
    }{
        {
            name:  "valid input",
            input: InputType{Field: "value"},
            want:  OutputType{Result: "expected"},
        },
        {
            name:    "invalid input",
            input:   InputType{Field: ""},
            wantErr: ErrInvalidInput,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Function(tt.input)
            if !errors.Is(err, tt.wantErr) {
                t.Errorf("got error %v, want %v", err, tt.wantErr)
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Mock Interfaces

- Define interfaces for external dependencies (SlidesService, DriveService)
- Create mock implementations in test files
- Use factory pattern to inject mocks in tests

### Test Naming

- Test functions: `TestFunctionName` or `TestType_MethodName`
- Subtests: Descriptive names like `"valid input"`, `"missing required field"`
- Test files: `*_test.go` in the same package

---

## Test Patterns

### Service Factory Mocking

```go
// Production
slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
    return slides.NewService(ctx, option.WithTokenSource(ts))
}

// Test
mockFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
    return &mockSlidesService{...}, nil
}
```

### Error Testing

```go
if !errors.Is(err, ErrExpectedError) {
    t.Errorf("got error %v, want %v", err, ErrExpectedError)
}
```

---

## Debugging Tips

### Structured Logging

Use `log/slog` for structured logging:

```go
import "log/slog"

// Create logger with context
logger := slog.Default()

// Log with structured fields
logger.Info("processing request",
    "presentation_id", id,
    "user_email", email,
)

// Log errors with context
logger.Error("failed to get presentation",
    "error", err,
    "presentation_id", id,
)
```

### Google API Error Inspection

Google API errors contain useful details:

```go
import "google.golang.org/api/googleapi"

if apiErr, ok := err.(*googleapi.Error); ok {
    logger.Error("google api error",
        "code", apiErr.Code,
        "message", apiErr.Message,
        "details", apiErr.Details,
    )
}
```

### Request/Response Debugging

Enable verbose HTTP logging for API calls:

```go
import (
    "net/http"
    "net/http/httputil"
)

// Debug HTTP client
transport := &debugTransport{Base: http.DefaultTransport}
client := &http.Client{Transport: transport}

type debugTransport struct {
    Base http.RoundTripper
}

func (t *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    dump, _ := httputil.DumpRequest(req, true)
    log.Printf("Request:\n%s", dump)

    resp, err := t.Base.RoundTrip(req)
    if resp != nil {
        dump, _ := httputil.DumpResponse(resp, true)
        log.Printf("Response:\n%s", dump)
    }
    return resp, err
}
```

---

## Common Issues and Solutions

| Issue | Cause | Solution |
|-------|-------|----------|
| "token expired" | OAuth2 refresh token expired or revoked | User needs to re-authenticate via /auth endpoint |
| "permission denied" | User doesn't have access to the presentation | Check file permissions in Google Drive |
| "rate limit exceeded" | Too many API requests in short time | Implement backoff or wait for Retry-After header |
| "invalid presentation ID" | Presentation ID format incorrect or file doesn't exist | Verify ID format (typically alphanumeric ~44 chars) |
| "slide index out of range" | Slide index (1-based) exceeds slide count | Use list_slides to verify slide count first |

---

## Inspecting Presentation Structure

Use get_presentation to understand the document structure:

```json
{
  "presentation_id": "abc123",
  "slides": [
    {
      "index": 1,
      "object_id": "slide_1",
      "objects": [
        {"object_id": "shape_1", "object_type": "TEXT_BOX"},
        {"object_id": "image_2", "object_type": "IMAGE"}
      ]
    }
  ]
}
```

---

## Cache Debugging

Check cache metrics:

```go
stats := cacheManager.Stats()
fmt.Printf("Presentations: hits=%d, misses=%d, rate=%.1f%%\n",
    stats.Presentations.Hits,
    stats.Presentations.Misses,
    stats.Presentations.HitRate(),
)
```

---

## Test Debugging

Run single test with verbose output:

```bash
go test -v -run TestSpecificFunction ./internal/tools/...
```

Print values during tests:

```go
t.Logf("got: %+v", result)
t.Logf("want: %+v", expected)
```

---

## Testing Locally

1. Set up OAuth2 credentials in Secret Manager
2. Run `make run`
3. Visit `http://localhost:8080/auth` to authenticate
4. Use returned API key for tool calls

---

## Common Test Patterns

### Interface-Based Design

All external services use interfaces for testability:

```go
// Define interface where it's used
type SlidesService interface {
    GetPresentation(ctx context.Context, id string) (*slides.Presentation, error)
    BatchUpdate(ctx context.Context, id string, req *slides.BatchUpdatePresentationRequest) (*slides.BatchUpdatePresentationResponse, error)
}

// Factory function for dependency injection
type SlidesServiceFactory func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error)
```

### Sentinel Errors

Define package-level errors for expected conditions:

```go
var (
    ErrPresentationNotFound = errors.New("presentation not found")
    ErrAccessDenied         = errors.New("access denied")
    ErrInvalidInput         = errors.New("invalid input")
)

// Usage
if err != nil {
    if isNotFoundError(err) {
        return nil, ErrPresentationNotFound
    }
    return nil, fmt.Errorf("failed to get presentation: %w", err)
}
```

### Context Propagation

Always pass context as first parameter:

```go
func (t *Tools) GetPresentation(ctx context.Context, tokenSource oauth2.TokenSource, input GetPresentationInput) (*GetPresentationOutput, error) {
    slidesService, err := t.slidesFactory(ctx, tokenSource)
    if err != nil {
        return nil, fmt.Errorf("create slides service: %w", err)
    }
    // Use ctx in all API calls
}
```

### Input Validation Pattern

Validate inputs at the start of functions:

```go
func (t *Tools) SomeOperation(ctx context.Context, input SomeInput) (*SomeOutput, error) {
    // Validate required fields first
    if input.PresentationID == "" {
        return nil, ErrInvalidPresentationID
    }
    if input.SlideIndex == 0 && input.SlideID == "" {
        return nil, ErrInvalidSlideReference
    }

    // Proceed with operation
}
```

### EMU Conversion

Google Slides uses EMU (English Metric Units):

```go
const pointsToEMU = 12700 // 1 point = 12700 EMU

// Convert points to EMU
func toEMU(points float64) float64 {
    return points * pointsToEMU
}

// Convert EMU to points
func toPoints(emu float64) float64 {
    return emu / pointsToEMU
}
```

### Error Wrapping

Add context when propagating errors:

```go
// Good - adds context
return nil, fmt.Errorf("get presentation %s: %w", id, err)

// Bad - loses context
return nil, err
```

### Cache Key Generation

Use consistent key formats for caching:

```go
// User + resource specific caching
key := fmt.Sprintf("%s:%s", userEmail, presentationID)

// Type-prefixed keys
tokenKey := "token:" + apiKey
permKey := "perm:" + userEmail + ":" + fileID
```

---

## Google Slides API Gotchas

Critical learnings from implementation:

### EMU Units
- All sizes/positions in API use EMU (English Metric Units): `1 point = 12700 EMU`
- Use `Transform.TranslateX/Y` for position, `Size.Width/Height` for dimensions
- Standard slide: 720 x 405 points

### Text Operations
- `TextRange` with Type `"ALL"` styles entire text; `"FIXED_RANGE"` requires StartIndex/EndIndex
- `Range.StartIndex` and `Range.EndIndex` are `*int64` pointers, not `int64` values
- Partial text replacement: delete first (if range has content), then insert at start position
- For tables: use `CellLocation` field on text operations to target specific cells

### Transforms
- Affine transform matrix: `[ScaleX ShearX TranslateX; ShearY ScaleY TranslateY; 0 0 1]`
- Decomposing: `Sx = sqrt(ScaleX² + ShearY²)`, rotation = `atan2(ShearY, ScaleX)`
- `Size` property in PageElement is read-only; resizing must use Transform scale factors
- `UpdatePageElementTransformRequest` with `ABSOLUTE` mode sets exact values

### Shape Properties
- Shape hyperlinks are in `element.Shape.ShapeProperties.Link`, NOT `element.Shape.Link`
- Image hyperlinks use `ImageProperties.Link`
- Text hyperlinks are in `TextRun.Style.Link` within shape text elements

### Table Operations
- `InsertTableRowsRequest` uses `InsertBelow` boolean and `Number` for count
- Delete operations must be done from highest index to lowest to avoid shifting
- `CellLocation` uses `RowIndex` or `ColumnIndex` depending on operation
- Vertical alignment uses `UpdateTableCellPropertiesRequest` with `TableRange`

### Video Properties
- API uses `AutoPlay` (camelCase), not `Autoplay`
- Times are stored in milliseconds internally; convert to/from seconds for users

### Comments (via Drive API)
- Comments are accessed via Drive API Comments endpoints, not Slides API
- Anchor format: `{"r":"content","a":[{"n":"objectId","v":"<id>"}]}`
- Page numbers in anchor are 1-based

### API Limitations
These operations are NOT supported by the Google Slides API:
- `set_transition` - No transition properties in SlideProperties
- `add_animation` / `manage_animations` - Issue tracker: https://issuetracker.google.com/issues/36761236
- `apply_theme` (gallery) - Gallery themes unsupported; can only copy color schemes between presentations

### Middleware Pattern

Chain middleware functions:

```go
func (s *Server) withMiddleware(handler http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Rate limiting
        if s.rateLimitMiddleware != nil {
            if !s.rateLimitMiddleware.Allow(r) {
                s.rateLimitMiddleware.WriteRateLimitResponse(w)
                return
            }
        }

        // API key validation
        if s.apiKeyMiddleware != nil {
            ctx, err := s.apiKeyMiddleware.ValidateAndEnrich(r.Context(), r)
            if err != nil {
                writeUnauthorized(w, err)
                return
            }
            r = r.WithContext(ctx)
        }

        handler(w, r)
    }
}
```
