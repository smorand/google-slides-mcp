# Service Accounts for Application Workloads
# Creates dedicated service accounts for Cloud Run

# ============================================
# CLOUD RUN SERVICE ACCOUNT
# ============================================

resource "google_service_account" "cloudrun" {
  account_id   = "${local.prefix}-cloudrun-${local.env}"
  display_name = "Google Slides MCP - Cloud Run Service Account"
  description  = "Custom service account for Cloud Run services"
  project      = local.project_id
}

# Grant Secret Manager access (for OAuth credentials)
resource "google_project_iam_member" "cloudrun_secretmanager" {
  project = local.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.cloudrun.email}"
}

# Grant Cloud Trace agent (for distributed tracing)
resource "google_project_iam_member" "cloudrun_trace" {
  project = local.project_id
  role    = "roles/cloudtrace.agent"
  member  = "serviceAccount:${google_service_account.cloudrun.email}"
}

# Grant Cloud Logging writer
resource "google_project_iam_member" "cloudrun_logging" {
  project = local.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.cloudrun.email}"
}

# Grant Cloud Monitoring metric writer
resource "google_project_iam_member" "cloudrun_monitoring" {
  project = local.project_id
  role    = "roles/monitoring.metricWriter"
  member  = "serviceAccount:${google_service_account.cloudrun.email}"
}

# Grant Firestore data user (for API key storage)
resource "google_project_iam_member" "cloudrun_firestore" {
  project = local.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.cloudrun.email}"
}

# ============================================
# OUTPUTS
# ============================================

output "cloudrun_service_account_email" {
  value       = google_service_account.cloudrun.email
  description = "Email of the Cloud Run service account"
}
