# Google Slides MCP Server

An HTTP streamable MCP (Model Context Protocol) server that provides AI assistants with tools to interact with Google Slides presentations.

## Overview

This MCP server enables AI assistants to:
- Read and analyze Google Slides presentations
- Create, modify, and manage slides
- Work with text, images, shapes, tables, and videos
- Apply themes, transitions, and animations
- Manage comments and speaker notes
- Translate presentations
- Export to PDF and other formats

## Architecture

The server is designed to run on Google Cloud Run and uses:
- **HTTP Streamable Transport**: JSON-RPC 2.0 over HTTP with chunked transfer encoding
- **OAuth2 Authentication**: User authorization via Google OAuth2
- **API Key Management**: Generated API keys stored in Firestore
- **Secret Manager**: Secure storage of OAuth2 credentials

### Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check endpoint |
| `/mcp/initialize` | POST | MCP protocol handshake |
| `/mcp` | POST | MCP tool calls |
| `/auth` | GET | Initiate OAuth2 flow |
| `/auth/callback` | GET | OAuth2 callback |

### MCP Protocol

The server implements the MCP (Model Context Protocol) specification:
- Protocol version: `2024-11-05`
- Transport: HTTP with chunked transfer encoding
- Format: JSON-RPC 2.0

Example initialize request:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": {
      "name": "my-client",
      "version": "1.0.0"
    }
  }
}
```

## Project Structure

```
google-slides-mcp/
├── cmd/
│   └── google-slides-mcp/    # Main application entry point
├── internal/                  # Private application code
│   ├── auth/                 # OAuth2 and API key authentication
│   ├── cache/                # Caching layer
│   ├── middleware/           # HTTP middleware
│   ├── permissions/          # Permission verification
│   ├── ratelimit/            # Rate limiting
│   ├── retry/                # Retry logic with backoff
│   ├── tools/                # MCP tool implementations
│   └── transport/            # HTTP transport layer
├── pkg/                      # Public library code
├── terraform/                # Infrastructure as Code
├── scripts/                  # Utility scripts
├── Makefile                  # Build automation
├── Dockerfile               # Container definition
└── cloudbuild.yaml          # CI/CD configuration
```

## Getting Started

### Prerequisites

- Go 1.21 or later
- Google Cloud Platform account
- Terraform (for deployment)
- Docker (for containerization)

### Development

```bash
# Build the project
make build

# Run tests
make test

# Run the server locally
make run

# Format code
make fmt

# Run all checks
make check
```

### Docker

```bash
# Build the Docker image
docker build -t google-slides-mcp .

# Run locally with Docker
docker run -p 8080:8080 google-slides-mcp

# Build with version information
docker build \
  --build-arg VERSION=1.0.0 \
  --build-arg COMMIT_SHA=$(git rev-parse HEAD) \
  --build-arg BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  -t google-slides-mcp .
```

The Docker image uses:
- Multi-stage build for minimal image size
- Distroless base image for security
- Non-root user execution (UID 65532)

### Deployment

The server is designed to be deployed on Google Cloud Run. Infrastructure is managed with Terraform.

#### Prerequisites

1. A GCP project with billing enabled
2. `gcloud` CLI configured with project access
3. Terraform 1.0+ installed

#### Infrastructure Setup

```bash
# 1. Update configuration
# Edit terraform/config.yaml with your GCP project ID
vim terraform/config.yaml

# 2. Initialize and deploy infrastructure
make plan    # Preview changes
make deploy  # Apply changes

# 3. Add OAuth2 credentials to Secret Manager
# After creating OAuth2 credentials in GCP Console:
gcloud secrets versions add gslides-dev-oauth-client-id --data-file=client_id.txt
gcloud secrets versions add gslides-dev-oauth-client-secret --data-file=client_secret.txt
gcloud secrets versions add gslides-dev-oauth-redirect-uri --data-file=redirect_uri.txt

# 4. Deploy the container image (via CI/CD or manual)
gcloud builds submit --tag gcr.io/YOUR_PROJECT_ID/gslides-mcp:latest
```

#### Terraform Resources Created

| Resource | Description |
|----------|-------------|
| Cloud Run Service | MCP server with auto-scaling |
| Firestore Database | API keys and refresh tokens |
| Secret Manager | OAuth2 credentials |
| Service Accounts | Cloud Run and Cloud Build |
| IAM Roles | Least-privilege permissions |

#### Destroying Infrastructure

```bash
make undeploy  # Destroy all resources
```

## Authentication Flow

The server uses OAuth2 with Google for user authentication:

### Flow Steps

1. **Initiate Flow**: Client calls `GET /auth`
2. **Get Authorization URL**: Server returns JSON with `authorization_url`
   ```json
   {
     "authorization_url": "https://accounts.google.com/o/oauth2/auth?...",
     "message": "Please visit the authorization URL to complete authentication"
   }
   ```
3. **User Consent**: User visits the URL and grants permissions for:
   - Google Slides API (read/write presentations)
   - Google Drive API (search, copy, share files)
   - Google Cloud Translation API (translate content)
4. **Callback**: Google redirects to `/auth/callback` with authorization code
5. **Token Exchange**: Server exchanges code for access and refresh tokens
6. **API Key Generation**: Server generates UUID-format API key
7. **Storage**: API key and refresh token stored in Firestore
8. **Response**: API key returned to user (shown only once)
   ```json
   {
     "message": "Authentication successful",
     "api_key": "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx",
     "api_key_warning": "Save this API key securely. It will not be shown again.",
     "has_refresh_token": true
   }
   ```
9. **Subsequent Requests**: Include `Authorization: Bearer <api_key>` header

### API Key Storage

API keys are stored in Firestore with the following structure:

| Field | Type | Description |
|-------|------|-------------|
| `api_key` | string | UUID v4 format key (document ID) |
| `refresh_token` | string | OAuth2 refresh token for API access |
| `user_email` | string | User's email (optional) |
| `created_at` | timestamp | When the key was generated |
| `last_used` | timestamp | Last time the key was used |

### Required OAuth2 Scopes

- `https://www.googleapis.com/auth/presentations` - Full Slides access
- `https://www.googleapis.com/auth/drive` - Full Drive access
- `https://www.googleapis.com/auth/cloud-translation` - Translation API

### Security Features

- CSRF protection via cryptographic state parameter
- State tokens are single-use (consumed after callback)
- Refresh tokens enable offline access without re-authentication
- API key validation middleware protects all MCP endpoints
- Token caching with TTL reduces Firestore reads
- Last-used timestamp tracking for key activity monitoring

### Permission Verification

Before modifying any presentation, the server verifies that the user has appropriate permissions:

- **Write operations**: Requires `writer` or `owner` role
- **Read operations**: Requires at least `viewer` or `commenter` role

Permission checks use the Google Drive API to verify file capabilities:
- Results are cached with a 5-minute TTL for performance
- Clear error messages are returned when permissions are insufficient

Example error response for insufficient permissions:
```json
{
  "error": "user does not have write permission on this presentation"
}
```

## Rate Limiting

The server implements global rate limiting to protect against abuse:

### Token Bucket Algorithm
- Configurable requests per second limit (default: 10 req/s)
- Burst capacity for handling traffic spikes (default: 20 requests)
- Per-endpoint rate limits for fine-grained control

### Response Headers
All responses include rate limit information:

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Maximum requests allowed (burst size) |
| `X-RateLimit-Remaining` | Remaining requests in current window |
| `X-RateLimit-Reset` | Unix timestamp when limit resets |

### Rate Limit Exceeded
When the rate limit is exceeded, the server returns:
- HTTP Status: `429 Too Many Requests`
- `Retry-After` header with seconds to wait
- JSON body:
  ```json
  {
    "error": "rate limit exceeded",
    "retry_after": 1
  }
  ```

### Configuration
Rate limits can be configured per-endpoint for different use cases:
- Heavy operations (e.g., export): Lower limits
- Read operations: Higher limits
- Authentication endpoints: Separate limits

## Available MCP Tools

The server provides comprehensive tools for Google Slides manipulation:

### Presentation Management

#### `get_presentation`

Load a Google Slides presentation and return its full structured content.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "include_thumbnails": false
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `include_thumbnails` | boolean | No | Include base64-encoded slide thumbnails (default: false) |

**Output:**
```json
{
  "presentation_id": "abc123xyz",
  "title": "My Presentation",
  "locale": "en_US",
  "slides_count": 5,
  "page_size": {
    "width": {"magnitude": 720, "unit": "PT"},
    "height": {"magnitude": 405, "unit": "PT"}
  },
  "slides": [
    {
      "index": 1,
      "object_id": "slide-id-1",
      "layout_id": "layout-id",
      "layout_name": "Title Slide",
      "text_content": [
        {
          "object_id": "text-box-1",
          "object_type": "TEXT_BOX",
          "text": "Hello World"
        }
      ],
      "speaker_notes": "Notes for this slide",
      "object_count": 3,
      "objects": [
        {"object_id": "text-box-1", "object_type": "TEXT_BOX"},
        {"object_id": "image-1", "object_type": "IMAGE"}
      ],
      "thumbnail_base64": "..." // Only if include_thumbnails=true
    }
  ],
  "masters": [
    {"object_id": "master-1", "name": "Default Master"}
  ],
  "layouts": [
    {"object_id": "layout-1", "name": "Title Slide", "master_id": "master-1", "layout_type": "TITLE"}
  ]
}
```

**Features:**
- Extracts all text content from slides including shapes, text boxes, and tables
- Retrieves speaker notes for each slide
- Returns complete presentation structure including masters and layouts
- Optionally includes slide thumbnails as base64-encoded images
- Handles grouped elements recursively

**Object Types Detected:**
- Shapes: `TEXT_BOX`, `RECTANGLE`, `ELLIPSE`, etc.
- Media: `IMAGE`, `VIDEO`
- Containers: `TABLE`, `GROUP`
- Other: `LINE`, `SHEETS_CHART`, `WORD_ART`

**Errors:**
- `presentation not found` - The presentation ID doesn't exist
- `access denied to presentation` - No permission to access the presentation
- `slides API error` - Other Slides API errors

---

#### `search_presentations`

Search for Google Slides presentations in Google Drive.

**Input:**
```json
{
  "query": "quarterly report",
  "max_results": 10
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Yes | Search term or Drive query operators |
| `max_results` | integer | No | Maximum results to return (default: 10, max: 100) |

**Output:**
```json
{
  "presentations": [
    {
      "id": "abc123xyz",
      "title": "Q4 Quarterly Report 2024",
      "owner": "user@example.com",
      "modified_date": "2024-01-15T10:30:00Z",
      "thumbnail_url": "https://drive.google.com/thumbnail/..."
    }
  ],
  "total_results": 5,
  "query": "quarterly report"
}
```

**Features:**
- Searches owned, shared, and shared drive presentations
- Only returns Google Slides files (not Docs, Sheets, etc.)
- Simple queries automatically use full-text search
- Supports advanced Google Drive search operators

**Advanced Query Examples:**
```json
// Search by name
{"query": "name contains 'Budget'"}

// Search by modification date
{"query": "modifiedTime > '2024-01-01'"}

// Combined search
{"query": "name contains 'Report' and modifiedTime > '2024-01-01'"}

