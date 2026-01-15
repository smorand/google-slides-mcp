# Cloud Run Configuration
# MCP server deployment

# ============================================
# LOCAL VARIABLES
# ============================================

locals {
  service_name = "${local.resource_prefix}-mcp"
  image_name   = "gcr.io/${local.project_id}/${local.prefix}-mcp:latest"
}

# ============================================
# CLOUD RUN SERVICE
# ============================================

resource "google_cloud_run_v2_service" "mcp_server" {
  name     = local.service_name
  location = local.cloud_run_config.region
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    service_account = google_service_account.cloudrun.email

    scaling {
      min_instance_count = local.cloud_run_config.min_instances
      max_instance_count = local.cloud_run_config.max_instances
    }

    timeout = "${local.cloud_run_config.timeout_seconds}s"

    max_instance_request_concurrency = local.cloud_run_config.container_concurrency

    containers {
      image = local.image_name

      resources {
        limits = {
          cpu    = local.cloud_run_config.cpu
          memory = local.cloud_run_config.memory
        }
        cpu_idle = true
      }

      ports {
        container_port = 8080
      }

      # Environment variables from config
      env {
        name  = "PORT"
        value = "8080"
      }

      env {
        name  = "GCP_PROJECT_ID"
        value = local.project_id
      }

      env {
        name  = "LOG_LEVEL"
        value = local.app_params.log_level
      }

      env {
        name  = "RATE_LIMIT_RPS"
        value = tostring(local.app_params.rate_limit_rps)
      }

      env {
        name  = "CACHE_TTL_MINUTES"
        value = tostring(local.app_params.cache_ttl_minutes)
      }

      env {
        name  = "TOKEN_CACHE_TTL_MINUTES"
        value = tostring(local.app_params.token_cache_ttl_minutes)
      }

      # OAuth2 secrets from Secret Manager
      env {
        name = "OAUTH_CLIENT_ID"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.oauth_client_id.secret_id
            version = "latest"
          }
        }
      }

      env {
        name = "OAUTH_CLIENT_SECRET"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.oauth_client_secret.secret_id
            version = "latest"
          }
        }
      }

      env {
        name = "OAUTH_REDIRECT_URI"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.oauth_redirect_uri.secret_id
            version = "latest"
          }
        }
      }

      # Startup probe
      startup_probe {
        http_get {
          path = "/health"
          port = 8080
        }
        initial_delay_seconds = 0
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
        initial_delay_seconds = 0
        timeout_seconds       = 3
        period_seconds        = 30
      }
    }
  }

  labels = local.common_labels

  depends_on = [
    google_project_service.required_apis["run.googleapis.com"],
    google_secret_manager_secret.oauth_client_id,
    google_secret_manager_secret.oauth_client_secret,
    google_secret_manager_secret.oauth_redirect_uri,
  ]

  lifecycle {
    # Ignore changes to the image tag - managed by CI/CD
    ignore_changes = [
      template[0].containers[0].image
    ]
  }
}

# ============================================
# IAM - PUBLIC ACCESS
# ============================================

# Allow unauthenticated access to Cloud Run service
# OAuth2 flow needs to be accessible without authentication
resource "google_cloud_run_v2_service_iam_member" "public_access" {
  project  = local.project_id
  location = google_cloud_run_v2_service.mcp_server.location
  name     = google_cloud_run_v2_service.mcp_server.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ============================================
# OUTPUTS
# ============================================

output "cloudrun_service_url" {
  description = "URL of the Cloud Run MCP server"
  value       = google_cloud_run_v2_service.mcp_server.uri
}

output "cloudrun_service_name" {
  description = "Name of the Cloud Run service"
  value       = google_cloud_run_v2_service.mcp_server.name
}

output "cloudrun_region" {
  description = "Region where Cloud Run service is deployed"
  value       = google_cloud_run_v2_service.mcp_server.location
}
