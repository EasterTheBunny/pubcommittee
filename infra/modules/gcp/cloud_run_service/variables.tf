variable "project" {
  type        = string
  description = "Project that owns the service."
}

variable "region" {
  type        = string
  description = "Cloud Run region."
}

variable "name" {
  type        = string
  description = "Service name. One service per unit so each deploys independently."
}

variable "image" {
  type        = string
  description = "Full image reference, by digest (registry/path@sha256:...). The registry lives outside this module."
}

variable "service_account_email" {
  type        = string
  description = "Runtime service account (one per unit)."
}

variable "container_port" {
  type        = number
  description = "Port the main container listens on."
}

variable "env" {
  type        = map(string)
  description = "Plain environment variables. No secret values."
  default     = {}
}

variable "secret_env" {
  type = map(object({
    secret  = string
    version = string
  }))
  description = "Environment variables read from Secret Manager (secret id and version). Values are added outside the IaC tool."
  default     = {}
}

variable "cpu" {
  type        = string
  description = "CPU limit, for example \"1\"."
}

variable "memory" {
  type        = string
  description = "Memory limit, for example \"512Mi\"."
}

variable "max_instances" {
  type        = number
  description = "Upper bound on instances. The minimum is always 0 (scale to zero); it is not an input."
}

variable "concurrency" {
  type        = number
  description = "Requests per instance."
  default     = 80
}

variable "timeout_seconds" {
  type        = number
  description = "Request timeout."
  default     = 60
}

variable "ingress" {
  type        = string
  description = "INGRESS_TRAFFIC_ALL, INGRESS_TRAFFIC_INTERNAL_ONLY, or INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER."

  validation {
    condition     = contains(["INGRESS_TRAFFIC_ALL", "INGRESS_TRAFFIC_INTERNAL_ONLY", "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"], var.ingress)
    error_message = "Unknown ingress value."
  }
}

variable "invokers" {
  type        = set(string)
  description = "IAM members granted roles/run.invoker. Use for admin-api (operators only) or for the API edge."
  default     = []
}

variable "allow_unauthenticated" {
  type        = bool
  description = "Grant roles/run.invoker to allUsers. Only for public read-api behind the API edge; never admin-api."
  default     = false
}

variable "deletion_protection" {
  type        = bool
  description = "Block deletion of the service."
  default     = true
}

variable "labels" {
  type        = map(string)
  description = "Labels."
  default     = {}
}

variable "metrics_sidecar" {
  type = object({
    image = string
    args  = optional(list(string), [])
    env   = optional(map(string), {})
  })
  description = "Managed Prometheus sidecar that scrapes localhost:<metrics port> and writes to Cloud Monitoring. Image, args, env (including scrape interval) are inputs. Null disables it."
  default     = null
}
