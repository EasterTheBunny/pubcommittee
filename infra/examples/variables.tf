variable "project" {
  type        = string
  description = "Project id (fake in this example)."
}

variable "region" {
  type        = string
  description = "Region for Cloud Run, Scheduler, and buckets."
}

variable "firestore_location" {
  type        = string
  description = "Firestore location id."
}

variable "images" {
  type        = map(string)
  description = "Image reference by unit name (read-api, admin-api, job-ingest, job-summarize, job-exports)."
}

variable "bucket_names" {
  type        = map(string)
  description = "Bucket name by purpose (documents, exports)."
}

variable "operator_principals" {
  type        = set(string)
  description = "Operator IAM members for admin-api (fake in this example)."
}

variable "deploy_service_account_member" {
  type        = string
  description = "Deploy service account member (fake in this example)."
}

variable "metrics_sidecar_image" {
  type        = string
  description = "Managed Prometheus sidecar image."
}

variable "metrics_sidecar_scrape_interval" {
  type        = string
  description = "Scrape interval passed to the sidecar. Open question: no default."
}

variable "otlp_endpoint" {
  type        = string
  description = "OTLP endpoint for jobs."
}

variable "ingest_schedule" {
  type = object({
    cron      = string
    time_zone = string
  })
  description = "Schedule for job-ingest, or null for manual runs. Open question: which jobs are scheduled, and when."
}

variable "edge_cache_control" {
  type        = string
  description = "Cache-Control for public GETs at the API edge."
}
