# Terraform Provider Configuration - Bootstrap Phase
# Uses local backend (no remote state yet)

terraform {
  required_version = ">= 1.5.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }

  # Local backend for bootstrap phase
  # State stored locally until GCS bucket is created
}

provider "google" {
  project = local.project_id
  region  = local.location
}
