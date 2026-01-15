# Firestore Configuration
# Database for API keys and refresh tokens

# ============================================
# FIRESTORE DATABASE
# ============================================

# Native mode Firestore database
resource "google_firestore_database" "main" {
  provider = google-beta

  project     = local.project_id
  name        = "(default)"
  location_id = local.gcp_params.firestore_location
  type        = "FIRESTORE_NATIVE"

  # Concurrency mode for better performance
  concurrency_mode = "OPTIMISTIC"

  # Point-in-time recovery disabled (can be enabled for prod)
  point_in_time_recovery_enablement = "POINT_IN_TIME_RECOVERY_DISABLED"

  # Delete protection disabled for dev (enable for prod)
  delete_protection_state = "DELETE_PROTECTION_DISABLED"

  depends_on = [google_project_service.required_apis["firestore.googleapis.com"]]
}

# ============================================
# FIRESTORE INDEXES
# ============================================

# Index for API keys collection - lookup by api_key field
resource "google_firestore_index" "api_keys_by_key" {
  provider = google-beta

  project    = local.project_id
  database   = google_firestore_database.main.name
  collection = "api_keys"

  fields {
    field_path = "api_key"
    order      = "ASCENDING"
  }

  fields {
    field_path = "__name__"
    order      = "ASCENDING"
  }

  depends_on = [google_firestore_database.main]
}

# Index for API keys collection - lookup by user_email
resource "google_firestore_index" "api_keys_by_email" {
  provider = google-beta

  project    = local.project_id
  database   = google_firestore_database.main.name
  collection = "api_keys"

  fields {
    field_path = "user_email"
    order      = "ASCENDING"
  }

  fields {
    field_path = "created_at"
    order      = "DESCENDING"
  }

  fields {
    field_path = "__name__"
    order      = "ASCENDING"
  }

  depends_on = [google_firestore_database.main]
}

# Index for cleanup - find keys by last_used timestamp
resource "google_firestore_index" "api_keys_by_last_used" {
  provider = google-beta

  project    = local.project_id
  database   = google_firestore_database.main.name
  collection = "api_keys"

  fields {
    field_path = "last_used"
    order      = "ASCENDING"
  }

  fields {
    field_path = "__name__"
    order      = "ASCENDING"
  }

  depends_on = [google_firestore_database.main]
}

# ============================================
# OUTPUTS
# ============================================

output "firestore_database_name" {
  description = "Name of the Firestore database"
  value       = google_firestore_database.main.name
}

output "firestore_location" {
  description = "Location of the Firestore database"
  value       = google_firestore_database.main.location_id
}
