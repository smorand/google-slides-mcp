# Deployment Guide

## Overview

The project deploys to Google Cloud Run using a **two-phase Terraform deployment** pattern for infrastructure management.

---

## Terraform Structure

```
google-slides-mcp/
├── config.yaml              # Single source of truth (project root)
├── Makefile                 # 7 terraform targets
├── init/                    # Phase 1: Bootstrap (one-time per GCP project)
│   ├── provider.tf         # Local backend
│   ├── local.tf            # Loads ../config.yaml
│   ├── state-backend.tf    # GCS bucket for terraform state
│   ├── services-apis.tf    # Enable required GCP APIs
│   └── services-accounts.tf # Service accounts + IAM
└── iac/                     # Phase 2: Main infrastructure
    ├── provider.tf.template # Template with bucket placeholder
    ├── provider.tf          # Generated after init-deploy
    ├── local.tf             # Loads ../config.yaml
    └── workload-mcp.tf      # Cloud Run + Secrets + Firestore
```

### Two-Phase Deployment

| Phase | Directory | Purpose | Frequency |
|-------|-----------|---------|-----------|
| 1 - Bootstrap | `init/` | State bucket, service accounts, APIs | Once per GCP project |
| 2 - Infrastructure | `iac/` | Cloud Run, secrets, Firestore | Each deployment |

---

## Configuration

Edit `config.yaml` at project root:

```yaml
# Global
prefix: smogslides              # Resource naming prefix
project_name: gslides-mcp       # Short project identifier
env: prd                        # Environment: dev, stg, prd

# GCP
project_id: project-xxx         # GCP project ID
location: europe-west1          # Primary region

# Resources
resources:
  cpu: "1"
  memory: 512Mi
  min_instances: 0
  max_instances: 5
  timeout_seconds: 300
  concurrency: 80

# Application
parameters:
  oauth_secret_name: smo-gslides-oauth-creds
  cache_ttl: "5m"
  log_level: info
```

### Naming Convention

Resources follow consistent naming patterns:

| Resource | Pattern | Example |
|----------|---------|---------|
| State bucket | `smo-tfstate-{location_id}-{env}` | `smo-tfstate-ew1-prd` |
| Service accounts | `{prefix}-{service}-{env}` | `smogslides-cloudrun-prd` |
| Cloud Run | `{prefix}-{project_name}-{env}` | `smogslides-gslides-mcp-prd` |
| OAuth secret | `smo-{project}-oauth-creds` | `smo-gslides-oauth-creds` |

---

## Deployment Commands

### First-Time Setup (Bootstrap)

```bash
# 1. Review bootstrap infrastructure
make init-plan

# 2. Deploy bootstrap (creates state bucket, service accounts)
make init-deploy
# This also generates iac/provider.tf with the correct bucket name

# 3. Review main infrastructure
make plan

# 4. Deploy main infrastructure
make deploy
```

### Subsequent Deployments

```bash
make plan    # Review changes
make deploy  # Apply changes
```

### Teardown

```bash
# Destroy main infrastructure only
make undeploy

# Destroy EVERYTHING including state bucket (DANGEROUS)
make init-destroy
```

---

## Docker

### Dockerfile Architecture

Multi-stage build for minimal image size and security:

1. **Builder stage** (`golang:1.21-alpine`):
   - Builds static binary with CGO_ENABLED=0
   - Supports build args: VERSION, COMMIT_SHA, BUILD_TIME

2. **Runtime stage** (`gcr.io/distroless/static-debian12:nonroot`):
   - Distroless image for minimal attack surface
   - Runs as non-root user (UID 65532)

### Build and Push

```bash
# Build locally
docker build -t google-slides-mcp .

# Tag and push to Artifact Registry
docker tag google-slides-mcp \
  europe-west1-docker.pkg.dev/project-xxx/gslides-mcp/gslides-mcp:latest

docker push \
  europe-west1-docker.pkg.dev/project-xxx/gslides-mcp/gslides-mcp:latest
```

---

## Secrets Management

OAuth2 credentials are stored in Secret Manager.

### Initial Setup

1. Create OAuth2 credentials in Google Cloud Console
2. Download the JSON file
3. Add to Secret Manager:

```bash
gcloud secrets versions add smo-gslides-oauth-creds \
  --data-file=$HOME/.credentials/smo-gslides-oauth.json \
  --project=project-3335b451-2ffb-4ece-8cd
```

**Note:** Secret versions are created manually (not via Terraform) to avoid storing sensitive data in terraform state.

---

## IAM Roles

### Cloud Run Service Account

Created in `init/services-accounts.tf`:
- `roles/secretmanager.secretAccessor` - Access OAuth credentials
- `roles/datastore.user` - Firestore access for API keys
- `roles/cloudtrace.agent` - Distributed tracing
- `roles/logging.logWriter` - Cloud Logging
- `roles/monitoring.metricWriter` - Cloud Monitoring

### Cloud Build Service Account

- `roles/artifactregistry.writer` - Push container images
- `roles/run.developer` - Deploy to Cloud Run
- `roles/iam.serviceAccountUser` - Act as Cloud Run service account
- `roles/cloudbuild.builds.builder` - Execute builds

---

## Environment Variables

Cloud Run environment variables (set via Terraform):

| Variable | Description |
|----------|-------------|
| `GOOGLE_CLOUD_PROJECT` | GCP project ID |
| `OAUTH_SECRET_NAME` | Secret Manager secret name |
| `CACHE_TTL` | Cache duration (e.g., "5m") |
| `MAX_RETRIES` | API retry count |
| `LOG_LEVEL` | Logging level |
| `RATE_LIMIT_RPS` | Rate limit per second |

---

## Health Checks

Cloud Run probes:
- **Startup probe**: `GET /health` (initial delay: 5s)
- **Liveness probe**: `GET /health` (period: 30s)

Response: `{"status": "healthy"}`

---

## Troubleshooting

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| "init/ not initialized" | Missing bootstrap | Run `make init-deploy` |
| "Permission denied" | Missing IAM roles | Check service account |
| "Container failed to start" | Missing secret | Add OAuth credentials |
| "Health check failed" | App crash | Check Cloud Run logs |

### Viewing Logs

```bash
# Stream logs
gcloud run services logs tail smogslides-gslides-mcp-prd

# View recent logs
gcloud logging read "resource.type=cloud_run_revision" --limit=100
```

### Rollback

```bash
# List revisions
gcloud run revisions list --service=smogslides-gslides-mcp-prd

# Route traffic to previous revision
gcloud run services update-traffic smogslides-gslides-mcp-prd \
  --to-revisions=smogslides-gslides-mcp-prd-00001-abc=100
```

---

## Cost Optimization

1. **Set min_instances: 0** for development (scales to zero)
2. **Enable CPU throttling** when idle (`cpu_idle: true`)
3. **Right-size memory** based on actual usage
4. **Use request-based billing** (default)
