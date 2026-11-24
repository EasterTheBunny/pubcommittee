resource "google_storage_bucket" "this" {
  project                     = var.project
  name                        = var.name
  location                    = var.location
  storage_class               = var.storage_class
  force_destroy               = var.force_destroy
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"

  versioning {
    enabled = var.versioning
  }

  dynamic "lifecycle_rule" {
    for_each = var.delete_after_days == null ? [] : [var.delete_after_days]
    content {
      condition {
        age = lifecycle_rule.value
      }
      action {
        type = "Delete"
      }
    }
  }
}

resource "google_storage_bucket_iam_member" "admins" {
  for_each = var.object_admins

  bucket = google_storage_bucket.this.name
  role   = "roles/storage.objectAdmin"
  member = each.value
}

resource "google_storage_bucket_iam_member" "viewers" {
  for_each = var.object_viewers

  bucket = google_storage_bucket.this.name
  role   = "roles/storage.objectViewer"
  member = each.value
}
