output "name" {
  description = "Service name."
  value       = google_cloud_run_v2_service.this.name
}

output "uri" {
  description = "Service URL. Environment value: do not publish it."
  value       = google_cloud_run_v2_service.this.uri
}
