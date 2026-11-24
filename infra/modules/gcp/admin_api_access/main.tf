// Operator-only invoker bindings for admin-api. admin-api is never routed
// through the API edge and never allows unauthenticated calls.
resource "google_cloud_run_v2_service_iam_member" "operators" {
  for_each = var.operator_principals

  project  = var.project
  location = var.region
  name     = var.service_name
  role     = "roles/run.invoker"
  member   = each.value
}

// Cloud Run IAM cannot limit a caller to one route. The deploy account gets
// invoker access here, and admin-api itself accepts it only on the roster
// route (architecture, Administration).
resource "google_cloud_run_v2_service_iam_member" "deploy" {
  count = var.deploy_service_account_member == null ? 0 : 1

  project  = var.project
  location = var.region
  name     = var.service_name
  role     = "roles/run.invoker"
  member   = var.deploy_service_account_member
}
