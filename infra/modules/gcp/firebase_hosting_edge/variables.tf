variable "project" {
  type        = string
  description = "Project that owns the Firebase Hosting site."
}

variable "site_id" {
  type        = string
  description = "Hosting site id."
}

variable "rewrites" {
  type = list(object({
    glob       = string
    service_id = string
    region     = string
  }))
  description = "Path globs rewritten to Cloud Run services. Only public services (read-api, or the web UI on its own site) belong here; admin-api is never routed through the edge."
}

variable "headers" {
  type = list(object({
    glob          = string
    cache_control = string
  }))
  description = "Cache-Control per path glob. Public GETs are cached by their Cache-Control; credentialed responses are never cached. Values are inputs (no defense or cost tuning here)."
  default     = []
}

variable "custom_domain" {
  type        = string
  description = "Custom domain for the site (api.<domain>, or <domain> for the web UI). Null runs on the provider domain."
  default     = null
}
