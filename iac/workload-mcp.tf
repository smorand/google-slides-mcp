# Main Infrastructure - MCP Server Workload
# Cloud Run, Secret Manager, Firestore

# ============================================
# SECRET MANAGER
# ============================================

resource "google_secret_manager_secret" "oauth_credentials" {
  secret_id = local.oauth_secret_name
  project   = local.project_id

  replication {
    auto {}
  }

  labels = merge(local.labels, {
    purpose = "oauth-credentials"
  })
}

# Note: Secret version is created MANUALLY via gcloud to avoid
# storing sensitive credentials in terraform state:
#
# gcloud secrets versions add smo-gslides-oauth-creds \
#   --data-file=$HOME/.credentials/smo-gslides-oauth.json \
#   --project=project-3335b451-2ffb-4ece-8cd

# ============================================
# FIRESTORE DATABASE
# ============================================

resource "google_firestore_database" "api_keys" {
  provider    = google-beta
  project     = local.project_id
  name        = "(default)"
  location_id = "eur3"  # Multi-region Europe
  type        = "FIRESTORE_NATIVE"

  # Prevent accidental deletion
  deletion_policy = "DELETE"
}

# ============================================
# CLOUD RUN SERVICE
# ============================================

resource "google_cloud_run_v2_service" "mcp" {
  name     = local.mcp_name
  location = local.location
  project  = local.project_id
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    service_account = local.cloudrun_sa_email

    scaling {
      min_instance_count = local.mcp_min_instances
      max_instance_count = local.mcp_max_instances
    }

    timeout = "${local.mcp_timeout}s"

    containers {
      image = local.docker_image

      resources {
        limits = {
          cpu    = local.mcp_cpu
          memory = local.mcp_memory
        }
        cpu_idle          = true
        startup_cpu_boost = true
      }

      # Environment variables
      env {
        name  = "GOOGLE_CLOUD_PROJECT"
        value = local.project_id
      }

      env {
        name  = "OAUTH_SECRET_NAME"
        value = local.oauth_secret_name
      }

      env {
        name  = "CACHE_TTL"
        value = local.cache_ttl
      }

      env {
        name  = "MAX_RETRIES"
        value = local.max_retries
      }

      env {
        name  = "LOG_LEVEL"
        value = local.log_level
      }

      env {
        name  = "RATE_LIMIT_RPS"
        value = local.rate_limit_rps
      }

      # Port configuration
      ports {
        container_port = 8080
        name           = "http1"
      }

      # Startup probe
      startup_probe {
        http_get {
          path = "/health"
          port = 8080
        }
        initial_delay_seconds = 5
        timeout_seconds       = 3
        period_seconds        = 10
        failure_threshold     = 3
      }

      # Liveness probe
      liveness_probe {
        http_get {
          path = "/health"
          port = 8080
        }
        initial_delay_seconds = 10
        timeout_seconds       = 3
        period_seconds        = 30
        failure_threshold     = 3
      }
    }

    # Execution environment
    execution_environment = "EXECUTION_ENVIRONMENT_GEN2"

    # Container concurrency
    max_instance_request_concurrency = local.mcp_concurrency
  }

  traffic {
    type    = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
    percent = 100
  }

  labels = local.labels

  depends_on = [
    google_artifact_registry_repository.docker,
    google_secret_manager_secret.oauth_credentials,
    docker_registry_image.mcp,
  ]
}

# ============================================
# IAM - PUBLIC ACCESS
# ============================================

# Allow unauthenticated access (authentication handled by app via API keys)
resource "google_cloud_run_v2_service_iam_member" "public_access" {
  project  = local.project_id
  location = local.location
  name     = google_cloud_run_v2_service.mcp.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ============================================
# OUTPUTS
# ============================================

output "cloud_run_url" {
  value       = google_cloud_run_v2_service.mcp.uri
  description = "URL of the deployed Cloud Run service"
}

output "oauth_secret_name" {
  value       = google_secret_manager_secret.oauth_credentials.secret_id
  description = "Name of the Secret Manager secret for OAuth credentials"
}

output "service_account_email" {
  value       = local.cloudrun_sa_email
  description = "Email of the Cloud Run service account"
}
