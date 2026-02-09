# GCP APIs/Services Enablement
# Enables all required APIs on the GCP project

resource "google_project_service" "required_apis" {
  for_each = toset(local.services)

  project = local.project_id
  service = each.value

  # Don't disable APIs when removing from config (safety)
  disable_on_destroy = false

  # Don't disable dependent services
  disable_dependent_services = false

  timeouts {
    create = "30m"
    update = "40m"
  }
}

# Output enabled services for verification
output "enabled_services" {
  value       = [for s in google_project_service.required_apis : s.service]
  description = "List of enabled GCP APIs"
}
