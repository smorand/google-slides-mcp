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

*More tools to be documented:*
- `copy_presentation` - Copy/duplicate presentations
- `create_presentation` - Create new presentations
- `export_pdf` - Export to PDF format

### Slide Operations
- `list_slides` - List all slides
- `describe_slide` - Get detailed slide description
- `add_slide` - Add new slides
- `delete_slide` - Remove slides
- `reorder_slides` - Change slide order
- `duplicate_slide` - Clone slides

### Content Manipulation
- `add_text_box` - Add text elements
- `modify_text` - Edit text content
- `search_text` - Find text across presentation
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
