# GCP: GCS + Cloud SQL + Workload Identity service account
# This is a starter template — customize for your VPC.
# Provisions the storage, database, and application identity needed for the
# guaranteed GCP + Okta deployment profile.
#
# What this starter does NOT wire:
#   - Internal Application Load Balancer
#   - Identity Aware Proxy (IAP)
#   - Wildcard TLS certificate
#   - VPC peering / private service networking
# Add those before going to production.

terraform {
  required_providers {
    google = { source = "hashicorp/google", version = "~> 5.0" }
  }
}

variable "project_id" {
  type = string
}

variable "region" {
  type    = string
  default = "us-central1"
}

# ── GCS bucket for site storage ──────────────────────────────────────────────

resource "google_storage_bucket" "sites" {
  name     = "${var.project_id}-artifact-sites"
  location = var.region
  project  = var.project_id

  uniform_bucket_level_access = true
}

# ── Cloud SQL Postgres 16 ────────────────────────────────────────────────────

resource "google_sql_database_instance" "artifact" {
  name             = "artifact"
  database_version = "POSTGRES_16"
  region           = var.region
  project          = var.project_id

  settings {
    tier = "db-f1-micro"
  }

  deletion_protection = false
}

# ── Workload Identity service account ────────────────────────────────────────
# The pod running Artifact uses this GCP SA via Workload Identity (ADC).
# No JSON key is required; the GCS driver picks up credentials automatically.

resource "google_service_account" "artifact" {
  account_id   = "artifact-app"
  display_name = "Artifact application service account"
  project      = var.project_id
}

# Grant the SA read/write access to the sites bucket.
resource "google_storage_bucket_iam_member" "artifact_sa_bucket" {
  bucket = google_storage_bucket.sites.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.artifact.email}"
}

# ── Outputs ───────────────────────────────────────────────────────────────────
# Pass these values into the Helm chart after apply.

# → Helm: config.storage.bucket
output "storage_bucket" {
  description = "GCS bucket name. Set as config.storage.bucket in values.yaml."
  value       = google_storage_bucket.sites.name
}

# → Helm: used to build externalDatabase.url via the Cloud SQL Auth Proxy
# Full DSN pattern: postgres://artifact:<password>@/<db>?host=/cloudsql/<connection_name>
output "cloudsql_connection_name" {
  description = "Cloud SQL instance connection name (project:region:instance). Used to build externalDatabase.url in values.yaml."
  value       = google_sql_database_instance.artifact.connection_name
}

# → Helm: serviceAccount.annotations."iam.gke.io/gcp-service-account"
output "workload_identity_sa_email" {
  description = "GCP service account email. Annotate the Kubernetes ServiceAccount with iam.gke.io/gcp-service-account=<this value> to enable Workload Identity."
  value       = google_service_account.artifact.email
}
