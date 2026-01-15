# Google Cloud APIs Enablement
# Enables required APIs for the project

resource "google_project_service" "required_apis" {
  for_each = toset(local.services)

  project            = local.project_id
  service            = each.value
  disable_on_destroy = false

  timeouts {
    create = "10m"
    update = "10m"
  }
}
