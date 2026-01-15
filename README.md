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

*More tools to be documented:*

### Content Manipulation
- `add_text` - Add text to existing placeholders
- `replace_text` - Find and replace text
- `style_text` - Apply text formatting
- `format_paragraph` - Set paragraph styles

### Media and Objects
- `add_image` - Insert images
- `modify_image` - Edit image properties
- `create_shape` - Add shapes
- `create_line` - Draw lines and arrows
- `add_video` - Embed videos
- `create_table` - Insert tables

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
