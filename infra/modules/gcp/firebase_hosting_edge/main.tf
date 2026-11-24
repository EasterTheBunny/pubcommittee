// Firebase Hosting as the API edge: CDN and TLS without a load balancer.
resource "google_firebase_hosting_site" "this" {
  provider = google-beta

  project = var.project
  site_id = var.site_id
}

resource "google_firebase_hosting_version" "this" {
  provider = google-beta

  site_id = google_firebase_hosting_site.this.site_id

  config {
    dynamic "rewrites" {
      for_each = var.rewrites
      content {
        glob = rewrites.value.glob
        run {
          service_id = rewrites.value.service_id
          region     = rewrites.value.region
        }
      }
    }

    dynamic "headers" {
      for_each = var.headers
      content {
        glob = headers.value.glob
        headers = {
          "Cache-Control" = headers.value.cache_control
        }
      }
    }
  }
}

resource "google_firebase_hosting_release" "this" {
  provider = google-beta

  site_id      = google_firebase_hosting_site.this.site_id
  version_name = google_firebase_hosting_version.this.name
  message      = "managed by infrastructure code"
}

resource "google_firebase_hosting_custom_domain" "this" {
  provider = google-beta
  count    = var.custom_domain == null ? 0 : 1

  project       = var.project
  site_id       = google_firebase_hosting_site.this.site_id
  custom_domain = var.custom_domain
}
