# Local Variables - Bootstrap Phase
# Loads config.yaml from project root and defines derived values

locals {
  # Load configuration from project root config.yaml
  config = yamldecode(file("${path.root}/../config.yaml"))

  # Extract global values
  prefix       = local.config.prefix
  project_name = local.config.project_name
  env          = local.config.env
  description  = local.config.description

  # Extract GCP-specific values
  project_id = local.config.project_id
  location   = local.config.location
  services   = lookup(local.config, "services", [])
  labels     = lookup(local.config, "labels", {})

  # Calculate location_id for resource naming
  # Converts europe-west1 -> ew1, us-central1 -> uc1, etc.
  # Multi-region locations (us, eu, asia) are used as-is
  location_id = (
    contains(["us", "eu", "asia"], local.location)
    ? local.location
    : "${substr(local.location, 0, 1)}${substr(split("-", local.location)[1], 0, 1)}${regex("\\d+", local.location)}"
  )

  # Resource naming pattern: {prefix}-{resource}-{location_id}-{env}
  state_bucket_name = "smo-tfstate-${local.location_id}-${local.env}"

  # Service account naming pattern: {prefix}-{service}-{env} (max 30 chars)
  service_accounts = {
    cloudrun   = "${local.prefix}-cloudrun-${local.env}"
    cloudbuild = "${local.prefix}-cloudbuild-${local.env}"
  }
}
