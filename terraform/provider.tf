# Terraform Provider Configuration
# Defines the required providers and backend configuration

terraform {
  required_version = ">= 1.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 5.0"
    }
  }

  # Backend configuration - uncomment after init-deploy creates the bucket
  # backend "gcs" {
  #   bucket = "gslides-tfstate-ew1-dev"
  #   prefix = "terraform/state"
  # }
}

provider "google" {
  project = local.project_id
  region  = local.location
}

provider "google-beta" {
  project = local.project_id
  region  = local.location
}
