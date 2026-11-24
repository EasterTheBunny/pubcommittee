output "site_id" {
  description = "Hosting site id."
  value       = google_firebase_hosting_site.this.site_id
}

output "default_url" {
  description = "Provider-domain URL of the site. Environment value: do not publish it."
  value       = google_firebase_hosting_site.this.default_url
}
