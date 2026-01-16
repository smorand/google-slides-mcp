# Service Accounts and IAM Roles
# Creates dedicated service accounts for Cloud Run and Cloud Build

# ============================================
# CLOUD RUN SERVICE ACCOUNT
# ============================================

resource "google_service_account" "cloudrun" {
  account_id   = local.service_accounts.cloudrun
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
# CLOUD BUILD SERVICE ACCOUNT
# ============================================

resource "google_service_account" "cloudbuild" {
  account_id   = local.service_accounts.cloudbuild
  display_name = "Google Slides MCP - Cloud Build Service Account"
  description  = "Custom service account for Cloud Build"
  project      = local.project_id
}

# Grant Artifact Registry writer (for pushing images)
resource "google_project_iam_member" "cloudbuild_artifactregistry" {
  project = local.project_id
  role    = "roles/artifactregistry.writer"
  member  = "serviceAccount:${google_service_account.cloudbuild.email}"
}

# Grant Cloud Run developer (for deploying services)
resource "google_project_iam_member" "cloudbuild_run" {
  project = local.project_id
  role    = "roles/run.developer"
  member  = "serviceAccount:${google_service_account.cloudbuild.email}"
}

# Grant Service Account user (for deploying with custom SA)
resource "google_project_iam_member" "cloudbuild_sa_user" {
  project = local.project_id
  role    = "roles/iam.serviceAccountUser"
  member  = "serviceAccount:${google_service_account.cloudbuild.email}"
}

# Grant Cloud Build builder
resource "google_project_iam_member" "cloudbuild_builder" {
  project = local.project_id
  role    = "roles/cloudbuild.builds.builder"
  member  = "serviceAccount:${google_service_account.cloudbuild.email}"
}

# ============================================
# OUTPUTS
# ============================================

output "cloudrun_service_account_email" {
  value       = google_service_account.cloudrun.email
  description = "Email of the Cloud Run service account"
}

output "cloudbuild_service_account_email" {
  value       = google_service_account.cloudbuild.email
  description = "Email of the Cloud Build service account"
}
