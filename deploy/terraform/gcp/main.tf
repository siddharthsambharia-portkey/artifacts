# GCP: GCS + Cloud SQL + Internal Load Balancer
# This is a starter template — customize for your VPC.

terraform {
  required_providers {
    google = { source = "hashicorp/google", version = "~> 5.0" }
  }
}

variable "project_id" { type = string }
variable "region"     { type = string default = "us-central1" }

resource "google_storage_bucket" "sites" {
  name     = "${var.project_id}-artifact-sites"
  location = var.region
}

resource "google_sql_database_instance" "artifact" {
  name             = "artifact"
  database_version = "POSTGRES_16"
  region           = var.region
  settings {
    tier = "db-f1-micro"
  }
}

# Deploy Artifact VM or GKE via Helm chart, pointing at the resources above.
