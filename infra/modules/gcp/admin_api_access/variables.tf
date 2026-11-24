variable "project" {
  type        = string
  description = "Project that owns the admin-api service."
}

variable "region" {
  type        = string
  description = "Cloud Run region of the service."
}

variable "service_name" {
  type        = string
  description = "Name of the admin-api Cloud Run service."
}

variable "operator_principals" {
  type        = set(string)
  description = "IAM members (for example user:<operator-principal>) for this environment, taken from the private operator roster. Never committed here."

  validation {
    condition     = !contains(var.operator_principals, "allUsers") && !contains(var.operator_principals, "allAuthenticatedUsers")
    error_message = "Public principals are not allowed on admin-api."
  }
}

variable "deploy_service_account_member" {
  type        = string
  description = "Deploy service account member (serviceAccount:...). Null grants no deploy access."
  default     = null
}
