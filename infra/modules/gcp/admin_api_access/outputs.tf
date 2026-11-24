output "operator_count" {
  description = "Number of operator invoker bindings."
  value       = length(google_cloud_run_v2_service_iam_member.operators)
}
