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
