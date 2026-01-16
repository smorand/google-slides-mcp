# Google Slides MCP Server - Development Guidelines

## Project Overview

HTTP streamable MCP server for Google Slides API integration. Built in Go with deployment to Google Cloud Run.

## Documentation Index

Detailed documentation is available in `.agent_docs/`:

| File | Contents |
|------|----------|
| [tools-reference.md](.agent_docs/tools-reference.md) | Complete tool documentation with inputs/outputs |
| [internal-packages.md](.agent_docs/internal-packages.md) | Internal package details (auth, cache, middleware, etc.) |
| [testing-debugging.md](.agent_docs/testing-debugging.md) | Testing patterns and debugging guide |
| [deployment.md](.agent_docs/deployment.md) | Terraform, Docker, Cloud Run deployment |

## Quick Commands

```bash
make build    # Build for current platform
make test     # Run all tests
make test-integration  # Run integration tests (requires credentials)
make run      # Build and run locally
make fmt      # Format code
make vet      # Run go vet
make check    # Run all checks (fmt, vet, lint, test)
make clean    # Remove build artifacts

# Terraform (two-phase deployment)
make init-plan    # Plan bootstrap (state bucket, service accounts)
make init-deploy  # Deploy bootstrap + generate iac/provider.tf
make plan         # Plan main infrastructure
make deploy       # Deploy main infrastructure (includes Docker build via Cloud Build)
make undeploy     # Destroy main infrastructure
```

## Project Structure

```
google-slides-mcp/
├── config.yaml              # Single source of truth for all config
├── cmd/google-slides-mcp/   # Entry point only - minimal code
├── internal/                # All implementation code
│   ├── auth/               # OAuth2 flow, API key generation
│   ├── cache/              # In-memory LRU caching with TTL
│   ├── integration/        # Integration tests
│   ├── middleware/         # API key validation, logging
│   ├── permissions/        # Drive permission checks
│   ├── ratelimit/          # Token bucket rate limiting
│   ├── retry/              # Exponential backoff retry
│   ├── tools/              # MCP tool implementations
│   └── transport/          # HTTP server, MCP protocol
├── init/                    # Terraform Phase 1: Bootstrap
│   ├── provider.tf         # Local backend
│   ├── local.tf            # Loads ../config.yaml
│   ├── state-backend.tf    # GCS bucket for state
│   ├── services-apis.tf    # Enable GCP APIs
│   └── services-accounts.tf # Service accounts + IAM
├── iac/                     # Terraform Phase 2: Infrastructure
│   ├── provider.tf.template # Template with bucket placeholder
│   ├── provider.tf          # Generated after init-deploy
│   ├── local.tf             # Loads ../config.yaml
│   └── workload-mcp.tf      # Cloud Run + Secrets + Firestore
├── .agent_docs/             # Detailed documentation for AI agents
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
- Use table-driven tests with `t.Run()`
- Prefer standard library over testify
- Mock interfaces via factory pattern
- `errors.Is(err, ErrExpected)` for error checking

## MCP Protocol

- Endpoint: `POST /mcp` for tool calls
- Format: JSON-RPC 2.0
- Transport: Chunked transfer encoding for streaming

### Tool Implementation Pattern

```go
// internal/tools/example.go
type ExampleInput struct {
    PresentationID string `json:"presentation_id"`
}

type ExampleOutput struct {
    Result string `json:"result"`
}