// Search shared files
{"query": "sharedWithMe = true"}
```

**Errors:**
- `invalid search query: query is required` - Empty query provided
- `access denied` - No permission to search Drive
- `drive API error` - Other Drive API errors

---

#### `copy_presentation`

Copy a Google Slides presentation to create a new one. Useful for creating presentations from templates.

**Input:**
```json
{
  "source_id": "1abc2def3ghi...",
  "new_title": "Q1 2024 Report",
  "destination_folder_id": "folder123..."
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `source_id` | string | Yes | ID of the presentation to copy |
| `new_title` | string | Yes | Title for the new presentation |
| `destination_folder_id` | string | No | Folder to place the copy in (default: root) |

**Output:**
```json
{
  "presentation_id": "new-id-123...",
  "title": "Q1 2024 Report",
  "url": "https://docs.google.com/presentation/d/new-id-123.../edit",
  "source_id": "1abc2def3ghi..."
}
```

**Features:**
- Creates an exact copy of the source presentation
- Preserves all formatting, themes, masters, and content
- Places copy in specified folder or user's Drive root
- Returns direct edit URL for immediate access

**Use Cases:**
- Creating presentations from company templates
- Duplicating presentations for different audiences
- Creating backups before making major changes

**Errors:**
- `invalid source presentation ID: source_id is required` - Empty source ID
- `invalid title: new_title is required` - Empty title
- `source presentation not found` - Source doesn't exist or no access
- `access denied to source presentation` - No permission to copy
- `destination folder not found or inaccessible` - Invalid folder ID

---

#### `export_pdf`

Export a Google Slides presentation to PDF format.

**Input:**
```json
{
  "presentation_id": "abc123xyz"
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID to export |

**Output:**
```json
{
  "pdf_base64": "JVBERi0xLjQK...",
  "page_count": 10,
  "file_size": 123456
}
```

| Field | Type | Description |
|-------|------|-------------|
| `pdf_base64` | string | Base64-encoded PDF content |
| `page_count` | integer | Number of pages detected in PDF |
| `file_size` | integer | PDF file size in bytes |

**Features:**
- Uses Google Drive API export functionality
- Returns PDF as base64 for easy transfer via JSON
- Detects page count using PDF structure analysis
- Includes file size metadata

**Use Cases:**
- Generating printable versions of presentations
- Creating archives of presentation content
- Sharing presentations with non-Google users

**Client-side PDF handling:**
```javascript
// Decode and save the PDF
const pdfData = atob(response.pdf_base64);
const blob = new Blob([pdfData], { type: 'application/pdf' });
const url = URL.createObjectURL(blob);
```

**Errors:**
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to export
- `failed to export presentation` - Export operation failed

---

#### `create_presentation`

Create a new empty Google Slides presentation.

**Input:**
```json
{
  "title": "My New Presentation",
  "folder_id": "folder-id-optional"
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `title` | string | Yes | Title for the new presentation |
| `folder_id` | string | No | Google Drive folder ID to place the presentation in |

**Output:**
```json
{
  "presentation_id": "new-presentation-id",
  "title": "My New Presentation",
  "url": "https://docs.google.com/presentation/d/new-presentation-id/edit",
  "folder_id": "folder-id-optional"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `presentation_id` | string | Unique ID of the created presentation |
| `title` | string | Title of the presentation |
| `url` | string | Direct edit URL for the presentation |
| `folder_id` | string | Folder ID if specified in input (omitted otherwise) |

**Features:**
- Creates a new empty presentation via Slides API
- Optionally places the presentation in a specific folder
- Returns direct edit URL for immediate access

**Use Cases:**
- Creating new presentations from scratch
- Setting up presentation structure programmatically
- Organizing presentations into specific folders

**Errors:**
- `invalid title for presentation: title is required` - Empty title
- `access denied` - No permission to create presentations
- `destination folder not found or inaccessible` - Invalid folder ID
- `failed to create presentation` - Creation failed

---

### Slide Operations

#### `list_slides`

List all slides in a presentation with metadata and summary statistics.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "include_thumbnails": false
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `include_thumbnails` | boolean | No | Include base64-encoded slide thumbnails (default: false) |

**Output:**
```json
{
  "presentation_id": "abc123xyz",
  "title": "My Presentation",
  "slides": [
    {
      "index": 1,
      "slide_id": "slide-object-id-1",
      "title": "Introduction",
      "layout_type": "TITLE",
      "object_count": 3,
      "thumbnail_base64": "..."
    },
    {
      "index": 2,
      "slide_id": "slide-object-id-2",
      "title": "Overview",
      "layout_type": "TITLE_AND_BODY",
      "object_count": 5,
      "thumbnail_base64": "..."
    }
  ],
  "statistics": {
    "total_slides": 10,
    "slides_with_notes": 5,
    "slides_with_videos": 2
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `presentation_id` | string | The presentation ID |
| `title` | string | Presentation title |
| `slides` | array | Array of slide metadata |
| `slides[].index` | integer | 1-based slide position |
| `slides[].slide_id` | string | Unique slide object ID |
| `slides[].title` | string | Slide title (from TITLE placeholder) |
| `slides[].layout_type` | string | Layout type (e.g., TITLE, TITLE_AND_BODY, BLANK) |
| `slides[].object_count` | integer | Number of page elements on the slide |
| `slides[].thumbnail_base64` | string | Base64 thumbnail (only if requested) |
| `statistics.total_slides` | integer | Total number of slides |
| `statistics.slides_with_notes` | integer | Count of slides with speaker notes |
| `statistics.slides_with_videos` | integer | Count of slides containing videos |

**Features:**
- Returns 1-based slide indices for easy human reference
- Extracts slide title from TITLE or CENTERED_TITLE placeholders
- Detects layout type from presentation layouts
- Counts slides with speaker notes (non-empty notes page)
- Counts slides containing video elements (including nested in groups)
- Optional thumbnail support via base64 encoding

**Use Cases:**
- Getting a quick overview of presentation structure
- Navigating large presentations by title
- Finding slides with specific characteristics (notes, videos)
- Generating table of contents or slide indexes

**Errors:**
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to access

---

#### `describe_slide`

Get a detailed human-readable description of a specific slide, including all objects with their positions and content summaries.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "slide_index": 1
}
```

Or using slide ID:
```json
{
  "presentation_id": "abc123xyz",
  "slide_id": "g1234567890"
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `slide_index` | integer | One of these | 1-based slide index |
| `slide_id` | string | required | Unique slide object ID |

**Output:**
```json
{
  "presentation_id": "abc123xyz",
  "slide_id": "g1234567890",
  "slide_index": 1,
  "title": "Introduction",
  "layout_type": "TITLE_AND_BODY",
  "page_size": {
    "width": {"magnitude": 720, "unit": "PT"},
    "height": {"magnitude": 405, "unit": "PT"}
  },
  "objects": [
    {
      "object_id": "title-shape-1",
      "object_type": "TEXT_BOX",
      "position": {"x": 60.0, "y": 30.0},
      "size": {"width": 600.0, "height": 50.0},
      "content_summary": "Introduction to the Project",
      "z_order": 0
    },
    {
      "object_id": "image-1",
      "object_type": "IMAGE",
      "position": {"x": 50.0, "y": 100.0},
      "size": {"width": 300.0, "height": 200.0},
      "content_summary": "Image (external)",
      "z_order": 1
    },
    {
      "object_id": "group-1",
      "object_type": "GROUP",
      "position": {"x": 400.0, "y": 100.0},
      "size": {"width": 200.0, "height": 150.0},
      "content_summary": "Group (3 items)",
      "z_order": 2,
      "children": [
        {
          "object_id": "shape-in-group",
          "object_type": "RECTANGLE",
          "content_summary": "",
          "z_order": 0
        }
      ]
    }
  ],
  "layout_description": "Title at top: \"Introduction\". 2 element(s) in center. Contains: 1 group, 1 image, 1 text_box",
  "screenshot_base64": "iVBORw0KGgoAAAANSUhEUgAA...",
  "speaker_notes": "Remember to emphasize the key points here"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `presentation_id` | string | The presentation ID |
| `slide_id` | string | Unique slide object ID |
| `slide_index` | integer | 1-based slide position |
| `title` | string | Slide title (from TITLE placeholder) |
| `layout_type` | string | Layout type (e.g., TITLE, TITLE_AND_BODY) |
| `page_size` | object | Slide dimensions |
| `objects` | array | Array of object descriptions |
| `objects[].object_id` | string | Unique object identifier |
| `objects[].object_type` | string | Type of object (TEXT_BOX, IMAGE, etc.) |
| `objects[].position` | object | X, Y position in points |
| `objects[].size` | object | Width, height in points |
| `objects[].content_summary` | string | Content summary (max 100 chars) |
| `objects[].z_order` | integer | Stacking order (0 = back) |
| `objects[].children` | array | Child objects (for groups) |
| `layout_description` | string | Human-readable layout description |
| `screenshot_base64` | string | Base64 PNG screenshot of the slide |
| `speaker_notes` | string | Speaker notes content |

**Features:**
- Accepts either slide index (1-based) OR slide ID for flexibility
- Returns position and size in points (converted from EMU)
- Generates human-readable layout description for AI context
- Includes screenshot for visual reference
- Recursively describes grouped elements
- Content summaries truncated to 100 characters

**Content Summary Types:**
- Text shapes: First 100 characters of text content
- Placeholders: `[TITLE placeholder]`, `[BODY placeholder]`
- Images: `Image` or `Image (external)`
- Videos: `YouTube video: <id>` or `Video: <id>`
- Tables: `Table (3x4)` (rows x columns)
- Lines: Line type (e.g., `STRAIGHT_LINE`)
- Groups: `Group (N items)`

**Use Cases:**
- Providing AI assistants with detailed slide context
- Understanding object layout and positioning
- Planning modifications to existing slides
- Accessibility: describing slide content textually

**Errors:**
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `either slide_index or slide_id must be provided` - No slide reference
- `slide not found: slide index N out of range` - Invalid slide index
- `slide not found: slide_id 'xxx' not found` - Invalid slide ID
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to access

---

#### `add_slide`

Add a new slide to a presentation at a specified position.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "position": 2,
  "layout": "TITLE_AND_BODY"
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `position` | integer | No | 1-based position (0 or omitted = end) |
| `layout` | string | Yes | Layout type (see supported layouts below) |

**Supported Layout Types:**
| Layout | Description |
|--------|-------------|
| `BLANK` | Empty slide with no placeholders |
| `CAPTION_ONLY` | Caption text at bottom |
| `TITLE` | Title slide |
| `TITLE_AND_BODY` | Title with body text area |
| `TITLE_AND_TWO_COLUMNS` | Title with two column layout |
| `TITLE_ONLY` | Title placeholder only |
| `ONE_COLUMN_TEXT` | Single column text layout |
| `MAIN_POINT` | Main point emphasis layout |
| `BIG_NUMBER` | Large number display layout |
| `SECTION_HEADER` | Section header slide |
| `SECTION_TITLE_AND_DESCRIPTION` | Section with description |

**Output:**
```json
{
  "slide_index": 2,
  "slide_id": "g123456789"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `slide_index` | integer | 1-based index of the new slide |
| `slide_id` | string | Object ID of the created slide |

**Features:**
- Position 0 or omitted inserts at the end of the presentation
- Position beyond the slide count inserts at the end
- Automatically finds matching layout in the presentation
- Falls back to first available layout if no exact match
- Falls back to predefined layout type if presentation has no layouts

**Use Cases:**
- Adding slides to existing presentations
- Building presentations programmatically
- Inserting slides at specific positions in a workflow

**Examples:**

Add a slide at the end:
```json
{
  "presentation_id": "abc123",
  "layout": "BLANK"
}
```

Insert a slide as the second slide:
```json
{
  "presentation_id": "abc123",
  "position": 2,
  "layout": "TITLE_AND_BODY"
}
```

**Errors:**
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `invalid layout type: layout is required` - Missing layout
- `invalid layout type: unsupported layout 'XXX'` - Unknown layout type
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify

---

#### `delete_slide`

Delete a slide from a presentation by index or ID.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "slide_index": 2
}
```

Or by slide ID:
```json
{
  "presentation_id": "abc123xyz",
  "slide_id": "g123456789"
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `slide_index` | integer | No* | 1-based index of slide to delete |
| `slide_id` | string | No* | Object ID of slide to delete |

*Either `slide_index` or `slide_id` is required. If both are provided, `slide_id` takes precedence.

**Output:**
```json
{
  "deleted_slide_id": "g123456789",
  "remaining_slide_count": 2
}
```

| Field | Type | Description |
|-------|------|-------------|
| `deleted_slide_id` | string | Object ID of the deleted slide |
| `remaining_slide_count` | integer | Number of slides remaining after deletion |

**Features:**
- Delete by 1-based index or by slide object ID
- Prevents deletion of the last remaining slide
- Returns confirmation with updated slide count

**Use Cases:**
- Removing unwanted slides from presentations
- Cleaning up template slides after population
- Programmatic presentation management

**Examples:**

Delete the second slide:
```json
{
  "presentation_id": "abc123",
  "slide_index": 2
}
```

Delete by slide ID:
```json
{
  "presentation_id": "abc123",
  "slide_id": "g987654321"
}
```

**Errors:**
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `invalid slide reference: either slide_index or slide_id is required` - No slide specified
- `cannot delete the last remaining slide: presentation must have at least one slide` - Last slide protection
- `slide not found: slide with ID 'X' not found` - Slide ID doesn't exist
- `slide not found: slide index X out of range (1-N)` - Index out of bounds
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify

---

#### `reorder_slides`

Move slides to new positions within a presentation.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "slide_indices": [3, 5],
  "insert_at": 1
}
```

Or by slide IDs:
```json
{
  "presentation_id": "abc123xyz",
  "slide_ids": ["g123456", "g789012"],
  "insert_at": 2
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `slide_indices` | array[int] | No* | 1-based indices of slides to move |
| `slide_ids` | array[string] | No* | Object IDs of slides to move |
| `insert_at` | integer | Yes | 1-based position to move slides to |

*Either `slide_indices` or `slide_ids` is required. If both are provided, `slide_ids` takes precedence.

**Output:**
```json
{
  "new_order": [
    {"index": 1, "slide_id": "g123456"},
    {"index": 2, "slide_id": "g789012"},
    {"index": 3, "slide_id": "g111111"},
    {"index": 4, "slide_id": "g222222"}
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `new_order` | array | Complete slide order after reordering |
| `new_order[].index` | integer | 1-based position in new order |
| `new_order[].slide_id` | string | Object ID of the slide |

**Features:**
- Move single or multiple slides at once
- Slides moved together maintain their relative order
- Insert position beyond slide count clamps to end
- Returns complete new slide order for verification

**Use Cases:**
- Reorganizing presentation flow
- Moving sections to different positions
- Reordering slides after review feedback
- Batch slide organization

**Examples:**

Move third slide to the beginning:
```json
{
  "presentation_id": "abc123",
  "slide_indices": [3],
  "insert_at": 1
}
```

Move slides 2 and 4 to the end:
```json
{
  "presentation_id": "abc123",
  "slide_indices": [2, 4],
  "insert_at": 10
}
```

Move specific slides by ID:
```json
{
  "presentation_id": "abc123",
  "slide_ids": ["g987654", "g123456"],
  "insert_at": 3
}
```

**Errors:**
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `no slides specified to move: either slide_indices or slide_ids is required` - No slides specified
- `invalid insert_at position: insert_at must be at least 1` - Invalid position
- `slide not found: slide index X out of range (1-N)` - Index out of bounds
- `slide not found: slide with ID 'X' not found` - Slide ID doesn't exist
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify

---

#### `duplicate_slide`

Duplicate an existing slide in a presentation, optionally placing the copy at a specific position.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "slide_index": 2
}
```

Or by slide ID:
```json
{
  "presentation_id": "abc123xyz",
  "slide_id": "g123456789",
  "insert_at": 1
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `slide_index` | integer | No* | 1-based index of slide to duplicate |
| `slide_id` | string | No* | Object ID of slide to duplicate |
| `insert_at` | integer | No | 1-based position for the copy (0 or omitted = after source slide) |

*Either `slide_index` or `slide_id` is required. If both are provided, `slide_id` takes precedence.

**Output:**
```json
{
  "slide_index": 3,
  "slide_id": "g987654321"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `slide_index` | integer | 1-based index of the new duplicated slide |
| `slide_id` | string | Object ID of the new duplicated slide |

**Features:**
- Creates an exact copy of the source slide including all objects
- Default position (insert_at omitted or 0) places copy immediately after the source slide
- Insert position beyond slide count is clamped to end
- If move operation fails, returns slide in default position with warning
- Uses Google Slides API `DuplicateObjectRequest` for atomic duplication

**Use Cases:**
- Creating variations of existing slides
- Building repetitive slide sections (agendas, separators)
- Preserving slide as backup before modifications
- Template-based slide generation

**Examples:**

Duplicate slide 2 (copy placed after slide 2 at position 3):
```json
{
  "presentation_id": "abc123",
  "slide_index": 2
}
```

Duplicate slide by ID and move to beginning:
```json
{
  "presentation_id": "abc123",
  "slide_id": "g987654321",
  "insert_at": 1
}
```

Duplicate last slide to the end:
```json
{
  "presentation_id": "abc123",
  "slide_index": 5,
  "insert_at": 6
}
```

**Errors:**
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `invalid slide reference: either slide_index or slide_id is required` - No slide specified
- `slide not found: slide with ID 'X' not found` - Slide ID doesn't exist
- `slide not found: slide index X out of range (1-N)` - Index out of bounds
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify
- `failed to duplicate slide` - Duplication operation failed

---

#### `list_objects`

List all objects on slides with optional filtering by slide indices and object types.

**Input:**
```json
{
  "presentation_id": "abc123xyz"
}
```

With filters:
```json
{
  "presentation_id": "abc123xyz",
  "slide_indices": [1, 3],
  "object_types": ["IMAGE", "VIDEO"]
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `slide_indices` | integer[] | No | 1-based slide indices to include (default: all slides) |
| `object_types` | string[] | No | Object types to include (default: all types) |

**Supported Object Types:**
- `TEXT_BOX`, `RECTANGLE`, `ELLIPSE`, `TRIANGLE`, etc. (shapes)
- `IMAGE`, `VIDEO`, `TABLE`, `LINE`
- `GROUP`, `SHEETS_CHART`, `WORD_ART`

**Output:**
```json
{
  "presentation_id": "abc123xyz",
  "objects": [
    {
      "slide_index": 1,
      "object_id": "g123456789",
      "object_type": "TEXT_BOX",
      "position": {"x": 100, "y": 50},
      "size": {"width": 300, "height": 100},
      "z_order": 0,
      "content_preview": "Hello World"
    },
    {
      "slide_index": 1,
      "object_id": "g987654321",
      "object_type": "IMAGE",
      "position": {"x": 400, "y": 150},
      "size": {"width": 200, "height": 200},
      "z_order": 1,
      "content_preview": ""
    }
  ],
  "total_count": 2,
  "filtered_by": {
    "slide_indices": [1],
    "object_types": ["TEXT_BOX", "IMAGE"]
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `presentation_id` | string | The presentation ID |
| `objects` | array | Array of object listings |
| `total_count` | integer | Total number of objects returned |
| `filtered_by` | object | Filters applied (only if filters were specified) |

**Object Listing Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `slide_index` | integer | 1-based slide index containing the object |
| `object_id` | string | Unique object identifier |
| `object_type` | string | Type of the object (TEXT_BOX, IMAGE, etc.) |
| `position` | object | Object position in points {x, y} |
| `size` | object | Object dimensions in points {width, height} |
| `z_order` | integer | Layering position (lower = further back) |
| `content_preview` | string | First 100 characters of text content (for text objects) |

**Features:**
- Lists objects from all slides when no filters applied
- Filter by slide indices (1-based) to limit scope
- Filter by object types to find specific elements
- Position and size in points (standard slide: 720x405 points)
- Content preview for shapes with text (first 100 characters)
- Z-order indicates stacking order on the slide
- Recursively includes objects within groups

**Use Cases:**
- Inventory all objects in a presentation
- Find all images for replacement
- Locate videos for playback configuration
- Identify text boxes for bulk text operations
- Count object types for analysis

**Examples:**

List all objects in a presentation:
```json
{
  "presentation_id": "abc123"
}
```

List only images and videos:
```json
{
  "presentation_id": "abc123",
  "object_types": ["IMAGE", "VIDEO"]
}
```

List objects from specific slides:
```json
{
  "presentation_id": "abc123",
  "slide_indices": [1, 3, 5]
}
```

Combined filter - images from first slide only:
```json
{
  "presentation_id": "abc123",
  "slide_indices": [1],
  "object_types": ["IMAGE"]
}
```

**Errors:**
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to access
- `slides API error` - API error occurred

---

#### `get_object`

Get detailed information about a specific object by its ID.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "g123456789"
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `object_id` | string | Yes | The unique object identifier |

**Output:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "g123456789",
  "object_type": "TEXT_BOX",
  "slide_index": 1,
  "position": {"x": 100, "y": 50},
  "size": {"width": 300, "height": 100},
  "shape": {
    "shape_type": "TEXT_BOX",
    "text": "Hello World",
    "text_style": {
      "font_family": "Arial",
      "font_size": 24,
      "bold": true,
      "color": "#FF0000",
      "link_url": "https://example.com"
    },
    "fill": {
      "type": "SOLID",
      "solid_color": "#007FFF"
    },
    "outline": {
      "color": "#000000",
      "weight": 2,
      "dash_style": "SOLID"
    },
    "placeholder_type": "TITLE"
  }
}
```

**Type-Specific Output Fields:**

The output includes a type-specific field based on `object_type`:

| Object Type | Field | Description |
|-------------|-------|-------------|
| TEXT_BOX, RECTANGLE, etc. | `shape` | Shape properties including text, style, fill, outline |
| IMAGE | `image` | Image properties including URLs, crop, brightness, contrast |
| TABLE | `table` | Table structure with rows, columns, and cell contents |
| VIDEO | `video` | Video properties including ID, source, timing, autoplay |
| LINE | `line` | Line properties including type, arrows, color, weight |
| GROUP | `group` | Group with child count and child IDs |
| SHEETS_CHART | `chart` | Sheets chart with spreadsheet ID and chart ID |
| WORD_ART | `word_art` | Word art with rendered text |

**Shape Details (`shape` field):**
```json
{
  "shape_type": "TEXT_BOX",
  "text": "Content text",
  "text_style": {
    "font_family": "Arial",
    "font_size": 24,
    "bold": true,
    "italic": false,
    "underline": false,
    "color": "#FF0000",
    "link_url": "https://..."
  },
  "fill": {
    "type": "SOLID",
    "solid_color": "#FFFFFF"
  },
  "outline": {
    "color": "#000000",
    "weight": 1,
    "dash_style": "SOLID"
  },
  "placeholder_type": "TITLE"
}
```

**Image Details (`image` field):**
```json
{
  "content_url": "https://...",
  "source_url": "https://...",
  "brightness": 0.5,
  "contrast": 0.3,
  "transparency": 0.1,
  "recolor": "GRAYSCALE",
  "crop": {
    "top": 0.1,
    "bottom": 0.2,
    "left": 0.05,
    "right": 0.15
  }
}
```

**Table Details (`table` field):**
```json
{
  "rows": 3,
  "columns": 4,
  "cells": [
    [
      {"row": 0, "column": 0, "text": "Header 1", "row_span": 1, "column_span": 1, "background": "#E5E5E5"},
      {"row": 0, "column": 1, "text": "Header 2"}
    ],
    [
      {"row": 1, "column": 0, "text": "Cell A1"},
      {"row": 1, "column": 1, "text": "Cell B1"}
    ]
  ]
}
```

**Video Details (`video` field):**
```json
{
  "video_id": "dQw4w9WgXcQ",
  "source": "YOUTUBE",
  "url": "https://www.youtube.com/watch?v=...",
  "start_time": 30,
  "end_time": 60,
  "autoplay": true,
  "mute": false
}
```

**Line Details (`line` field):**
```json
{
  "line_type": "STRAIGHT_CONNECTOR_1",
  "start_arrow": "ARROW",
  "end_arrow": "NONE",
  "color": "#0000FF",
  "weight": 3,
  "dash_style": "DASH"
}
```

**Group Details (`group` field):**
```json
{
  "child_count": 3,
  "child_ids": ["child-1", "child-2", "child-3"]
}
```

**Features:**
- Returns complete object properties based on type
- Finds objects anywhere in the presentation (including nested in groups)
- Position and size in points (standard slide: 720x405 points)
- Colors returned as hex strings (#RRGGBB) or theme references (theme:ACCENT1)
- Video times in seconds (converted from internal milliseconds)
- Text style includes font, size, bold/italic, color, and hyperlinks

**Use Cases:**
- Inspect object properties before modification
- Get exact styling for duplication
- Extract text content and formatting
- Retrieve image URLs and adjustments
- Check video timing and playback settings
- Navigate group contents via child IDs

**Examples:**

Get a shape's details:
```json
{
  "presentation_id": "abc123",
  "object_id": "shape-xyz"
}
```

Get an image's properties:
```json
{
  "presentation_id": "abc123",
  "object_id": "image-123"
}
```

**Errors:**
- `object not found: object 'xyz' not found in presentation` - Object ID not found
- `object not found: object_id is required` - Empty object ID
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to access
- `slides API error` - API error occurred

---

#### `add_text_box`

Add a text box to a slide with optional styling.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "slide_index": 1,
  "text": "Hello World",
  "position": {"x": 100, "y": 50},
  "size": {"width": 300, "height": 100},
  "style": {
    "font_family": "Arial",
    "font_size": 24,
    "bold": true,
    "italic": false,
    "color": "#FF0000"
  }
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `slide_index` | integer | No* | 1-based index of the target slide |
| `slide_id` | string | No* | Object ID of the target slide |
| `text` | string | Yes | Text content for the text box |
| `position` | object | No | Position in points (default: 0, 0) |
| `position.x` | number | No | X coordinate in points from left edge |
| `position.y` | number | No | Y coordinate in points from top edge |
| `size` | object | Yes | Size in points |
| `size.width` | number | Yes | Width in points (must be > 0) |
| `size.height` | number | Yes | Height in points (must be > 0) |
| `style` | object | No | Text styling options |
| `style.font_family` | string | No | Font family name (e.g., "Arial") |
| `style.font_size` | integer | No | Font size in points |
| `style.bold` | boolean | No | Bold text |
| `style.italic` | boolean | No | Italic text |
| `style.color` | string | No | Hex color string (e.g., "#FF0000") |

*Either `slide_index` or `slide_id` must be provided.

**Output:**
```json
{
  "object_id": "textbox_1234567890123456"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | Unique identifier of the created text box |

**Features:**
- Uses either 1-based slide index or slide ID for flexibility
- Position defaults to (0, 0) if not specified
- Size is required with positive width and height
- Styling is optional - only specified style fields are applied
- Standard slide dimensions: 720x405 points
- 1 point = 12700 EMU (English Metric Units)

**Use Cases:**
- Add titles and headings to slides
- Insert body text content
- Create annotations or labels
- Add styled captions
- Place text at specific positions

**Examples:**

Add a simple text box:
```json
{
  "presentation_id": "abc123",
  "slide_index": 1,
  "text": "Hello World",
  "size": {"width": 200, "height": 50}
}
```

Add a styled title:
```json
{
  "presentation_id": "abc123",
  "slide_id": "g123456",
  "text": "Quarterly Report",
  "position": {"x": 110, "y": 20},
  "size": {"width": 500, "height": 60},
  "style": {
    "font_family": "Arial",
    "font_size": 36,
    "bold": true,
    "color": "#0000FF"
  }
}
```

**Errors:**
- `text content is required` - Empty text provided
- `size (width and height) is required` - Size missing or dimensions ≤ 0
- `invalid slide reference: either slide_index or slide_id must be provided` - Neither slide reference provided
- `slide not found` - Slide index out of range or slide ID not found
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify
- `failed to add text box` - API error during creation

---

#### `modify_text`

Modify text content in an existing shape (replace, append, prepend, or delete).

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "g123456789",
  "action": "replace",
  "text": "New text content"
}
```

For partial replacement:
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "g123456789",
  "action": "replace",
  "text": "REPLACED",
  "start_index": 5,
  "end_index": 10
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `object_id` | string | Yes | ID of the shape containing text to modify |
| `action` | string | Yes | Action to perform: `replace`, `append`, `prepend`, or `delete` |
| `text` | string | Conditional | New text content (required for replace/append/prepend, not for delete) |
| `start_index` | integer | No | Start index for partial replacement (0-based) |
| `end_index` | integer | No | End index for partial replacement (0-based, exclusive) |

**Actions:**

| Action | Description |
|--------|-------------|
| `replace` | Replace all text in the shape, or partial text if indices provided |
| `append` | Add text at the end of existing content |
| `prepend` | Add text at the beginning of existing content |
| `delete` | Remove all text from the shape |

**Output:**
```json
{
  "object_id": "g123456789",
  "updated_text": "New text content",
  "action": "replace"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | The modified object's ID |
| `updated_text` | string | The resulting text content after modification |
| `action` | string | The action that was performed |

**Features:**
- Supports four actions: replace all, append, prepend, delete
- Partial replacement using start_index/end_index for surgical edits
- Works with any shape containing text (TEXT_BOX, RECTANGLE, etc.)
- Returns the expected resulting text for confirmation
- Validates that target object supports text modification

**Use Cases:**
- Updating placeholder text with actual content
- Appending timestamps or signatures to existing text
- Clearing text boxes before re-populating
- Making targeted edits within existing text

**Examples:**

Replace all text in a shape:
```json
{
  "presentation_id": "abc123",
  "object_id": "title-shape",
  "action": "replace",
  "text": "Q1 2024 Report"
}
```

Append text to existing content:
```json
{
  "presentation_id": "abc123",
  "object_id": "body-shape",
  "action": "append",
  "text": "\n\nLast updated: January 2024"
}
```

Replace part of the text (characters 5-10):
```json
{
  "presentation_id": "abc123",
  "object_id": "shape-xyz",
  "action": "replace",
  "text": "NEW",
  "start_index": 5,
  "end_index": 10
}
```

Delete all text:
```json
{
  "presentation_id": "abc123",
  "object_id": "shape-xyz",
  "action": "delete"
}
```

**Errors:**
- `invalid action: action must be 'replace', 'append', 'prepend', or 'delete'` - Unknown action
- `text is required for this action: text is required for 'replace' action` - Missing text
- `invalid object_id: object_id is required` - Missing object ID
- `invalid text range: start_index cannot be negative` - Invalid index
- `invalid text range: start_index cannot be greater than end_index` - Invalid range
- `object does not contain editable text` - Object type doesn't support text
- `object does not contain editable text: tables must be modified cell by cell` - Table modification
- `object not found: object 'xyz' not found in presentation` - Invalid object ID
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify

---

#### `style_text`

Apply styling to text in a shape (font, size, color, bold, italic, etc.).

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "shape-789",
  "start_index": 0,
  "end_index": 10,
  "style": {
    "font_family": "Arial",
    "font_size": 24,
    "bold": true,
    "italic": false,
    "underline": true,
    "strikethrough": false,
    "foreground_color": "#FF0000",
    "background_color": "#FFFF00",
    "link_url": "https://example.com"
  }
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `object_id` | string | Yes | The ID of the shape containing text |
| `start_index` | integer | No | Start character index (0-based). Omit for whole text |
| `end_index` | integer | No | End character index (exclusive). Omit for whole text |
| `style` | object | Yes | Style properties to apply |
| `style.font_family` | string | No | Font family name (e.g., "Arial", "Times New Roman") |
| `style.font_size` | integer | No | Font size in points |
| `style.bold` | boolean | No | Apply bold formatting |
| `style.italic` | boolean | No | Apply italic formatting |
| `style.underline` | boolean | No | Apply underline formatting |
| `style.strikethrough` | boolean | No | Apply strikethrough formatting |
| `style.foreground_color` | string | No | Text color as hex (e.g., "#FF0000") |
| `style.background_color` | string | No | Text highlight color as hex (e.g., "#FFFF00") |
| `style.link_url` | string | No | URL to create hyperlink |

**Output:**
```json
{
  "object_id": "shape-789",
  "applied_styles": [
    "font_family=Arial",
    "font_size=24pt",
    "bold=true",
    "italic=false",
    "underline=true",
    "strikethrough=false",
    "foreground_color=#FF0000",
    "background_color=#FFFF00",
    "link_url=https://example.com"
  ],
  "text_range": "FIXED_RANGE (0-10)"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | The styled object's ID |
| `applied_styles` | array | List of style properties that were applied |
| `text_range` | string | Range description: "ALL" or "FIXED_RANGE (start-end)" |

**Features:**
- Apply multiple style properties in a single call
- Style entire text content or specific character ranges
- Support for all standard text formatting options
- Boolean values distinguish between "set to false" and "not set"
- Colors specified as hex strings (#RRGGBB format)
- Creates clickable hyperlinks with link_url

**Use Cases:**
- Formatting titles with bold and larger font
- Highlighting important text with colors
- Adding hyperlinks to text
- Applying consistent styling across text elements
- Removing formatting by setting properties to false

**Examples:**

Make text bold and red:
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "shape-789",
  "style": {
    "bold": true,
    "foreground_color": "#FF0000"
  }
}
```

Style specific characters (first 5 characters):
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "shape-789",
  "start_index": 0,
  "end_index": 5,
  "style": {
    "font_size": 36,
    "underline": true
  }
}
```

Add hyperlink to text:
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "shape-789",
  "style": {
    "link_url": "https://example.com",
    "foreground_color": "#0000FF",
    "underline": true
  }
}
```

**Errors:**
- `no style properties provided` - Style object is empty or missing
- `invalid object_id: object_id is required` - Missing object ID
- `invalid text range: start_index cannot be negative` - Invalid index
- `invalid text range: end_index cannot be negative` - Invalid index
- `invalid text range: start_index cannot be greater than end_index` - Invalid range
- `object does not contain text` - Object type doesn't support text styling
- `object does not contain editable text: tables must be styled cell by cell` - Table styling
- `object not found: object 'xyz' not found in presentation` - Invalid object ID
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify

---

#### `format_paragraph`

Set paragraph formatting options like alignment, spacing, and indentation.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "textbox_123",
  "paragraph_index": 0,
  "formatting": {
    "alignment": "CENTER",
    "line_spacing": 150,
    "space_above": 12,
    "space_below": 12,
    "indent_first_line": 36,
    "indent_start": 18,
    "indent_end": 18
  }
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `object_id` | string | Yes | ID of the shape containing text |
| `paragraph_index` | integer | No | 0-based index of paragraph to format (all if omitted) |
| `formatting` | object | Yes | Formatting options to apply |

**Formatting Options:**

| Property | Type | Description |
|----------|------|-------------|
| `alignment` | string | Text alignment: `START`, `CENTER`, `END`, `JUSTIFIED` |
| `line_spacing` | number | Line spacing as percentage (100 = single, 150 = 1.5 lines, 200 = double) |
| `space_above` | number | Space above paragraph in points |
| `space_below` | number | Space below paragraph in points |
| `indent_first_line` | number | First line indent in points |
| `indent_start` | number | Left indent in points (for LTR text) |
| `indent_end` | number | Right indent in points (for LTR text) |

**Output:**
```json
{
  "object_id": "textbox_123",
  "applied_formatting": [
    "alignment=CENTER",
    "line_spacing=150.0%",
    "space_above=12.0pt"
  ],
  "paragraph_scope": "ALL"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | ID of the formatted object |
| `applied_formatting` | array | List of formatting properties that were applied |
| `paragraph_scope` | string | `"ALL"` or `"INDEX (N)"` indicating which paragraphs were formatted |

**Example - Center align all paragraphs:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "textbox_123",
  "formatting": {
    "alignment": "CENTER"
  }
}
```

**Example - Format specific paragraph with multiple options:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "textbox_123",
  "paragraph_index": 1,
  "formatting": {
    "alignment": "JUSTIFIED",
    "line_spacing": 150,
    "indent_first_line": 36
  }
}
```

**Errors:**
- `no formatting properties provided` - Formatting object is empty or missing
- `invalid alignment value: must be START, CENTER, END, or JUSTIFIED` - Invalid alignment
- `invalid paragraph_index: paragraph_index cannot be negative` - Negative index
- `invalid paragraph_index: paragraph index N is out of range (object has M paragraphs)` - Index too large
- `object does not contain text` - Object type doesn't support text formatting
- `object does not contain editable text: tables must be formatted cell by cell` - Table formatting
- `object not found: object 'xyz' not found in presentation` - Invalid object ID
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify

---

#### `create_bullet_list`

Convert text to a bullet list or add bullets to existing text in a shape.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "textbox_123",
  "bullet_style": "DISC",
  "bullet_color": "#FF0000",
  "paragraph_indices": [0, 2]
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `object_id` | string | Yes | ID of the shape containing text |
| `bullet_style` | string | Yes | Bullet style name or full preset name |
| `bullet_color` | string | No | Hex color for bullets (e.g., `#FF0000`) |
| `paragraph_indices` | array | No | 0-based indices of paragraphs to apply bullets to (all if omitted) |

**Bullet Styles:**

| User-Friendly Name | API Preset | Description |
|--------------------|------------|-------------|
| `DISC` | BULLET_DISC_CIRCLE_SQUARE | Filled circle bullets |
| `CIRCLE` | BULLET_DISC_CIRCLE_SQUARE | Same as DISC |
| `SQUARE` | BULLET_DISC_CIRCLE_SQUARE | Same as DISC |
| `DIAMOND` | BULLET_DIAMOND_CIRCLE_SQUARE | Diamond bullets |
| `ARROW` | BULLET_ARROW_DIAMOND_DISC | Arrow bullets |
| `STAR` | BULLET_STAR_CIRCLE_SQUARE | Star bullets |
| `CHECKBOX` | BULLET_CHECKBOX | Checkbox bullets |

Full preset names are also accepted (e.g., `BULLET_DISC_CIRCLE_SQUARE`).

**Output:**
```json
{
  "object_id": "textbox_123",
  "bullet_preset": "BULLET_DISC_CIRCLE_SQUARE",
  "paragraph_scope": "ALL",
  "bullet_color": "#FF0000"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | ID of the modified object |
| `bullet_preset` | string | The actual API preset that was applied |
| `paragraph_scope` | string | `"ALL"` or `"INDICES [0, 2]"` indicating which paragraphs received bullets |
| `bullet_color` | string | The color applied (only present if color was specified) |

**Example - Apply disc bullets to all paragraphs:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "textbox_123",
  "bullet_style": "DISC"
}
```

**Example - Apply colored star bullets:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "textbox_123",
  "bullet_style": "STAR",
  "bullet_color": "#FFD700"
}
```

**Example - Apply checkbox bullets to specific paragraphs:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "textbox_123",
  "bullet_style": "CHECKBOX",
  "paragraph_indices": [0, 2, 4]
}
```

**Errors:**
- `invalid bullet_style: bullet_style is required` - Empty bullet style
- `invalid bullet_style: 'XYZ' is not a valid bullet style` - Invalid style name
- `invalid paragraph_index: paragraph indices cannot be negative` - Negative index
- `invalid paragraph_index: paragraph index N is out of range (object has M paragraphs)` - Index too large
- `object does not contain text` - Object type doesn't support bullets
- `object does not contain editable text: tables must have bullets applied cell by cell` - Table object
- `object not found: object 'xyz' not found in presentation` - Invalid object ID
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify

---

#### `create_numbered_list`

Convert text to a numbered list or add numbering to existing text in a shape.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "textbox_123",
  "number_style": "DECIMAL",
  "start_number": 1,
  "paragraph_indices": [0, 2]
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `object_id` | string | Yes | ID of the shape containing text |
| `number_style` | string | Yes | Number style name or full preset name |
| `start_number` | integer | No | Starting number (default: 1) |
| `paragraph_indices` | array | No | 0-based indices of paragraphs to apply numbering to (all if omitted) |

**Number Styles:**

| User-Friendly Name | API Preset | Description |
|--------------------|------------|-------------|
| `DECIMAL` | NUMBERED_DECIMAL_ALPHA_ROMAN | 1, 2, 3... |
| `ALPHA_UPPER` | NUMBERED_UPPERALPHA_ALPHA_ROMAN | A, B, C... |
| `ALPHA_LOWER` | NUMBERED_ALPHA_ALPHA_ROMAN | a, b, c... |
| `ROMAN_UPPER` | NUMBERED_UPPERROMAN_UPPERALPHA_DECIMAL | I, II, III... |
| `ROMAN_LOWER` | NUMBERED_ROMAN_UPPERALPHA_DECIMAL | i, ii, iii... |

Full preset names are also accepted (e.g., `NUMBERED_DECIMAL_NESTED`, `NUMBERED_DECIMAL_ALPHA_ROMAN_PARENS`).

**Output:**
```json
{
  "object_id": "textbox_123",
  "number_preset": "NUMBERED_DECIMAL_ALPHA_ROMAN",
  "paragraph_scope": "ALL",
  "start_number": 1
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | ID of the modified object |
| `number_preset` | string | The actual API preset that was applied |
| `paragraph_scope` | string | `"ALL"` or `"INDICES [0, 2]"` indicating which paragraphs received numbering |
| `start_number` | integer | The start number that was applied |

**Example - Apply decimal numbering to all paragraphs:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "textbox_123",
  "number_style": "DECIMAL"
}
```

**Example - Apply uppercase Roman numerals:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "textbox_123",
  "number_style": "ROMAN_UPPER"
}
```

**Example - Apply lowercase alphabetic numbering to specific paragraphs:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "textbox_123",
  "number_style": "ALPHA_LOWER",
  "paragraph_indices": [0, 2, 4]
}
```

**Errors:**
- `invalid number_style: number_style is required` - Empty number style
- `invalid number_style: 'XYZ' is not a valid number style` - Invalid style name
- `invalid start_number: start_number must be at least 1` - Start number less than 1
- `invalid paragraph_index: paragraph indices cannot be negative` - Negative index
- `invalid paragraph_index: paragraph index N is out of range (object has M paragraphs)` - Index too large
- `object does not contain text` - Object type doesn't support numbering
- `object does not contain editable text: tables must have numbering applied cell by cell` - Table object
- `object not found: object 'xyz' not found in presentation` - Invalid object ID
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify

---

#### `modify_list`

Modify existing list properties, remove list formatting, or change indentation.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "textbox_123",
  "action": "modify",
  "paragraph_indices": [0, 2],
  "properties": {
    "bullet_style": "STAR",
    "color": "#FF0000"
  }
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `object_id` | string | Yes | ID of the shape containing text |
| `action` | string | Yes | Action to perform: `modify`, `remove`, `increase_indent`, `decrease_indent` |
| `paragraph_indices` | array | No | 0-based indices of paragraphs to apply action to (all if omitted) |
| `properties` | object | For `modify` | Properties to modify (required for `modify` action) |

**Actions:**

| Action | Description |
|--------|-------------|
| `modify` | Change bullet style, number style, or color (requires `properties`) |
| `remove` | Remove list formatting, convert to plain text |
| `increase_indent` | Increase indentation by 18 points (nest deeper) |
| `decrease_indent` | Decrease indentation by 18 points (minimum 0) |

**Properties (for `modify` action):**

| Property | Type | Description |
|----------|------|-------------|
| `bullet_style` | string | New bullet style: `DISC`, `CIRCLE`, `SQUARE`, `DIAMOND`, `ARROW`, `STAR`, `CHECKBOX` |
| `number_style` | string | New number style: `DECIMAL`, `ALPHA_UPPER`, `ALPHA_LOWER`, `ROMAN_UPPER`, `ROMAN_LOWER` |
| `color` | string | Hex color for bullets/numbers (e.g., `#FF0000`) |

At least one property must be provided for `modify` action.

**Output:**
```json
{
  "object_id": "textbox_123",
  "action": "modify",
  "paragraph_scope": "ALL",
  "result": "Modified: bullet_style=BULLET_STAR_CIRCLE_SQUARE, color=#FF0000"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | ID of the modified object |
| `action` | string | The action that was performed |
| `paragraph_scope` | string | `"ALL"` or `"INDICES [0, 2]"` indicating affected paragraphs |
| `result` | string | Description of what was changed |

**Example - Remove list formatting:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "textbox_123",
  "action": "remove"
}
```

**Example - Increase indentation:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "textbox_123",
  "action": "increase_indent"
}
```

**Example - Change bullet style to star:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "textbox_123",
  "action": "modify",
  "properties": {
    "bullet_style": "STAR"
  }
}
```

**Example - Change list color for specific paragraphs:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "textbox_123",
  "action": "modify",
  "paragraph_indices": [0, 2],
  "properties": {
    "color": "#FF0000"
  }
}
```

**Errors:**
- `invalid list action: action must be 'modify', 'remove', 'increase_indent', or 'decrease_indent'` - Invalid action
- `no list properties provided: properties are required for 'modify' action` - Missing properties for modify
- `no list properties provided: at least one property (bullet_style, number_style, or color) must be provided` - Empty properties
- `invalid bullet_style: 'XYZ' is not a valid bullet style` - Invalid bullet style
- `invalid number_style: 'XYZ' is not a valid number style` - Invalid number style
- `invalid paragraph_index: paragraph indices cannot be negative` - Negative index
- `invalid paragraph_index: paragraph index N is out of range (object has M paragraphs)` - Index too large
- `object does not contain text` - Object type doesn't support text
- `object does not contain editable text: tables must have list properties modified cell by cell` - Table object
- `object not found: object 'xyz' not found in presentation` - Invalid object ID
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify

---

#### `search_text`

Search for text across all slides in a presentation.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "query": "important keyword",
  "case_sensitive": false
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `query` | string | Yes | Text to search for |
| `case_sensitive` | boolean | No | Enable case-sensitive search (default: false) |

**Output:**
```json
{
  "presentation_id": "abc123xyz",
  "query": "important keyword",
  "case_sensitive": false,
  "total_matches": 5,
  "results": [
    {
      "slide_index": 1,
      "slide_id": "g123456",
      "matches": [
        {
          "object_id": "shape-789",
          "object_type": "TEXT_BOX",
          "start_index": 25,
          "text_context": "...text before the important keyword and text after..."
        }
      ]
    },
    {
      "slide_index": 3,
      "slide_id": "g789012",
      "matches": [
        {
          "object_id": "table-1[0,2]",
          "object_type": "TABLE_CELL",
          "start_index": 5,
          "text_context": "This important keyword appears in a table cell"
        }
      ]
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `presentation_id` | string | The searched presentation ID |
| `query` | string | The search query used |
| `case_sensitive` | boolean | Whether search was case-sensitive |
| `total_matches` | integer | Total number of matches found |
| `results` | array | Results grouped by slide |
| `results[].slide_index` | integer | 1-based slide index |
| `results[].slide_id` | string | Object ID of the slide |
| `results[].matches` | array | Matches found on this slide |
| `matches[].object_id` | string | ID of object containing match (for tables: includes cell position) |
| `matches[].object_type` | string | Type of object (TEXT_BOX, TABLE_CELL, SPEAKER_NOTES:TEXT_BOX, etc.) |
| `matches[].start_index` | integer | Character position where match begins |
| `matches[].text_context` | string | Surrounding text (50 chars before/after the match) |

**Features:**
- Searches all slides, text shapes, tables, and speaker notes
- Case-insensitive by default (configurable with `case_sensitive`)
- Includes surrounding context (50 characters) for each match
- Results grouped by slide for easy navigation
- Searches recursively through grouped elements
- Table cell matches include row/column position in object_id
- Speaker notes matches are prefixed with "SPEAKER_NOTES:" in object_type

**Use Cases:**
- Finding specific content in a presentation
- Locating all occurrences of a term before replacement
- Reviewing where a keyword appears across slides
- Finding content in speaker notes

**Examples:**

Case-insensitive search (default):
```json
{
  "presentation_id": "abc123",
  "query": "revenue"
}
```

Case-sensitive search:
```json
{
  "presentation_id": "abc123",
  "query": "Revenue",
  "case_sensitive": true
}
```

**Errors:**
- `invalid query: query is required` - Empty search query
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to access

---

#### `replace_text`

Find and replace text across a presentation.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "find": "{{name}}",
  "replace_with": "John Doe",
  "case_sensitive": false,
  "scope": "all"
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `find` | string | Yes | Text to search for |
| `replace_with` | string | Yes | Replacement text (empty string to delete matches) |
| `case_sensitive` | boolean | No | Enable case-sensitive matching (default: false) |
| `scope` | string | No | Scope of replacement: "all", "slide", "object" (default: "all") |
| `slide_id` | string | Conditional | Required when scope is "slide" |
| `object_id` | string | Conditional | Required when scope is "object" |

**Output:**
```json
{
  "presentation_id": "abc123xyz",
  "find": "{{name}}",
  "replace_with": "John Doe",
  "case_sensitive": false,
  "scope": "all",
  "replacement_count": 5,
  "affected_objects": [
    {
      "slide_index": 1,
      "slide_id": "g123456",
      "object_id": "shape-789",
      "object_type": "TEXT_BOX"
    },
    {
      "slide_index": 2,
      "slide_id": "g456789",
      "object_id": "table-1",
      "object_type": "TABLE"
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `presentation_id` | string | The modified presentation ID |
| `find` | string | The search term used |
| `replace_with` | string | The replacement text |
| `case_sensitive` | boolean | Whether matching was case-sensitive |
| `scope` | string | The scope used for replacement |
| `replacement_count` | integer | Total number of replacements made |
| `affected_objects` | array | Objects that contained matches |
| `affected_objects[].slide_index` | integer | 1-based slide index |
| `affected_objects[].slide_id` | string | Object ID of the containing slide |
| `affected_objects[].object_id` | string | Object ID of the modified element |
| `affected_objects[].object_type` | string | Type of modified object (TEXT_BOX, TABLE, etc.) |

**Features:**
- Uses Google Slides `ReplaceAllTextRequest` API for efficient bulk replacement
- Scope `all` replaces across entire presentation (default)
- Scope `slide` limits to a specific slide
- Scope `object` limits to the slide containing a specific object
- Case-insensitive by default (configurable)
- Empty `replace_with` effectively deletes all matches
- Reports all affected objects for verification

**Use Cases:**
- Template mail merge (replacing placeholders like {{name}})
- Bulk text updates across presentation
- Removing draft watermarks or placeholder text
- Correcting typos or outdated information
- Updating company names or product names

**Examples:**

Replace template placeholders:
```json
{
  "presentation_id": "abc123",
  "find": "{{company_name}}",
  "replace_with": "Acme Corporation"
}
```

Case-sensitive replacement on specific slide:
```json
{
  "presentation_id": "abc123",
  "find": "Q3",
  "replace_with": "Q4",
  "case_sensitive": true,
  "scope": "slide",
  "slide_id": "g456789"
}
```

Delete text (replace with empty):
```json
{
  "presentation_id": "abc123",
  "find": "[DRAFT]",
  "replace_with": ""
}
```

**Errors:**
- `find text is required: find text cannot be empty` - Empty find text
- `invalid scope: scope must be 'all', 'slide', or 'object'` - Invalid scope value
- `invalid scope: slide_id is required when scope is 'slide'` - Missing slide_id for slide scope
- `invalid scope: object_id is required when scope is 'object'` - Missing object_id for object scope
- `slide not found` - Specified slide_id doesn't exist
- `object not found` - Specified object_id doesn't exist
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify

---

*More tools to be documented:*

### Content Manipulation
- `add_text` - Add text to existing placeholders
- `style_text` - Apply text formatting
- `format_paragraph` - Set paragraph styles
- `create_bullet_list` - Convert text to bullet lists
- `create_numbered_list` - Convert text to numbered lists
- `modify_list` - Modify list properties, remove formatting, or change indentation

### Media and Objects

#### `add_image`

Add an image to a slide from base64-encoded data.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "slide_index": 1,
  "image_base64": "iVBORw0KGgoAAAANSUhEUgAA...",
  "position": {"x": 100, "y": 50},
  "size": {"width": 300, "height": 200}
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `slide_index` | integer | No* | 1-based index of the target slide |
| `slide_id` | string | No* | Object ID of the target slide |
| `image_base64` | string | Yes | Base64-encoded image data |
| `position` | object | No | Position in points (default: 0, 0) |
| `position.x` | number | No | X coordinate in points from left edge |
| `position.y` | number | No | Y coordinate in points from top edge |
| `size` | object | No | Size in points (optional, preserves aspect ratio) |
| `size.width` | number | No | Width in points (optional) |
| `size.height` | number | No | Height in points (optional) |

*Either `slide_index` or `slide_id` must be provided.

**Output:**
```json
{
  "object_id": "image_1234567890123456"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | Unique identifier of the created image |

**Supported Image Formats:**
- PNG (image/png)
- JPEG (image/jpeg)
- GIF (image/gif)
- WebP (image/webp)
- BMP (image/bmp)

**Features:**
- Uses either 1-based slide index or slide ID for flexibility
- Automatically detects image MIME type from magic bytes
- Uploads image to Google Drive first for persistence
- Makes uploaded file publicly accessible so Slides can display it
- Position defaults to (0, 0) if not specified
- Size is optional - if omitted, uses original image dimensions
- If only width or height specified, aspect ratio is preserved
- Standard slide dimensions: 720x405 points
- 1 point = 12700 EMU (English Metric Units)

**Use Cases:**
- Adding logos to presentations
- Inserting charts or diagrams
- Adding screenshots or photos
- Creating image galleries on slides
- Programmatic presentation generation with images

**Examples:**

Add an image with default size:
```json
{
  "presentation_id": "abc123",
  "slide_index": 1,
  "image_base64": "iVBORw0KGgoAAAANSUhEUgAA..."
}
```

Add an image at specific position:
```json
{
  "presentation_id": "abc123",
  "slide_id": "g123456",
  "image_base64": "iVBORw0KGgoAAAANSUhEUgAA...",
  "position": {"x": 100, "y": 150}
}
```

Add an image with specific size:
```json
{
  "presentation_id": "abc123",
  "slide_index": 2,
  "image_base64": "iVBORw0KGgoAAAANSUhEUgAA...",
  "position": {"x": 50, "y": 100},
  "size": {"width": 400, "height": 300}
}
```

Add an image with width only (preserves aspect ratio):
```json
{
  "presentation_id": "abc123",
  "slide_index": 1,
  "image_base64": "iVBORw0KGgoAAAANSUhEUgAA...",
  "size": {"width": 200}
}
```

**Errors:**
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `invalid slide reference: either slide_index or slide_id is required` - Neither slide reference provided
- `invalid image data: image_base64 is required` - Missing image data
- `invalid image data: base64 decoding failed` - Invalid base64 encoding
- `invalid image data: unable to detect image format` - Unrecognized image format
- `invalid image size: size must have positive width and/or height` - Size dimensions ≤ 0
- `invalid image position: position coordinates must be non-negative` - Negative position values
- `failed to upload image to Drive` - Drive API upload error
- `slide not found` - Slide index out of range or slide ID not found
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify
- `failed to add image` - API error during image creation

---

#### `modify_image`

Modify properties of an existing image in a presentation.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "image_1234567890",
  "properties": {
    "position": {"x": 100, "y": 50},
    "size": {"width": 300, "height": 200},
    "crop": {"top": 0.1, "bottom": 0.1, "left": 0.05, "right": 0.05},
    "brightness": 0.3,
    "contrast": -0.2,
    "transparency": 0.5,
    "recolor": "GRAYSCALE"
  }
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `object_id` | string | Yes | Object ID of the image to modify |
| `properties` | object | Yes | Properties to modify (at least one required) |
| `properties.position` | object | No | New position in points |
| `properties.position.x` | number | No | X coordinate in points from left edge |
| `properties.position.y` | number | No | Y coordinate in points from top edge |
| `properties.size` | object | No | New size in points |
| `properties.size.width` | number | No | Width in points |
| `properties.size.height` | number | No | Height in points |
| `properties.crop` | object | No | Crop percentages (0-1) |
| `properties.crop.top` | number | No | Percentage to crop from top (0-1) |
| `properties.crop.bottom` | number | No | Percentage to crop from bottom (0-1) |
| `properties.crop.left` | number | No | Percentage to crop from left (0-1) |
| `properties.crop.right` | number | No | Percentage to crop from right (0-1) |
| `properties.brightness` | number | No | Brightness adjustment (-1 to 1) |
| `properties.contrast` | number | No | Contrast adjustment (-1 to 1) |
| `properties.transparency` | number | No | Transparency level (0 to 1, where 0 is opaque) |
| `properties.recolor` | string | No | Recolor preset name or "none" to remove |

**Output:**
```json
{
  "object_id": "image_1234567890",
  "modified_properties": ["position", "brightness", "contrast"]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | The modified image's object ID |
| `modified_properties` | array | List of properties that were modified |

**Recolor Presets:**
| Preset | Description |
|--------|-------------|
| `GRAYSCALE` | Grayscale recolor |
| `SEPIA` | Sepia tone |
| `NEGATIVE` | Negative colors |
| `LIGHT1` - `LIGHT10` | Light theme variations |
| `DARK1` - `DARK10` | Dark theme variations |
| `NONE` | Remove recolor effect |

**Features:**
- Modify multiple properties in a single call
- Position and size use UpdatePageElementTransformRequest with ABSOLUTE mode
- Image properties use UpdateImagePropertiesRequest with field masks
- All properties are optional but at least one must be specified
- Validates all property ranges before making API calls
- Recolor "none" or "NONE" removes any existing recolor effect
- Position and size changes preserve other transform properties
- Standard slide dimensions: 720x405 points
- 1 point = 12700 EMU (English Metric Units)

**Use Cases:**
- Repositioning images after initial placement
- Resizing images to fit layout
- Cropping images to focus on specific areas
- Adjusting image brightness and contrast
- Making images semi-transparent for watermarks or overlays
- Applying visual effects with recolor presets
- Removing previously applied recolor effects

**Examples:**

Move and resize an image:
```json
{
  "presentation_id": "abc123",
  "object_id": "image_xyz",
  "properties": {
    "position": {"x": 150, "y": 100},
    "size": {"width": 400, "height": 300}
  }
}
```

Crop and adjust brightness/contrast:
```json
{
  "presentation_id": "abc123",
  "object_id": "image_xyz",
  "properties": {
    "crop": {"top": 0.1, "left": 0.1, "right": 0.1, "bottom": 0.1},
    "brightness": 0.3,
    "contrast": -0.2
  }
}
```

Apply grayscale recolor:
```json
{
  "presentation_id": "abc123",
  "object_id": "image_xyz",
  "properties": {
    "recolor": "GRAYSCALE"
  }
}
```

Remove recolor effect:
```json
{
  "presentation_id": "abc123",
  "object_id": "image_xyz",
  "properties": {
    "recolor": "none"
  }
}
```

Set transparency for watermark effect:
```json
{
  "presentation_id": "abc123",
  "object_id": "image_xyz",
  "properties": {
    "transparency": 0.7
  }
}
```

**Errors:**
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `object not found: object 'xyz' not found in presentation` - Object doesn't exist
- `object is not an image: object 'xyz' is not an image (type: TEXT_BOX)` - Object is not an image
- `no image properties to modify` - Properties object is empty or nil
- `crop values must be between 0 and 1: top crop value X is invalid` - Invalid crop value
- `brightness must be between -1 and 1` - Invalid brightness value
- `contrast must be between -1 and 1` - Invalid contrast value
- `transparency must be between 0 and 1` - Invalid transparency value
- `invalid image size: size must have positive width and/or height` - Size dimensions ≤ 0
- `invalid image position: position coordinates must be non-negative` - Negative position values
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify
- `failed to modify image` - API error during modification

#### `replace_image`

Replace an existing image with a new one while optionally preserving size and position.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "image_1234567890",
  "image_base64": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
  "preserve_size": true
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `object_id` | string | Yes | Object ID of the image to replace |
| `image_base64` | string | Yes | Base64-encoded image data |
| `preserve_size` | boolean | No | Whether to preserve original size (default: true) |

**Output:**
```json
{
  "object_id": "image_1234567890",
  "new_object_id": "image_9876543210",
  "preserved_size": true
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | The original image's object ID |
| `new_object_id` | string | The new image's object ID (always set since replacement creates new object) |
| `preserved_size` | boolean | Whether the original size was preserved |

**Supported Image Formats:**
| Format | Magic Bytes |
|--------|-------------|
| PNG | `89 50 4E 47` |
| JPEG | `FF D8 FF` |
| GIF | `47 49 46` (GIF) |
| WebP | `52 49 46 46...57 45 42 50` (RIFF...WEBP) |
| BMP | `42 4D` (BM) |

**Features:**
- Replaces image content by deleting old image and creating new one at same position
- Preserves original position via AffineTransform (translateX, translateY, scale, shear)
- Preserves original size when preserve_size is true (default)
- Uploads new image to Google Drive first, then references it in Slides API
- Makes uploaded image publicly accessible for Slides API to read
- Automatically detects image MIME type from magic bytes
- Returns new object ID since image replacement creates a new object

**Use Cases:**
- Updating placeholder images with final versions
- Replacing logos or branding images across presentations
- Swapping out outdated screenshots or diagrams
- Updating charts or graphs with new data visualizations
- Replacing draft images with production-ready versions

**Examples:**

Replace image preserving original size:
```json
{
  "presentation_id": "abc123",
  "object_id": "image_xyz",
  "image_base64": "iVBORw0KGgo...",
  "preserve_size": true
}
```

Replace image using new image's natural size:
```json
{
  "presentation_id": "abc123",
  "object_id": "image_xyz",
  "image_base64": "iVBORw0KGgo...",
  "preserve_size": false
}
```

**Errors:**
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `object not found: object_id is required` - Empty object ID
- `invalid image data: image_base64 is required` - Empty image data
- `invalid image data: illegal base64 data` - Malformed base64 encoding
- `invalid image data: unable to detect image format` - Unrecognized image format
- `object not found: object 'xyz' not found in presentation` - Object doesn't exist
- `object is not an image: object 'xyz' is not an image (type: TEXT_BOX)` - Object is not an image
- `failed to upload image` - Drive API upload failed
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify
- `failed to replace image` - API error during replacement

---

#### `create_shape`

Create a shape on a slide with optional fill and outline styling.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "slide_index": 1,
  "shape_type": "RECTANGLE",
  "position": {"x": 100, "y": 50},
  "size": {"width": 200, "height": 100},
  "fill_color": "#007FFF",
  "outline_color": "#000000",
  "outline_weight": 2
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `slide_index` | integer | No* | 1-based index of the target slide |
| `slide_id` | string | No* | Object ID of the target slide |
| `shape_type` | string | Yes | Shape type (RECTANGLE, ELLIPSE, etc.) |
| `position` | object | No | Position in points (default: 0, 0) |
| `position.x` | number | No | X coordinate in points from left edge |
| `position.y` | number | No | Y coordinate in points from top edge |
| `size` | object | Yes | Size in points |
| `size.width` | number | Yes | Width in points (must be positive) |
| `size.height` | number | Yes | Height in points (must be positive) |
| `fill_color` | string | No | Fill color as hex string (#RRGGBB) or "transparent" |
| `outline_color` | string | No | Outline color as hex string (#RRGGBB) or "transparent" |
| `outline_weight` | number | No | Outline weight in points (must be positive) |

*Either `slide_index` or `slide_id` must be provided.

**Output:**
```json
{
  "object_id": "shape_1234567890123456"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | Unique identifier of the created shape |

**Supported Shape Types:**

| Category | Shape Types |
|----------|-------------|
| Basic | `RECTANGLE`, `ROUND_RECTANGLE`, `ELLIPSE`, `TRIANGLE`, `DIAMOND`, `PENTAGON`, `HEXAGON`, `HEPTAGON`, `OCTAGON`, `DECAGON`, `DODECAGON`, `PARALLELOGRAM`, `TRAPEZOID` |
| Stars | `STAR_4`, `STAR_5`, `STAR_6`, `STAR_7`, `STAR_8`, `STAR_10`, `STAR_12`, `STAR_16`, `STAR_24`, `STAR_32` |
| Arrows | `ARROW_RIGHT`, `ARROW_LEFT`, `ARROW_UP`, `ARROW_DOWN`, `ARROW_LEFT_RIGHT`, `ARROW_UP_DOWN`, `NOTCHED_RIGHT_ARROW`, `BENT_ARROW`, `U_TURN_ARROW`, `CURVED_RIGHT_ARROW`, `CURVED_LEFT_ARROW`, `CURVED_UP_ARROW`, `CURVED_DOWN_ARROW`, `STRIPED_RIGHT_ARROW`, `CHEVRON`, `HOME_PLATE` |
| Callouts | `RECTANGULAR_CALLOUT`, `ROUNDED_RECTANGULAR_CALLOUT`, `ELLIPTICAL_CALLOUT`, `WEDGE_RECTANGLE_CALLOUT`, `WEDGE_ROUND_RECT_CALLOUT`, `WEDGE_ELLIPSE_CALLOUT`, `CLOUD_CALLOUT` |
| Flowchart | `FLOWCHART_PROCESS`, `FLOWCHART_DECISION`, `FLOWCHART_INPUT_OUTPUT`, `FLOWCHART_PREDEFINED_PROCESS`, `FLOWCHART_INTERNAL_STORAGE`, `FLOWCHART_DOCUMENT`, `FLOWCHART_MULTIDOCUMENT`, `FLOWCHART_TERMINATOR`, `FLOWCHART_PREPARATION`, `FLOWCHART_MANUAL_INPUT`, `FLOWCHART_MANUAL_OPERATION`, `FLOWCHART_CONNECTOR`, `FLOWCHART_PUNCHED_CARD`, `FLOWCHART_PUNCHED_TAPE`, `FLOWCHART_SUMMING_JUNCTION`, `FLOWCHART_OR`, `FLOWCHART_COLLATE`, `FLOWCHART_SORT`, `FLOWCHART_EXTRACT`, `FLOWCHART_MERGE`, `FLOWCHART_OFFLINE_STORAGE`, `FLOWCHART_ONLINE_STORAGE`, `FLOWCHART_MAGNETIC_TAPE`, `FLOWCHART_MAGNETIC_DISK`, `FLOWCHART_MAGNETIC_DRUM`, `FLOWCHART_DISPLAY`, `FLOWCHART_DELAY`, `FLOWCHART_ALTERNATE_PROCESS`, `FLOWCHART_DATA` |
| Equation | `PLUS`, `MINUS`, `MULTIPLY`, `DIVIDE`, `EQUAL`, `NOT_EQUAL` |
| Block | `CUBE`, `CAN`, `BEVEL`, `FOLDED_CORNER`, `SMILEY_FACE`, `DONUT`, `NO_SMOKING`, `BLOCK_ARC`, `HEART`, `LIGHTNING_BOLT`, `SUN`, `MOON`, `CLOUD`, `ARC`, `PLAQUE`, `FRAME`, `HALF_FRAME`, `CORNER`, `DIAGONAL_STRIPE`, `CHORD`, `PIE`, `L_SHAPE`, `CORNER_RIBBON`, `RIBBON`, `RIBBON_2`, `WAVE`, `DOUBLE_WAVE`, `CROSS`, `IRREGULAR_SEAL_1`, `IRREGULAR_SEAL_2`, `TEARDROP` |
| Rectangles | `SNIP_1_RECTANGLE`, `SNIP_2_SAME_RECTANGLE`, `SNIP_2_DIAGONAL_RECTANGLE`, `SNIP_ROUND_RECTANGLE`, `ROUND_1_RECTANGLE`, `ROUND_2_SAME_RECTANGLE`, `ROUND_2_DIAGONAL_RECTANGLE` |
| Brackets | `LEFT_BRACKET`, `RIGHT_BRACKET`, `LEFT_BRACE`, `RIGHT_BRACE`, `LEFT_RIGHT_BRACKET`, `BRACKET_PAIR`, `BRACE_PAIR` |

**Features:**
- Uses either 1-based slide index or slide ID for flexibility
- Shape type names are case-insensitive (normalized to uppercase)
- Position defaults to (0, 0) if not specified
- Fill color supports hex colors or "transparent" for no fill
- Outline color supports hex colors or "transparent" for no outline
- Outline weight specifies the border thickness in points
- Standard slide dimensions: 720x405 points
- 1 point = 12700 EMU (English Metric Units)

**Use Cases:**
- Creating diagrams and flowcharts
- Adding decorative shapes to slides
- Building organizational charts
- Creating callout annotations
- Drawing process flows

**Examples:**

Create a simple rectangle:
```json
{
  "presentation_id": "abc123",
  "slide_index": 1,
  "shape_type": "RECTANGLE",
  "position": {"x": 100, "y": 100},
  "size": {"width": 200, "height": 100}
}
```

Create a star with fill color:
```json
{
  "presentation_id": "abc123",
  "slide_id": "g123456",
  "shape_type": "STAR_5",
  "position": {"x": 300, "y": 150},
  "size": {"width": 100, "height": 100},
  "fill_color": "#FFD700"
}
```

Create an arrow with outline:
```json
{
  "presentation_id": "abc123",
  "slide_index": 2,
  "shape_type": "ARROW_RIGHT",
  "position": {"x": 50, "y": 200},
  "size": {"width": 150, "height": 50},
  "fill_color": "#007FFF",
  "outline_color": "#000000",
  "outline_weight": 2
}
```

Create a transparent shape with border only:
```json
{
  "presentation_id": "abc123",
  "slide_index": 1,
  "shape_type": "ELLIPSE",
  "position": {"x": 200, "y": 100},
  "size": {"width": 150, "height": 150},
  "fill_color": "transparent",
  "outline_color": "#FF0000",
  "outline_weight": 3
}
```

**Errors:**
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `invalid slide reference: either slide_index or slide_id is required` - Neither slide reference provided
- `invalid shape type: shape_type is required` - Missing shape type
- `invalid shape type: 'X' is not a valid shape type` - Unsupported shape type
- `invalid size: size is required with positive width and height` - Missing or invalid size
- `outline weight must be positive` - Outline weight ≤ 0
- `slide not found` - Slide index out of range or slide ID not found
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify
- `failed to create shape` - API error during shape creation

---

#### `create_table`

Create a table on a slide with specified rows and columns.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "slide_index": 1,
  "rows": 3,
  "columns": 4,
  "position": {"x": 100, "y": 50},
  "size": {"width": 400, "height": 200}
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `slide_index` | integer | No* | 1-based index of the target slide |
| `slide_id` | string | No* | Object ID of the target slide |
| `rows` | integer | Yes | Number of rows (minimum 1) |
| `columns` | integer | Yes | Number of columns (minimum 1) |
| `position` | object | No | Position in points (default: 0, 0) |
| `position.x` | number | No | X coordinate in points from left edge |
| `position.y` | number | No | Y coordinate in points from top edge |
| `size` | object | No | Size in points (optional) |
| `size.width` | number | No | Width in points (must be positive) |
| `size.height` | number | No | Height in points (must be positive) |

*Either `slide_index` or `slide_id` must be provided.

**Output:**
```json
{
  "object_id": "table_1234567890123456",
  "rows": 3,
  "columns": 4
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | Unique identifier of the created table |
| `rows` | integer | Number of rows in the table |
| `columns` | integer | Number of columns in the table |

**Features:**
- Creates an empty table with specified dimensions
- Uses either 1-based slide index or slide ID for flexibility
- Position defaults to (0, 0) if not specified
- Size is optional - if not provided, table uses default sizing based on rows/columns
- Standard slide dimensions: 720x405 points
- 1 point = 12700 EMU (English Metric Units)

**Use Cases:**
- Creating data tables in presentations
- Building comparison matrices
- Adding structured content layouts
- Creating schedules or timelines
- Building organization charts

**Examples:**

Create a simple 3x4 table:
```json
{
  "presentation_id": "abc123",
  "slide_index": 1,
  "rows": 3,
  "columns": 4
}
```

Create a table with position and size:
```json
{
  "presentation_id": "abc123",
  "slide_id": "g123456",
  "rows": 5,
  "columns": 3,
  "position": {"x": 50, "y": 100},
  "size": {"width": 600, "height": 300}
}
```

Create a single-cell table:
```json
{
  "presentation_id": "abc123",
  "slide_index": 1,
  "rows": 1,
  "columns": 1
}
```

**Errors:**
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `invalid slide reference: either slide_index or slide_id is required` - Neither slide reference provided
- `rows must be at least 1` - Invalid row count
- `columns must be at least 1` - Invalid column count
- `invalid size: size is required with positive width and height` - Invalid size (if provided)
- `slide not found` - Slide index out of range or slide ID not found
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify
- `failed to create table` - API error during table creation

---

#### `modify_table_structure`

Add or remove rows/columns from an existing table.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "table_xyz123",
  "action": "add_row",
  "index": 1,
  "count": 1,
  "insert_after": true
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `object_id` | string | Yes | Object ID of the table to modify |
| `action` | string | Yes | Action to perform: `add_row`, `delete_row`, `add_column`, `delete_column` |
| `index` | integer | Yes | 0-based index where to add/which to delete |
| `count` | integer | No | Number of rows/columns to add/delete (default: 1) |
| `insert_after` | boolean | No | For add actions: insert after index (default: true) |

**Output:**
```json
{
  "object_id": "table_xyz123",
  "action": "add_row",
  "index": 1,
  "count": 1,
  "new_rows": 4,
  "new_columns": 3
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | The modified table's object ID |
| `action` | string | The action performed (normalized lowercase) |
| `index` | integer | The index used for the operation |
| `count` | integer | Number of rows/columns added/deleted |
| `new_rows` | integer | Updated row count after modification |
| `new_columns` | integer | Updated column count after modification |

**Actions:**
| Action | Description |
|--------|-------------|
| `add_row` | Adds row(s) at the specified index |
| `delete_row` | Deletes row(s) starting at the specified index |
| `add_column` | Adds column(s) at the specified index |
| `delete_column` | Deletes column(s) starting at the specified index |

**Features:**
- Action names are case-insensitive (add_row, ADD_ROW both work)
- Index is 0-based (first row/column is index 0)
- For add actions, `insert_after` controls position:
  - `true` (default): Insert after the index (below for rows, right for columns)
  - `false`: Insert before the index (above for rows, left for columns)
- For delete actions, deletes starting from index and continuing for count items
- Validates that table has at least 1 row and 1 column after deletion

**Use Cases:**
- Expanding tables with new data rows
- Adding header rows to existing tables
- Removing empty rows/columns
- Restructuring table layouts
- Dynamic table resizing based on content

**Examples:**

Add a single row after row index 1:
```json
{
  "presentation_id": "abc123",
  "object_id": "table_xyz",
  "action": "add_row",
  "index": 1
}
```

Add 3 columns to the right of column index 0:
```json
{
  "presentation_id": "abc123",
  "object_id": "table_xyz",
  "action": "add_column",
  "index": 0,
  "count": 3
}
```

Add row above index 0 (at the beginning):
```json
{
  "presentation_id": "abc123",
  "object_id": "table_xyz",
  "action": "add_row",
  "index": 0,
  "insert_after": false
}
```

Delete 2 rows starting at index 1:
```json
{
  "presentation_id": "abc123",
  "object_id": "table_xyz",
  "action": "delete_row",
  "index": 1,
  "count": 2
}
```

**Errors:**
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `invalid object ID: object_id is required` - Empty object ID
- `invalid table action: action must be 'add_row', 'delete_row', 'add_column', or 'delete_column'` - Invalid action
- `invalid table index: index must be non-negative` - Negative index
- `count must be at least 1` - Invalid count
- `invalid table index: row/column index X is out of range` - Index out of bounds
- `cannot delete all rows/columns` - Would leave table with 0 rows or columns
- `object is not a table` - Object ID doesn't refer to a table
- `object not found` - Table object ID not found
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify
- `failed to modify table structure` - API error during modification

---

#### `merge_cells`

Merge or unmerge cells in a table.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "table_xyz123",
  "action": "merge",
  "start_row": 0,
  "start_column": 0,
  "end_row": 2,
  "end_column": 2
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `object_id` | string | Yes | Object ID of the table |
| `action` | string | Yes | Action to perform: `merge` or `unmerge` |
| `start_row` | integer | For merge | 0-based starting row index |
| `start_column` | integer | For merge | 0-based starting column index |
| `end_row` | integer | For merge | 0-based ending row index (exclusive) |
| `end_column` | integer | For merge | 0-based ending column index (exclusive) |
| `row` | integer | For unmerge | 0-based row index of merged cell |
| `column` | integer | For unmerge | 0-based column index of merged cell |

**Output:**
```json
{
  "object_id": "table_xyz123",
  "action": "merge",
  "range": "rows 0-1, columns 0-1"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | The table's object ID |
| `action` | string | The action performed (normalized lowercase) |
| `range` | string | Human-readable description of the affected range |

**Actions:**
| Action | Description |
|--------|-------------|
| `merge` | Merges cells in the specified rectangular range |
| `unmerge` | Unmerges a previously merged cell at the specified position |

**Features:**
- Action names are case-insensitive (merge, MERGE both work)
- For merge: uses 0-based indices with exclusive end (like Python slicing)
- For unmerge: specifies any cell position within a merged cell
- Validates range is within table bounds
- Validates merge range spans at least 2 cells

**Use Cases:**
- Creating table headers that span multiple columns
- Merging cells for category labels
- Creating complex table layouts with merged regions
- Unmerging previously merged cells for restructuring

**Examples:**

Merge a 2x2 range of cells:
```json
{
  "presentation_id": "abc123",
  "object_id": "table_xyz",
  "action": "merge",
  "start_row": 0,
  "start_column": 0,
  "end_row": 2,
  "end_column": 2
}
```

Merge an entire row (row 0, all 4 columns):
```json
{
  "presentation_id": "abc123",
  "object_id": "table_xyz",
  "action": "merge",
  "start_row": 0,
  "start_column": 0,
  "end_row": 1,
  "end_column": 4
}
```

Merge an entire column (column 0, all 3 rows):
```json
{
  "presentation_id": "abc123",
  "object_id": "table_xyz",
  "action": "merge",
  "start_row": 0,
  "start_column": 0,
  "end_row": 3,
  "end_column": 1
}
```

Unmerge a merged cell at position (1, 2):
```json
{
  "presentation_id": "abc123",
  "object_id": "table_xyz",
  "action": "unmerge",
  "row": 1,
  "column": 2
}
```

**Errors:**
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `invalid object ID: object_id is required` - Empty object ID
- `invalid merge action: action must be 'merge' or 'unmerge'` - Invalid action
- `invalid merge range: start indices must be non-negative` - Negative start indices
- `invalid merge range: end_row must be greater than start_row` - Invalid row range
- `invalid merge range: end_column must be greater than start_column` - Invalid column range
- `invalid merge range: end_row X exceeds table row count Y` - Row out of bounds
- `invalid merge range: end_column X exceeds table column count Y` - Column out of bounds
- `invalid merge range: merge range must span at least 2 cells` - Single cell range
- `invalid merge range: row and column indices must be non-negative` - Negative unmerge indices
- `invalid merge range: row X is out of range` - Unmerge row out of bounds
- `invalid merge range: column X is out of range` - Unmerge column out of bounds
- `object is not a table` - Object ID doesn't refer to a table
- `object not found` - Table object ID not found
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify
- `failed to merge cells` - API error during merge
- `failed to unmerge cells` - API error during unmerge

---

#### `modify_table_cell`

Modify the content and styling of a table cell.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "table_xyz123",
  "row": 0,
  "column": 1,
  "text": "Hello World",
  "style": {
    "font_family": "Arial",
    "font_size": 18,
    "bold": true,
    "foreground_color": "#FF0000"
  },
  "alignment": {
    "horizontal": "CENTER",
    "vertical": "MIDDLE"
  }
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `object_id` | string | Yes | Object ID of the table |
| `row` | integer | Yes | 0-based row index |
| `column` | integer | Yes | 0-based column index |
| `text` | string | No | Cell text content (replaces existing) |
| `style` | object | No | Text styling options |
| `style.font_family` | string | No | Font family name (e.g., "Arial") |
| `style.font_size` | integer | No | Font size in points |
| `style.bold` | boolean | No | Bold text |
| `style.italic` | boolean | No | Italic text |
| `style.underline` | boolean | No | Underline text |
| `style.strikethrough` | boolean | No | Strikethrough text |
| `style.foreground_color` | string | No | Text color (hex, e.g., "#FF0000") |
| `style.background_color` | string | No | Highlight color (hex) |
| `alignment` | object | No | Cell alignment options |
| `alignment.horizontal` | string | No | Horizontal: `START`, `CENTER`, `END`, `JUSTIFIED` |
| `alignment.vertical` | string | No | Vertical: `TOP`, `MIDDLE`, `BOTTOM` |

**Note:** At least one of `text`, `style`, or `alignment` must be provided.

**Output:**
```json
{
  "object_id": "table_xyz123",
  "row": 0,
  "column": 1,
  "modified_properties": ["text", "font_family=Arial", "bold=true", "horizontal_alignment=CENTER"]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | The table's object ID |
| `row` | integer | The row index |
| `column` | integer | The column index |
| `modified_properties` | array | List of properties that were modified |

**Features:**
- Set cell text content (replaces existing text)
- Apply text styling (font, size, bold, italic, color, etc.)
- Set horizontal alignment (paragraph-level)
- Set vertical alignment (cell-level)
- Alignment values are case-insensitive (center, CENTER both work)
- Invalid colors are silently ignored (other styles still apply)

**Use Cases:**
- Populating table cells with data
- Formatting header cells (bold, centered)
- Applying consistent styling to data cells
- Creating visually distinct categories with colors
- Adjusting cell content alignment for readability

**Examples:**

Set cell text content:
```json
{
  "presentation_id": "abc123",
  "object_id": "table_xyz",
  "row": 0,
  "column": 1,
  "text": "Hello World"
}
```

Apply text styling:
```json
{
  "presentation_id": "abc123",
  "object_id": "table_xyz",
  "row": 1,
  "column": 0,
  "style": {
    "font_family": "Arial",
    "font_size": 18,
    "bold": true,
    "foreground_color": "#FF0000"
  }
}
```

Set horizontal alignment:
```json
{
  "presentation_id": "abc123",
  "object_id": "table_xyz",
  "row": 0,
  "column": 0,
  "alignment": {
    "horizontal": "CENTER"
  }
}
```

Set vertical alignment:
```json
{
  "presentation_id": "abc123",
  "object_id": "table_xyz",
  "row": 2,
  "column": 1,
  "alignment": {
    "vertical": "MIDDLE"
  }
}
```

Combine text, styling, and alignment:
```json
{
  "presentation_id": "abc123",
  "object_id": "table_xyz",
  "row": 1,
  "column": 2,
  "text": "Styled Text",
  "style": {
    "bold": true,
    "font_size": 24
  },
  "alignment": {
    "horizontal": "END",
    "vertical": "BOTTOM"
  }
}
```

**Errors:**
- `invalid presentation ID: presentation_id is required` - Empty presentation ID
- `invalid object ID: object_id is required` - Empty object ID
- `invalid cell index: row must be non-negative` - Negative row index
- `invalid cell index: column must be non-negative` - Negative column index
- `no modification specified: text, style, or alignment must be provided` - No modifications
- `invalid horizontal alignment: must be START, CENTER, END, or JUSTIFIED` - Invalid horizontal alignment
- `invalid vertical alignment: must be TOP, MIDDLE, or BOTTOM` - Invalid vertical alignment
- `invalid cell index: row X is out of range (table has Y rows)` - Row out of bounds
- `invalid cell index: column X is out of range (table has Y columns)` - Column out of bounds
- `object is not a table` - Object ID doesn't refer to a table
- `object not found` - Table object ID not found
- `presentation not found` - Presentation doesn't exist
- `access denied to presentation` - No permission to modify
- `failed to modify table cell` - API error during modification

---

#### `create_line`

Create a line or arrow on a slide.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "slide_index": 1,
  "start_point": {"x": 10, "y": 10},
  "end_point": {"x": 100, "y": 100},
  "line_type": "STRAIGHT",
  "start_arrow": "NONE",
  "end_arrow": "ARROW",
  "line_color": "#FF0000",
  "line_weight": 2
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `slide_index` | integer | No* | 1-based index of the target slide |
| `slide_id` | string | No* | Object ID of the target slide |
| `start_point` | object | Yes | Start coordinates in points {x, y} |
| `end_point` | object | Yes | End coordinates in points {x, y} |
| `line_type` | string | No | Line type (STRAIGHT, CURVED, ELBOW) |
| `start_arrow` | string | No | Arrow style at start (NONE, ARROW, etc.) |
| `end_arrow` | string | No | Arrow style at end (NONE, ARROW, etc.) |
| `line_color` | string | No | Line color hex string |
| `line_weight` | number | No | Line weight in points |
| `line_dash` | string | No | Dash style (SOLID, DASH, DOT, etc.) |

*Either `slide_index` or `slide_id` must be provided.

**Output:**
```json
{
  "object_id": "line_1234567890"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | Unique identifier of the created line |

**Line Types:**
- `STRAIGHT` - Straight line connection
- `CURVED` - Simple curved line
- `ELBOW` - Angled connector line

**Arrow Styles:**
- `NONE` - No arrow head
- `ARROW` / `FILL_ARROW` - Filled triangle arrow
- `DIAMOND` / `FILL_DIAMOND` - Filled diamond
- `OVAL` / `CIRCLE` / `FILL_CIRCLE` - Filled circle
- `OPEN_ARROW` - Open arrow head
- `OPEN_CIRCLE` - Open circle
- `OPEN_DIAMOND` - Open diamond
- `STEALTH_ARROW` - Narrow filled arrow

**Features:**
- Handles start/end point geometry automatically
- Supports lines in any direction (including anti-diagonal)
- Applies styling and arrow heads in a single operation
- Maps user-friendly names to API constants

**Examples:**

Create a simple line:
```json
{
  "presentation_id": "abc123",
  "slide_index": 1,
  "start_point": {"x": 100, "y": 100},
  "end_point": {"x": 300, "y": 100}
}
```

Create an arrow:
```json
{
  "presentation_id": "abc123",
  "slide_index": 1,
  "start_point": {"x": 50, "y": 50},
  "end_point": {"x": 250, "y": 150},
  "end_arrow": "ARROW",
  "line_weight": 3,
  "line_color": "#0000FF"
}
```

Create a curved connector:
```json
{
  "presentation_id": "abc123",
  "slide_index": 1,
  "start_point": {"x": 100, "y": 100},
  "end_point": {"x": 200, "y": 200},
  "line_type": "CURVED",
  "start_arrow": "OVAL",
  "end_arrow": "ARROW",
  "line_dash": "DOT"
}
```

**Errors:**
- `start_point and end_point are required` - Missing coordinates
- `invalid slide reference` - Neither slide_index nor slide_id provided
- `slide not found` - Slide doesn't exist
- `presentation not found` - Presentation doesn't exist
- `access denied` - No permission to modify
- `failed to create line` - API error

---

#### `modify_shape`

Modify shape appearance (fill, outline, shadow).

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "shape-123",
  "properties": {
    "fill_color": "#FF0000",
    "outline_color": "#0000FF",
    "outline_weight": 3,
    "outline_dash": "DASH",
    "shadow": true
  }
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `object_id` | string | Yes | ID of the shape to modify |
| `properties` | object | Yes | Properties to update |
| `properties.fill_color` | string | No | Hex color or "transparent" |
| `properties.outline_color` | string | No | Hex color or "transparent" |
| `properties.outline_weight` | number | No | Outline weight in points |
| `properties.outline_dash` | string | No | Dash style (SOLID, DASH, DOT, etc.) |
| `properties.shadow` | boolean | No | Enable (true) or disable (false) shadow |

**Output:**
```json
{
  "object_id": "shape-123",
  "updated_properties": ["fill_color", "outline_color", "shadow"]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | The modified object's ID |
| `updated_properties` | array | List of property names that were updated |

**Features:**
- Modify fill and outline colors with support for transparency
- Change outline style (weight, dash)
- Toggle shadow visibility
- Updates are applied in a single batch request

**Examples:**

Change fill to green:
```json
{
  "presentation_id": "abc123",
  "object_id": "shape-xyz",
  "properties": {
    "fill_color": "#00FF00"
  }
}
```

Make transparent with dashed outline:
```json
{
  "presentation_id": "abc123",
  "object_id": "shape-xyz",
  "properties": {
    "fill_color": "transparent",
    "outline_color": "#000000",
    "outline_weight": 2,
    "outline_dash": "DASH"
  }
}
```

**Errors:**
- `no properties to update` - Properties object is empty or missing
- `object not found` - Object ID not found
- `access denied` - No permission to modify
- `failed to modify shape` - API error

---

#### `transform_object`

Move, resize, or rotate any object.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "shape-123",
  "position": {"x": 100, "y": 100},
  "size": {"width": 200, "height": 100},
  "rotation": 45,
  "scale_proportionally": true
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `object_id` | string | Yes | ID of the object to transform |
| `position` | object | No | New position in points {x, y} |
| `size` | object | No | New size in points {width, height} |
| `rotation` | number | No | Rotation angle in degrees (0-360) |
| `scale_proportionally` | boolean | No | Whether to scale proportionally when resizing (default: true) |

**Output:**
```json
{
  "position": {"x": 100, "y": 100},
  "size": {"width": 200, "height": 100},
  "rotation": 45
}
```

| Field | Type | Description |
|-------|------|-------------|
| `position` | object | Final position in points |
| `size` | object | Final size in points |
| `rotation` | number | Final rotation in degrees |

**Features:**
- Move objects to absolute coordinates
- Resize objects with optional proportional scaling
- Rotate objects to specific angles
- Handles complex transform math automatically

**Examples:**

Move an object:
```json
{
  "presentation_id": "abc123",
  "object_id": "shape-xyz",
  "position": {"x": 50, "y": 50}
}
```

Resize (double width):
```json
{
  "presentation_id": "abc123",
  "object_id": "shape-xyz",
  "size": {"width": 400}
}
```

Rotate 45 degrees:
```json
{
  "presentation_id": "abc123",
  "object_id": "shape-xyz",
  "rotation": 45
}
```

**Errors:**
- `object not found` - Object ID not found
- `cannot resize object with unknown base size` - Cannot determine original size for scaling
- `access denied` - No permission to modify
- `failed to transform object` - API error

---

#### `change_z_order`

Change the z-order (layering) of an object on a slide.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "shape-123",
  "action": "bring_to_front"
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `object_id` | string | Yes | ID of the object to reorder |
| `action` | string | Yes | Z-order action (see Actions table) |

**Actions:**

| Action | Description |
|--------|-------------|
| `bring_to_front` | Moves object to the top of the stack (front) |
| `send_to_back` | Moves object to the bottom of the stack (back) |
| `bring_forward` | Moves object up one layer |
| `send_backward` | Moves object down one layer |

**Output:**
```json
{
  "object_id": "shape-123",
  "action": "bring_to_front",
  "new_z_order": 2,
  "total_layers": 3
}
```

| Field | Type | Description |
|-------|------|-------------|
| `object_id` | string | The modified object's ID |
| `action` | string | The action performed (lowercase) |
| `new_z_order` | number | New 0-based position (0 = furthest back) |
| `total_layers` | number | Total number of objects on the slide |

**Features:**
- Action names are case-insensitive
- Returns position information after change
- Grouped objects cannot have z-order changed (API limitation)

**Examples:**

Bring to front:
```json
{
  "presentation_id": "abc123",
  "object_id": "shape-xyz",
  "action": "bring_to_front"
}
```

Move backward one layer:
```json
{
  "presentation_id": "abc123",
  "object_id": "shape-xyz",
  "action": "send_backward"
}
```

**Errors:**
- `invalid z-order action` - Invalid action specified
- `cannot change z-order of grouped objects` - Object is inside a group
- `object not found` - Object ID not found
- `access denied` - No permission to modify
- `failed to change z-order` - API error

---

#### `group_objects`

Group or ungroup objects on a slide.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "action": "group",
  "object_ids": ["shape-1", "shape-2", "shape-3"]
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `action` | string | Yes | Action to perform: `group` or `ungroup` |
| `object_ids` | array | For group | Array of object IDs to group (minimum 2) |
| `object_id` | string | For ungroup | ID of the group to ungroup |

**Actions:**

| Action | Description |
|--------|-------------|
| `group` | Groups multiple objects together into a single group |
| `ungroup` | Separates a group into individual objects |

**Output (group action):**
```json
{
  "action": "group",
  "group_id": "group_1234567890"
}
```

**Output (ungroup action):**
```json
{
  "action": "ungroup",
  "object_ids": ["shape-1", "shape-2", "shape-3"]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `action` | string | The action performed |
| `group_id` | string | Created group's object ID (for group action) |
| `object_ids` | array | IDs of ungrouped objects (for ungroup action) |

**Features:**
- Action names are case-insensitive
- Requires at least 2 objects for grouping
- All objects must be on the same slide
- Validates that objects can be grouped

**Objects That Cannot Be Grouped:**
- Tables
- Videos
- Placeholder shapes (title, body, etc.)
- Objects already inside another group

**Examples:**

Group objects:
```json
{
  "presentation_id": "abc123",
  "action": "group",
  "object_ids": ["shape-1", "shape-2"]
}
```

Ungroup a group:
```json
{
  "presentation_id": "abc123",
  "action": "ungroup",
  "object_id": "group_1234567890"
}
```

**Errors:**
- `invalid group action` - Action must be 'group' or 'ungroup'
- `at least two objects are required to group` - Need minimum 2 objects
- `all objects must be on the same page` - Objects on different slides
- `object is not a group` - Trying to ungroup a non-group object
- `object cannot be grouped` - Object type not supported (table, video, placeholder)
- `object not found` - Object ID not found
- `access denied` - No permission to modify
- `failed to group/ungroup objects` - API error

---

#### `delete_object`

Delete one or more objects from a presentation.

**Input:**
```json
{
  "presentation_id": "abc123xyz",
  "object_id": "shape-xyz"
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation_id` | string | Yes | The Google Slides presentation ID |
| `object_id` | string | No* | Single object ID to delete |
| `multiple` | array | No* | Array of object IDs for batch delete |

*At least one of `object_id` or `multiple` is required. Both can be provided together.

**Output:**
```json
{
  "deleted_count": 3,
  "deleted_ids": ["shape-1", "shape-2", "shape-3"],
  "not_found_ids": ["shape-4"]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `deleted_count` | number | Number of objects deleted |
| `deleted_ids` | array | List of deleted object IDs |
| `not_found_ids` | array | Object IDs that were not found (optional) |

**Features:**
- Delete single object or multiple objects in batch
- Both `object_id` and `multiple` can be combined
- Automatically deduplicates object IDs
- Partial success: deletes found objects, reports not found separately
- Finds objects anywhere (slides, masters, layouts, nested in groups)

**Examples:**

Delete single object:
```json
{
  "presentation_id": "abc123",
  "object_id": "shape-xyz"
}
```

Delete multiple objects:
```json
{
  "presentation_id": "abc123",
  "multiple": ["shape-1", "shape-2", "image-1"]
}
```

Delete with both (all unique IDs):
```json
{
  "presentation_id": "abc123",
  "object_id": "shape-1",
  "multiple": ["shape-2", "shape-3"]
}
```

**Errors:**
- `no objects specified for deletion` - Neither object_id nor multiple provided
- `none of the specified objects were found` - All objects not found
- `presentation not found` - Presentation doesn't exist
- `access denied` - No permission to modify
- `failed to delete object` - API error

---
- `add_video` - Embed videos
- `create_table` - Insert tables
- `modify_table_structure` - Add/remove rows and columns from tables
- `merge_cells` - Merge or unmerge table cells
- `modify_table_cell` - Modify table cell content and styling

### Styling and Themes
- `apply_theme` - Apply presentation themes
- `set_background` - Configure backgrounds
- `set_transition` - Add slide transitions
- `add_animation` - Create object animations

## Configuration

Environment variables:
- `PORT` - HTTP server port (default: 8080)
- `GCP_PROJECT_ID` - Google Cloud project ID
- `OAUTH_CLIENT_ID` - OAuth2 client ID (from Secret Manager)
- `OAUTH_CLIENT_SECRET` - OAuth2 client secret (from Secret Manager)

## License

MIT License
