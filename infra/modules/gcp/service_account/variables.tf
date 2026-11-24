variable "project" {
  description = "Project that owns the service account."
  type        = string
}

variable "account_id" {
  description = "Service account id (6-30 characters). One account per unit."
  type        = string
}

variable "display_name" {
  description = "Human-readable name."
  type        = string
}

variable "project_roles" {
  description = "Project-level roles granted to this account. Keep to the unit's needs; read-api never gets SQL or key roles."
  type        = set(string)
  default     = []

  validation {
    condition = alltrue([
      for r in var.project_roles :
      !can(regex("^roles/(cloudsql\\.|cloudkms\\.|owner$|editor$)", r))
    ])
    error_message = "SQL, KMS, owner, and editor roles are not allowed on a unit service account in M1."
  }
}
