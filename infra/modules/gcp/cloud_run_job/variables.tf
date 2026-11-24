variable "project" {
  type        = string
  description = "Project that owns the job."
}

variable "region" {
  type        = string
  description = "Cloud Run region."
}

variable "name" {
  type        = string
  description = "Job name. One job per unit."
}

variable "image" {
  type        = string
  description = "Full image reference, by digest. The registry lives outside this module."
}

variable "service_account_email" {
  type        = string
  description = "Runtime service account (one per unit)."
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
  description = "Environment variables read from Secret Manager."
  default     = {}
}

variable "otlp_endpoint" {
  type        = string
  description = "Value for OTEL_EXPORTER_OTLP_ENDPOINT (jobs push metrics over OTLP before exit). Null leaves it unset."
  default     = null
}

variable "cpu" {
  type        = string
  description = "CPU limit."
}

variable "memory" {
  type        = string
  description = "Memory limit."
}

variable "timeout_seconds" {
  type        = number
  description = "Task timeout."
}

variable "max_retries" {
  type        = number
  description = "Retries per task."
  default     = 0
}

variable "deletion_protection" {
  type        = bool
  description = "Block deletion of the job."
  default     = true
}

variable "labels" {
  type        = map(string)
  description = "Labels."
  default     = {}
}

variable "schedule" {
  type = object({
    cron                  = string
    time_zone             = string
    service_account_email = string
  })
  description = "Cloud Scheduler trigger. Null means manual runs only. Which jobs run on a schedule, and when, is an open question: no default."
  default     = null
}
