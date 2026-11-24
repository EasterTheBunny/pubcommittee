variable "project" {
  type        = string
  description = "Project that owns the database."
}

variable "location" {
  type        = string
  description = "Firestore location id."
}

variable "database_name" {
  type        = string
  description = "Database id. Use \"(default)\" for the default database."
}

variable "deletion_protection" {
  type        = bool
  description = "Block deletion of the database."
  default     = true
}

variable "indexes" {
  description = "Composite indexes. The Firestore adapter (M1-03b) owns the list; the deployment passes it in."
  type = map(object({
    collection  = string
    query_scope = optional(string, "COLLECTION")
    fields = list(object({
      field_path   = string
      order        = optional(string)
      array_config = optional(string)
    }))
  }))
  default = {}
}

variable "exempt_fields" {
  description = "Fields exempt from single-field indexing, keyed by a stable name. The Firestore adapter (M1-03b) owns the list (fieldOverrides with no indexes)."
  type = map(object({
    collection = string
    field      = string
  }))
  default = {}
}
