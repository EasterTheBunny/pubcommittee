resource "google_cloud_run_v2_job" "this" {
  project             = var.project
  location            = var.region
  name                = var.name
  deletion_protection = var.deletion_protection
  labels              = var.labels

  template {
    template {
      service_account = var.service_account_email
      timeout         = "${var.timeout_seconds}s"
      max_retries     = var.max_retries

      containers {
        image = var.image

        resources {
          limits = {
            cpu    = var.cpu
            memory = var.memory
          }
        }

        dynamic "env" {
          for_each = var.env
          content {
            name  = env.key
            value = env.value
          }
        }

        dynamic "env" {
          for_each = var.otlp_endpoint == null ? [] : [var.otlp_endpoint]
          content {
            name  = "OTEL_EXPORTER_OTLP_ENDPOINT"
            value = env.value
          }
        }

        dynamic "env" {
          for_each = var.secret_env
          content {
            name = env.key
            value_source {
              secret_key_ref {
                secret  = env.value.secret
                version = env.value.version
              }
            }
          }
        }
      }
    }
  }
}

# The scheduler account may only run this job.
resource "google_cloud_run_v2_job_iam_member" "scheduler" {
  count = var.schedule == null ? 0 : 1

  project  = var.project
  location = var.region
  name     = google_cloud_run_v2_job.this.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${var.schedule.service_account_email}"
}

resource "google_cloud_scheduler_job" "this" {
  count = var.schedule == null ? 0 : 1

  project   = var.project
  region    = var.region
  name      = "${var.name}-schedule"
  schedule  = var.schedule.cron
  time_zone = var.schedule.time_zone

  http_target {
    http_method = "POST"
    uri         = "https://run.googleapis.com/v2/projects/${var.project}/locations/${var.region}/jobs/${google_cloud_run_v2_job.this.name}:run"

    oauth_token {
      service_account_email = var.schedule.service_account_email
    }
  }
}
