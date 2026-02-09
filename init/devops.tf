# DevOps Service Account for Cloud Build/Terraform deployments
# Minimal permissions for CI/CD pipeline

resource "google_service_account" "devops" {
  account_id   = local.devops_sa_name
  display_name = "Google Slides MCP - DevOps Service Account"
  description  = "Service account for Cloud Build and Terraform deployments"
  project      = local.project_id
}

# Grant Artifact Registry writer (for pushing images)
resource "google_project_iam_member" "devops_artifactregistry" {
  project = local.project_id
  role    = "roles/artifactregistry.writer"
  member  = "serviceAccount:${google_service_account.devops.email}"
}

# Grant Cloud Run developer (for deploying services)
resource "google_project_iam_member" "devops_run" {
  project = local.project_id
  role    = "roles/run.developer"
  member  = "serviceAccount:${google_service_account.devops.email}"
}

# Grant Service Account user (for deploying with custom SA)
resource "google_project_iam_member" "devops_sa_user" {
  project = local.project_id
  role    = "roles/iam.serviceAccountUser"
  member  = "serviceAccount:${google_service_account.devops.email}"
}

# Grant Cloud Build builder
resource "google_project_iam_member" "devops_builder" {
  project = local.project_id
  role    = "roles/cloudbuild.builds.builder"
  member  = "serviceAccount:${google_service_account.devops.email}"
}

# ============================================
# OUTPUTS
# ============================================

output "devops_service_account_email" {
  value       = google_service_account.devops.email
  description = "Email of the DevOps service account"
}
