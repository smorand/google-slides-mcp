# Secret Manager Configuration
# Secrets for OAuth2 credentials

# ============================================
# OAUTH2 CREDENTIALS SECRETS
# ============================================

# OAuth2 Client ID
resource "google_secret_manager_secret" "oauth_client_id" {
  secret_id = "${local.resource_prefix}-oauth-client-id"

  labels = local.common_labels

  replication {
    auto {}
  }

  depends_on = [google_project_service.required_apis["secretmanager.googleapis.com"]]
}

# OAuth2 Client Secret
resource "google_secret_manager_secret" "oauth_client_secret" {
  secret_id = "${local.resource_prefix}-oauth-client-secret"

  labels = local.common_labels

  replication {
    auto {}
  }

  depends_on = [google_project_service.required_apis["secretmanager.googleapis.com"]]
}

# OAuth2 Redirect URI (for callback configuration)
resource "google_secret_manager_secret" "oauth_redirect_uri" {
  secret_id = "${local.resource_prefix}-oauth-redirect-uri"

  labels = local.common_labels

  replication {
    auto {}
  }

  depends_on = [google_project_service.required_apis["secretmanager.googleapis.com"]]
}

# ============================================
# SECRET ACCESS FOR CLOUD RUN
# ============================================

# Grant Cloud Run service account access to OAuth Client ID secret
resource "google_secret_manager_secret_iam_member" "cloudrun_oauth_client_id" {
  secret_id = google_secret_manager_secret.oauth_client_id.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.cloudrun.email}"
}

# Grant Cloud Run service account access to OAuth Client Secret
resource "google_secret_manager_secret_iam_member" "cloudrun_oauth_client_secret" {
  secret_id = google_secret_manager_secret.oauth_client_secret.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.cloudrun.email}"
}

# Grant Cloud Run service account access to OAuth Redirect URI
resource "google_secret_manager_secret_iam_member" "cloudrun_oauth_redirect_uri" {
  secret_id = google_secret_manager_secret.oauth_redirect_uri.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.cloudrun.email}"
}

# ============================================
# OUTPUTS
# ============================================

output "oauth_client_id_secret_id" {
  description = "Secret Manager secret ID for OAuth2 Client ID"
  value       = google_secret_manager_secret.oauth_client_id.secret_id
}

output "oauth_client_secret_secret_id" {
  description = "Secret Manager secret ID for OAuth2 Client Secret"
  value       = google_secret_manager_secret.oauth_client_secret.secret_id
}

output "oauth_redirect_uri_secret_id" {
  description = "Secret Manager secret ID for OAuth2 Redirect URI"
  value       = google_secret_manager_secret.oauth_redirect_uri.secret_id
}
