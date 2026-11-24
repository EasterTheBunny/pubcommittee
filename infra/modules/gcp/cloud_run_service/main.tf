resource "google_cloud_run_v2_service" "this" {
  project             = var.project
  location            = var.region
  name                = var.name
  ingress             = var.ingress
  deletion_protection = var.deletion_protection
  labels              = var.labels

  template {
    service_account                  = var.service_account_email
    max_instance_request_concurrency = var.concurrency
    timeout                          = "${var.timeout_seconds}s"

    # Scale to zero: minimum is fixed at 0.
    scaling {
      min_instance_count = 0
      max_instance_count = var.max_instances
    }

    containers {
      name  = "app"
      image = var.image

      ports {
        container_port = var.container_port
      }

      resources {
        limits = {
          cpu    = var.cpu
          memory = var.memory
        }
        cpu_idle = true
      }

      dynamic "env" {
        for_each = var.env
        content {
          name  = env.key
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

    dynamic "containers" {
      for_each = var.metrics_sidecar == null ? [] : [var.metrics_sidecar]
      content {
        name  = "metrics"
        image = containers.value.image
        args  = containers.value.args

        dynamic "env" {
          for_each = containers.value.env
          content {
            name  = env.key
            value = env.value
          }
        }
      }
    }
  }

  lifecycle {
    # Deploys (M1-17) change the image by passing a new digest as input.
    # Scaling is set per revision in template.scaling. The service-level scaling
    # block is never set here, but Cloud Run reports its defaults (0) back, which
    # would show as a change on every plan.
    ignore_changes = [scaling]

    precondition {
      condition     = !(var.allow_unauthenticated && length(var.invokers) > 0)
      error_message = "Set either allow_unauthenticated or invokers, not both."
    }
  }
}

resource "google_cloud_run_v2_service_iam_member" "invokers" {
  for_each = var.invokers

  project  = var.project
  location = var.region
  name     = google_cloud_run_v2_service.this.name
  role     = "roles/run.invoker"
  member   = each.value
}

resource "google_cloud_run_v2_service_iam_member" "public" {
  count = var.allow_unauthenticated ? 1 : 0

  project  = var.project
  location = var.region
  name     = google_cloud_run_v2_service.this.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
