terraform {
  required_version = ">= 1.6.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 6.0, < 7.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = ">= 6.0, < 7.0"
    }
  }
}

# Fake token: this root is only validated and planned with -refresh=false,
# never applied. Real environment roots live in the private deployment repository.
provider "google" {
  project      = var.project
  region       = var.region
  access_token = "fake-token-for-plan-only"
}

provider "google-beta" {
  project      = var.project
  region       = var.region
  access_token = "fake-token-for-plan-only"
}

locals {
  units = toset(["read-api", "admin-api", "job-ingest", "job-summarize", "job-exports"])
}

# One service account per unit. read-api gets read-only database access and no SQL or key roles.
module "sa" {
  source   = "../modules/gcp/service_account"
  for_each = local.units

  project      = var.project
  account_id   = "ex-${each.key}"
  display_name = each.key
  project_roles = {
    "read-api"      = ["roles/datastore.viewer"]
    "admin-api"     = ["roles/datastore.user"]
    "job-ingest"    = ["roles/datastore.user"]
    "job-summarize" = ["roles/datastore.user", "roles/aiplatform.user"]
    "job-exports"   = ["roles/datastore.viewer"]
  }[each.key]
}

module "sa_scheduler" {
  source = "../modules/gcp/service_account"

  project      = var.project
  account_id   = "ex-scheduler"
  display_name = "scheduler"
}

module "documents" {
  source = "../modules/gcp/bucket"

  project  = var.project
  name     = var.bucket_names["documents"]
  location = var.region
  object_admins = [
    module.sa["admin-api"].member,
    module.sa["job-ingest"].member,
    module.sa["job-summarize"].member,
  ]
  object_viewers = [module.sa["read-api"].member]
}

module "exports" {
  source = "../modules/gcp/bucket"

  project        = var.project
  name           = var.bucket_names["exports"]
  location       = var.region
  object_admins  = [module.sa["job-exports"].member]
  object_viewers = [module.sa["read-api"].member]
}

module "firestore" {
  source = "../modules/gcp/firestore"

  project       = var.project
  location      = var.firestore_location
  database_name = "(default)"

  indexes = {
    bills_example = {
      collection = "bills"
      fields = [
        { field_path = "tracked", order = "ASCENDING" },
        { field_path = "updated_at", order = "DESCENDING" },
      ]
    }
  }
}

module "read_api" {
  source = "../modules/gcp/cloud_run_service"

  project               = var.project
  region                = var.region
  name                  = "read-api"
  image                 = var.images["read-api"]
  service_account_email = module.sa["read-api"].email
  container_port        = 8080
  cpu                   = "1"
  memory                = "512Mi"
  max_instances         = 2
  ingress               = "INGRESS_TRAFFIC_ALL"
  allow_unauthenticated = true
  deletion_protection   = false

  metrics_sidecar = {
    image = var.metrics_sidecar_image
    env   = { SCRAPE_INTERVAL = var.metrics_sidecar_scrape_interval }
  }
}

module "admin_api" {
  source = "../modules/gcp/cloud_run_service"

  project               = var.project
  region                = var.region
  name                  = "admin-api"
  image                 = var.images["admin-api"]
  service_account_email = module.sa["admin-api"].email
  container_port        = 8080
  cpu                   = "1"
  memory                = "512Mi"
  max_instances         = 1
  ingress               = "INGRESS_TRAFFIC_ALL"
  deletion_protection   = false
}

module "admin_api_access" {
  source = "../modules/gcp/admin_api_access"

  project                       = var.project
  region                        = var.region
  service_name                  = module.admin_api.name
  operator_principals           = var.operator_principals
  deploy_service_account_member = var.deploy_service_account_member
}

module "job_ingest" {
  source = "../modules/gcp/cloud_run_job"

  project               = var.project
  region                = var.region
  name                  = "job-ingest"
  image                 = var.images["job-ingest"]
  service_account_email = module.sa["job-ingest"].email
  cpu                   = "1"
  memory                = "512Mi"
  timeout_seconds       = 600
  otlp_endpoint         = var.otlp_endpoint
  deletion_protection   = false

  schedule = var.ingest_schedule == null ? null : {
    cron                  = var.ingest_schedule.cron
    time_zone             = var.ingest_schedule.time_zone
    service_account_email = module.sa_scheduler.email
  }
}

module "job_summarize" {
  source = "../modules/gcp/cloud_run_job"

  project               = var.project
  region                = var.region
  name                  = "job-summarize"
  image                 = var.images["job-summarize"]
  service_account_email = module.sa["job-summarize"].email
  cpu                   = "1"
  memory                = "512Mi"
  timeout_seconds       = 600
  otlp_endpoint         = var.otlp_endpoint
  deletion_protection   = false
}

module "job_exports" {
  source = "../modules/gcp/cloud_run_job"

  project               = var.project
  region                = var.region
  name                  = "job-exports"
  image                 = var.images["job-exports"]
  service_account_email = module.sa["job-exports"].email
  cpu                   = "1"
  memory                = "512Mi"
  timeout_seconds       = 600
  otlp_endpoint         = var.otlp_endpoint
  deletion_protection   = false
}

# API edge: only read-api is routed. admin-api is never listed here.
module "edge" {
  source = "../modules/gcp/firebase_hosting_edge"

  project = var.project
  site_id = "example-edge"

  rewrites = [{
    glob       = "/v1/**"
    service_id = module.read_api.name
    region     = var.region
  }]

  headers = [{
    glob          = "/v1/**"
    cache_control = var.edge_cache_control
  }]
}
