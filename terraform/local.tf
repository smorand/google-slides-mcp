# Local Variables Configuration
# Loads config.yaml and defines derived values

locals {
  # Load configuration from config.yaml
  config_file = yamldecode(file("${path.root}/config.yaml"))
  config      = local.config_file

  # Extract global values
  prefix      = local.config.prefix
  env         = local.config.env
  description = local.config.description

  # Extract GCP-specific values
  gcp_config = local.config.gcp
  project_id = local.gcp_config.project_id
  location   = local.gcp_config.location
  services   = local.gcp_config.services

  # Cloud Run resources
  cloud_run_config = local.gcp_config.resources.cloud_run

  # GCP parameters
  gcp_params = local.gcp_config.parameters

  # Application parameters
  app_params = local.config.parameters

  # Calculate location_id for resource naming
  # Converts europe-west1 -> ew1, us-central1 -> uc1, etc.
  location_id = (
    contains(["us", "eu", "asia"], local.location)
    ? local.location
    : "${substr(local.location, 0, 1)}${substr(split("-", local.location)[1], 0, 1)}${regex("\\d+", local.location)}"
  )

  # Standard resource naming
  resource_prefix = "${local.prefix}-${local.env}"

  # Common labels for all resources
  common_labels = {
    project     = local.prefix
    environment = local.env
    managed_by  = "terraform"
  }
}
