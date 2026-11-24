resource "google_service_account" "this" {
  project      = var.project
  account_id   = var.account_id
  display_name = var.display_name
}

resource "google_project_iam_member" "roles" {
  for_each = var.project_roles

  project = var.project
  role    = each.value
  member  = "serviceAccount:${google_service_account.this.email}"
}
