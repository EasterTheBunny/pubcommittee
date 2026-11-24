output "email" {
  description = "Service account email (an identifier, not a credential)."
  value       = google_service_account.this.email
}

output "member" {
  description = "IAM member string for this account."
  value       = "serviceAccount:${google_service_account.this.email}"
}
