# Terraform State Backend - GCS Bucket
# Creates the bucket used for storing terraform state

resource "google_storage_bucket" "terraform_state" {
  name     = local.state_bucket_name
  location = local.location
  project  = local.project_id

  # Enable versioning for state file protection
  versioning {
    enabled = true
  }

  # Lifecycle rule to clean up old versions after 30 days
  lifecycle_rule {
    condition {
      num_newer_versions = 5
      with_state         = "ARCHIVED"
    }
    action {
      type = "Delete"
    }
  }

  # Prevent accidental deletion
  force_destroy = false

  # Uniform bucket-level access (recommended)
  uniform_bucket_level_access = true

  labels = local.labels
}

# Output the bucket name for use in iac/provider.tf
output "state_bucket_name" {
  value       = google_storage_bucket.terraform_state.name
  description = "Name of the GCS bucket for terraform state"
}

output "state_bucket_url" {
  value       = google_storage_bucket.terraform_state.url
  description = "URL of the GCS bucket for terraform state"
}
