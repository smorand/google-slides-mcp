# Deployment Guide

## Overview

The project deploys to Google Cloud Run using Terraform for infrastructure management.

---

## Terraform Structure

```
terraform/
├── config.yaml      # Single source of configuration
├── provider.tf      # Google provider and backend
├── local.tf         # Loads config.yaml, defines derived values
├── apis.tf          # Enables required Google APIs
├── iam.tf           # Service accounts for Cloud Run and Cloud Build
├── cloudrun.tf      # MCP server deployment
├── firestore.tf     # Database for API keys and tokens
└── secrets.tf       # OAuth2 credentials storage
```

---

## Configuration

Edit `terraform/config.yaml` to customize:

```yaml
gcp:
  project_id: "your-project-id"
  location: "europe-west1"
  resources:
    cloud_run:
      cpu: "1"
      memory: "512Mi"
      min_instances: 0
      max_instances: 10
      concurrency: 80

parameters:
  log_level: "info"
  cors_origins: "*"
```

### Key Configuration Options

| Setting | Description | Default |
|---------|-------------|---------|
| `gcp.project_id` | Your GCP project ID | Required |
| `gcp.location` | GCP region | europe-west1 |
| `cloud_run.cpu` | CPU allocation | 1 |
| `cloud_run.memory` | Memory allocation | 512Mi |
| `cloud_run.min_instances` | Minimum instances | 0 |
| `cloud_run.max_instances` | Maximum instances | 10 |
| `cloud_run.concurrency` | Requests per instance | 80 |

---

## Deployment Commands

### Using Makefile

```bash
make plan     # Preview infrastructure changes
make deploy   # Apply changes
make undeploy # Destroy resources
```

### Using Terraform Directly

```bash
cd terraform
terraform init
terraform plan
terraform apply
```

---

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

### Local Docker Run

```bash
docker build -t google-slides-mcp .
docker run -p 8080:8080 google-slides-mcp
```

---

## Cloud Build

`cloudbuild.yaml` defines the CI/CD pipeline:

1. **test**: Run `go test -race` with coverage
2. **build**: Build Docker image with version tags
3. **push**: Push to Artifact Registry
4. **deploy**: Deploy to Cloud Run

### Substitutions

- `_REGION`: GCP region (default: europe-west1)
- `_SERVICE_NAME`: Cloud Run service name (default: google-slides-mcp)

### Manual Trigger

```bash
gcloud builds submit --config=cloudbuild.yaml
```

---

## Required GCP APIs

The following APIs are enabled automatically via `apis.tf`:

- Cloud Run API
- Cloud Build API
- Artifact Registry API
- Firestore API
- Secret Manager API
- Slides API
- Drive API
- Cloud Translation API

---

## IAM Roles

### Cloud Run Service Account

The service account for Cloud Run has:
- `roles/datastore.user` - Firestore access for API keys
- `roles/secretmanager.secretAccessor` - Access OAuth credentials
- `roles/cloudtranslate.user` - Translation API access

### Cloud Build Service Account

- `roles/run.admin` - Deploy to Cloud Run
- `roles/iam.serviceAccountUser` - Act as service account
- `roles/artifactregistry.writer` - Push container images

---

## Secrets Management

OAuth2 credentials are stored in Secret Manager:

- `oauth-client-id` - Google OAuth2 client ID
- `oauth-client-secret` - Google OAuth2 client secret

### Setting Up Secrets

```bash
# Create secrets (first time)
echo -n "your-client-id" | gcloud secrets create oauth-client-id --data-file=-
echo -n "your-client-secret" | gcloud secrets create oauth-client-secret --data-file=-

# Update secrets
echo -n "new-client-id" | gcloud secrets versions add oauth-client-id --data-file=-
```

---

## Firestore Setup

The Firestore database stores:
- API keys
- Refresh tokens
- User email associations
- Usage timestamps

Collection structure:
```
api_keys/
  {api_key}/
    api_key: string
    refresh_token: string
    user_email: string
    created_at: timestamp
    last_used: timestamp
```

---

## Environment Variables

Cloud Run environment variables (set via Terraform):

| Variable | Description |
|----------|-------------|
| `GCP_PROJECT_ID` | GCP project ID |
| `FIRESTORE_COLLECTION` | API keys collection name |
| `LOG_LEVEL` | Logging level (debug, info, warn, error) |
| `CORS_ORIGINS` | Allowed CORS origins |

---

## Health Checks

Cloud Run performs health checks on:
- **Startup probe**: `GET /health`
- **Liveness probe**: `GET /health`

Response: `{"status": "healthy"}`

---

## Scaling Configuration

### Cold Start Optimization

- Set `min_instances: 1` for production to avoid cold starts
- Use `cpu_idle: false` for consistent performance

### Concurrency Tuning

- Default concurrency: 80 requests per instance
- Adjust based on memory usage and response times
- Monitor with Cloud Run metrics

---

## Monitoring

### Recommended Alerts

1. **Error rate** > 1% over 5 minutes
2. **Latency** p95 > 5 seconds
3. **Instance count** approaching max
4. **Memory usage** > 80%

### Logging

Structured logs are sent to Cloud Logging:
- Request/response logging
- Error traces
- API call metrics

---

## Rollback

### Using gcloud

```bash
# List revisions
gcloud run revisions list --service=google-slides-mcp

# Route traffic to previous revision
gcloud run services update-traffic google-slides-mcp \
  --to-revisions=google-slides-mcp-00001-abc=100
```

### Using Terraform

```bash
# Revert to previous state
git checkout HEAD~1 terraform/
terraform apply
```

---

## Cost Optimization

1. **Set min_instances: 0** for development/staging
2. **Use CPU throttling** when idle (`cpu_idle: true`)
3. **Right-size memory** based on actual usage
4. **Enable request-based billing** (default)

---

## Troubleshooting Deployment

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| "Permission denied" | Missing IAM roles | Check service account permissions |
| "Container failed to start" | Missing env vars | Verify all required secrets exist |
| "Health check failed" | App crash on startup | Check Cloud Run logs |
| "Quota exceeded" | API quota limits | Request quota increase |

### Viewing Logs

```bash
# Stream logs
gcloud run services logs tail google-slides-mcp

# View recent logs
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=google-slides-mcp" --limit=100
```
