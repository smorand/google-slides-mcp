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

### Deployment

The server is designed to be deployed on Google Cloud Run. See the `terraform/` directory for infrastructure configuration.

```bash
# Deploy infrastructure
cd terraform
terraform init
terraform plan
terraform apply
```

## Authentication Flow

1. User visits `/auth` endpoint
2. Server redirects to Google OAuth2 consent screen
3. User grants permissions (Slides, Drive, Translate APIs)
4. OAuth2 callback exchanges code for tokens
5. Server generates and returns an API key
6. User includes API key in subsequent requests

## Available MCP Tools

The server provides comprehensive tools for Google Slides manipulation:

### Presentation Management
- `get_presentation` - Load presentation content
- `search_presentations` - Search for presentations in Drive
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
