# Artifact Registry repository for Docker images

resource "google_artifact_registry_repository" "docker" {
  project       = local.project_id
  location      = local.location
  repository_id = "${local.prefix}-${local.project_name}-${local.env}"
  description   = "Docker repository for ${local.project_name}"
  format        = "DOCKER"

  labels = local.labels

  cleanup_policies {
    id     = "keep-minimum-versions"
    action = "KEEP"
    most_recent_versions {
      keep_count = 5
    }
  }

  cleanup_policies {
    id     = "delete-old-images"
    action = "DELETE"
    condition {
      older_than = "2592000s" # 30 days
    }
  }
}

# ============================================
# DOCKER IMAGE BUILD AND PUSH
# ============================================
# Uses kreuzwerker/docker provider for proper state management
# Prerequisites:
#   1. Docker daemon running: docker info
#   2. Registry auth: gcloud auth configure-docker europe-west1-docker.pkg.dev

# Build image locally
resource "docker_image" "mcp" {
  name = local.docker_image

  build {
    context    = "${path.root}/.."
    dockerfile = "Dockerfile"

    label = {
      "org.opencontainers.image.source" = "https://github.com/smorand/google-slides-mcp"
      "environment"                      = local.env
      "managed_by"                       = "terraform"
    }
  }

  # Triggers rebuild when source files change
  triggers = {
    dockerfile_hash = filesha256("${path.root}/../Dockerfile")
    go_mod_hash     = filesha256("${path.root}/../go.mod")
    go_sum_hash     = filesha256("${path.root}/../go.sum")
    main_hash       = filesha256("${path.root}/../cmd/google-slides-mcp/main.go")
  }
}

# Push to Artifact Registry
resource "docker_registry_image" "mcp" {
  name = docker_image.mcp.name

  # Keep old images during updates
  keep_remotely = true

  # Trigger push when local image changes
  triggers = {
    image_id = docker_image.mcp.image_id
  }

  depends_on = [google_artifact_registry_repository.docker]
}

# ============================================
# OUTPUTS
# ============================================

output "docker_repository_url" {
  description = "Artifact Registry repository URL"
  value       = "${local.location}-docker.pkg.dev/${local.project_id}/${google_artifact_registry_repository.docker.repository_id}"
}

output "docker_image" {
  description = "Full Docker image URL"
  value       = docker_registry_image.mcp.name
}

output "docker_image_digest" {
  description = "Docker image SHA256 digest"
  value       = docker_registry_image.mcp.sha256_digest
}