func (t *Tools) Example(ctx context.Context, tokenSource oauth2.TokenSource, input ExampleInput) (*ExampleOutput, error) {
    if input.PresentationID == "" {
        return nil, ErrInvalidPresentationID
    }
    // Implementation
    return &ExampleOutput{}, nil
}
```

## Authentication Flow

1. `/auth` - Initiates OAuth2 flow
2. `/auth/callback` - Receives OAuth2 code, exchanges for tokens
3. Generate API key, store in Firestore
4. Return API key to user

All subsequent requests require `Authorization: Bearer <api_key>` header.

## Google APIs Used

- **Slides API**: Presentation manipulation
- **Drive API**: File search, permissions, copy, comments
- **Translate API**: Text translation

## Key Design Decisions

1. **HTTP Streamable vs SSE**: Using chunked transfer for better compatibility
2. **Firestore**: Chosen for API key storage due to low latency
3. **In-memory cache**: LRU cache for access tokens and permissions
4. **Rate limiting**: Token bucket algorithm for fairness

---

## Tools Quick Reference

| Category | Tool | Description |
|----------|------|-------------|
| **Presentation** | `get_presentation` | Load full presentation structure |
| | `search_presentations` | Search Drive for presentations |
| | `copy_presentation` | Copy presentation (useful for templates) |
| | `create_presentation` | Create new empty presentation |
| | `export_pdf` | Export to PDF (base64) |
| **Slides** | `list_slides` | List all slides with metadata |
| | `describe_slide` | Detailed description of single slide |
| | `add_slide` | Add slide with layout |
| | `delete_slide` | Delete slide by index or ID |
| | `reorder_slides` | Move slides to new positions |
| | `duplicate_slide` | Duplicate existing slide |
| **Objects** | `list_objects` | List objects with optional filtering |
| | `get_object` | Get detailed object info by ID |
| | `delete_object` | Delete one or more objects |
| | `transform_object` | Move, resize, rotate any object |
| | `change_z_order` | Change layering (front/back) |
| | `group_objects` | Group/ungroup objects |
| **Text** | `add_text_box` | Add text box with optional styling |
| | `modify_text` | Replace, append, prepend, delete text |
| | `style_text` | Apply font, color, bold, italic, etc. |
| | `format_paragraph` | Alignment, spacing, indentation |
| | `search_text` | Search text across all slides |
| | `replace_text` | Find and replace text |
| **Lists** | `create_bullet_list` | Convert text to bullets |
| | `create_numbered_list` | Convert text to numbered list |
| | `modify_list` | Modify/remove list, change indent |
| **Images** | `add_image` | Add image from base64 |
| | `modify_image` | Position, size, crop, brightness, etc. |
| | `replace_image` | Replace image preserving transform |
| **Video** | `add_video` | Add YouTube or Drive video |
| | `modify_video` | Position, size, start/end time, autoplay |
| **Shapes** | `create_shape` | Create shape with fill/outline |
| | `modify_shape` | Change fill, outline, shadow |
| | `create_line` | Create line/arrow |
| **Tables** | `create_table` | Create table with rows/columns |
| | `modify_table_structure` | Add/delete rows/columns |
| | `merge_cells` | Merge/unmerge cells |
| | `modify_table_cell` | Set text, style, alignment |
| | `style_table_cells` | Background, borders |
| **Theme/Background** | `apply_theme` | Copy theme from another presentation |
| | `set_background` | Solid color, image, or gradient |
| | `configure_footer` | Slide numbers, date, custom text |
| **Comments** | `list_comments` | List all comments |
| | `add_comment` | Add comment with optional anchor |
| | `manage_comment` | Reply, resolve, unresolve, delete |
| **Other** | `manage_speaker_notes` | Get, set, append, clear notes |
| | `manage_hyperlinks` | List, add, remove hyperlinks |
| | `translate_presentation` | Translate text using Cloud Translation |
| | `batch_update` | Execute multiple operations efficiently |
| **Not Supported** | `set_transition` | API limitation - use Slides UI |
| | `add_animation` | API limitation - use Slides UI |
| | `manage_animations` | API limitation - use Slides UI |

> **For detailed tool documentation** (inputs, outputs, errors, usage patterns), see [.agent_docs/tools-reference.md](.agent_docs/tools-reference.md)

---

## Common Conventions

### Slide Reference (choose one)
- `slide_index` (int): 1-based index
- `slide_id` (string): Object ID

### Position/Size (in points)
```json
{"position": {"x": 100, "y": 50}, "size": {"width": 300, "height": 100}}
```
Note: 1 point = 12700 EMU. Standard slide: 720 x 405 points.

### Colors
- Hex strings: `#RRGGBB` (e.g., `#FF0000`)
- Transparent: `"transparent"`

### Common Sentinel Errors
```go
ErrInvalidPresentationID  // Empty presentation ID
ErrPresentationNotFound   // 404 - presentation does not exist
ErrAccessDenied           // 403 - no permission
ErrInvalidSlideReference  // Neither slide_index nor slide_id provided
ErrSlideNotFound          // Slide index out of range or ID not found
ErrObjectNotFound         // Object ID not found
ErrSlidesAPIError         // Other Slides API errors
ErrDriveAPIError          // Drive API errors
```

### Layout Types
`BLANK`, `CAPTION_ONLY`, `TITLE`, `TITLE_AND_BODY`, `TITLE_AND_TWO_COLUMNS`, `TITLE_ONLY`, `ONE_COLUMN_TEXT`, `MAIN_POINT`, `BIG_NUMBER`, `SECTION_HEADER`, `SECTION_TITLE_AND_DESCRIPTION`

### Shape Types (Common)
`RECTANGLE`, `ROUND_RECTANGLE`, `ELLIPSE`, `TRIANGLE`, `DIAMOND`, `STAR_5`, `ARROW_RIGHT`, `ARROW_LEFT`, `CLOUD_CALLOUT`, `HEART`, `LIGHTNING_BOLT`

---

## Common Patterns

### EMU Conversion
```go
const pointsToEMU = 12700  // 1 point = 12700 EMU
```

### Error Wrapping
```go
return nil, fmt.Errorf("get presentation %s: %w", id, err)
```

### Input Validation
```go
if input.PresentationID == "" {
    return nil, ErrInvalidPresentationID
}
if input.SlideIndex == 0 && input.SlideID == "" {
    return nil, ErrInvalidSlideReference
}
```

### Context Values
```go
tokenSource := middleware.GetTokenSource(ctx)
userEmail := middleware.GetUserEmail(ctx)
```

---

## Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| "token expired" | Refresh token revoked | Re-authenticate via /auth |
| "permission denied" | No file access | Check Drive permissions |
| "rate limit exceeded" | Too many requests | Wait for Retry-After |
| "slide index out of range" | Index > slide count | Use list_slides first |

---

## Contributing

### Adding a New Tool

1. Create `internal/tools/{tool_name}.go` with Input/Output structs and handler
2. Create `internal/tools/{tool_name}_test.go` with table-driven tests
3. Register in MCP handler if required
4. Update this CLAUDE.md (add to Tools Quick Reference table)
5. Add detailed documentation to `.agent_docs/tools-reference.md`

### Commit Messages
```
[US-00XXX] Short description

- Detail 1
- Detail 2
```

### Code Review Checklist
- [ ] Tests cover new functionality
- [ ] Error handling (no ignored errors)
- [ ] Interfaces for external dependencies
- [ ] Context propagated correctly
- [ ] Input validation present
- [ ] Documentation updated
