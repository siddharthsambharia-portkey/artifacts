# AWS: S3 + RDS + ALB with OIDC

terraform {
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 5.0" }
  }
}

variable "region" { type = string default = "us-east-1" }

resource "aws_s3_bucket" "sites" {
  bucket = "artifact-sites"
}

resource "aws_db_instance" "artifact" {
  identifier     = "artifact"
  engine         = "postgres"
  engine_version = "16"
  instance_class = "db.t3.micro"
  allocated_storage = 20
  username       = "artifact"
  password       = var.db_password
  skip_final_snapshot = true
}

variable "db_password" {
  type      = string
  sensitive = true
}

# Add ALB + OIDC auth action, deploy Artifact on ECS/EC2/K8s.
