# Local Variables - Main Infrastructure
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
  labels     = lookup(local.config, "labels", {})

  # Calculate location_id for resource naming
  # Converts europe-west1 -> ew1, us-central1 -> uc1, etc.
  location_id = (
    contains(["us", "eu", "asia"], local.location)
    ? local.location
    : "${substr(local.location, 0, 1)}${substr(split("-", local.location)[1], 0, 1)}${regex("\\d+", local.location)}"
  )

  # Resource configuration
  resources  = lookup(local.config, "resources", {})
  parameters = lookup(local.config, "parameters", {})

  # Cloud Run configuration
  mcp_name           = "${local.prefix}-${local.project_name}-${local.env}"
  mcp_cpu            = lookup(local.resources, "cpu", "1")
  mcp_memory         = lookup(local.resources, "memory", "512Mi")
  mcp_min_instances  = lookup(local.resources, "min_instances", 0)
  mcp_max_instances  = lookup(local.resources, "max_instances", 5)
  mcp_timeout        = lookup(local.resources, "timeout_seconds", 300)
  mcp_concurrency    = lookup(local.resources, "concurrency", 80)

  # Note: Cloud Run service account is created in service-accounts.tf

  # Secret name for OAuth credentials
  oauth_secret_name = lookup(local.parameters, "oauth_secret_name", "smo-gslides-oauth-creds")

  # Application parameters
  cache_ttl      = lookup(local.parameters, "cache_ttl", "5m")
  max_retries    = lookup(local.parameters, "max_retries", "5")
  log_level      = lookup(local.parameters, "log_level", "info")
  rate_limit_rps = lookup(local.parameters, "rate_limit_rps", "100")

  # Docker image in Artifact Registry
  docker_image = "${local.location}-docker.pkg.dev/${local.project_id}/${local.prefix}-${local.project_name}-${local.env}/${local.project_name}:latest"
}
