# IAM Configuration
# Service accounts and roles for the MCP server

# ============================================
# SERVICE ACCOUNTS
# ============================================

# Cloud Run service account - runs the MCP server
resource "google_service_account" "cloudrun" {
  account_id   = "${local.resource_prefix}-cloudrun"
  display_name = "Cloud Run Service Account for ${local.prefix}"
  description  = "Service account used by the Google Slides MCP Cloud Run service"

  depends_on = [google_project_service.required_apis["iam.googleapis.com"]]
}

# Cloud Build service account - builds and deploys container images
resource "google_service_account" "cloudbuild" {
  account_id   = "${local.resource_prefix}-cloudbuild"
  display_name = "Cloud Build Service Account for ${local.prefix}"
  description  = "Service account used by Cloud Build for CI/CD"

  depends_on = [google_project_service.required_apis["iam.googleapis.com"]]
}

# ============================================
# CLOUD RUN SERVICE ACCOUNT ROLES
# ============================================

# Allow Cloud Run service to access Firestore
resource "google_project_iam_member" "cloudrun_firestore" {
  project = local.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.cloudrun.email}"
}

# Allow Cloud Run service to access Secret Manager
resource "google_project_iam_member" "cloudrun_secrets" {
  project = local.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.cloudrun.email}"
}

# Allow Cloud Run service to write logs
resource "google_project_iam_member" "cloudrun_logging" {
  project = local.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.cloudrun.email}"
}

# Allow Cloud Run service to write metrics
resource "google_project_iam_member" "cloudrun_monitoring" {
  project = local.project_id
  role    = "roles/monitoring.metricWriter"
  member  = "serviceAccount:${google_service_account.cloudrun.email}"
}

# ============================================
# CLOUD BUILD SERVICE ACCOUNT ROLES
# ============================================

# Allow Cloud Build to build images
resource "google_project_iam_member" "cloudbuild_builder" {
  project = local.project_id
  role    = "roles/cloudbuild.builds.builder"
  member  = "serviceAccount:${google_service_account.cloudbuild.email}"
}

# Allow Cloud Build to push to Artifact Registry
resource "google_project_iam_member" "cloudbuild_artifacts" {
  project = local.project_id
  role    = "roles/artifactregistry.writer"
  member  = "serviceAccount:${google_service_account.cloudbuild.email}"
}

# Allow Cloud Build to deploy to Cloud Run
resource "google_project_iam_member" "cloudbuild_run_admin" {
  project = local.project_id
  role    = "roles/run.admin"
  member  = "serviceAccount:${google_service_account.cloudbuild.email}"
}

# Allow Cloud Build to act as Cloud Run service account
resource "google_service_account_iam_member" "cloudbuild_act_as_cloudrun" {
  service_account_id = google_service_account.cloudrun.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.cloudbuild.email}"
}

# ============================================
# OUTPUTS
# ============================================

output "cloudrun_service_account_email" {
  description = "Email of the Cloud Run service account"
  value       = google_service_account.cloudrun.email
}

output "cloudbuild_service_account_email" {
  description = "Email of the Cloud Build service account"
  value       = google_service_account.cloudbuild.email
}
