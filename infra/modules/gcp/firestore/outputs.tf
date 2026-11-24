output "database_name" {
  description = "Database id."
  value       = google_firestore_database.this.name
}
