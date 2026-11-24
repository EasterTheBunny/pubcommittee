# Native mode, pay per use: no always-on cost.
resource "google_firestore_database" "this" {
  project                 = var.project
  name                    = var.database_name
  location_id             = var.location
  type                    = "FIRESTORE_NATIVE"
  deletion_policy         = var.deletion_protection ? "ABANDON" : "DELETE"
  delete_protection_state = var.deletion_protection ? "DELETE_PROTECTION_ENABLED" : "DELETE_PROTECTION_DISABLED"
}

resource "google_firestore_index" "this" {
  for_each = var.indexes

  project     = var.project
  database    = google_firestore_database.this.name
  collection  = each.value.collection
  query_scope = each.value.query_scope

  dynamic "fields" {
    for_each = each.value.fields
    content {
      field_path   = fields.value.field_path
      order        = fields.value.order
      array_config = fields.value.array_config
    }
  }
}
