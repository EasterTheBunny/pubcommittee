variable "project" {
  type        = string
  description = "Project that owns the bucket."
}

variable "name" {
  type        = string
  description = "Globally unique bucket name."
}

variable "location" {
  type        = string
  description = "Bucket location (region or multi-region)."
}

variable "storage_class" {
  type        = string
  description = "Default storage class."
  default     = "STANDARD"
}

variable "versioning" {
  type        = bool
  description = "Keep noncurrent object versions."
  default     = false
}

variable "delete_after_days" {
  type        = number
  description = "Delete objects older than this many days. Null keeps objects."
  default     = null
}

variable "force_destroy" {
  type        = bool
  description = "Allow destroying a non-empty bucket."
  default     = false
}

variable "object_admins" {
  type        = set(string)
  description = "IAM members granted roles/storage.objectAdmin on this bucket (writers)."
  default     = []
}

variable "object_viewers" {
  type        = set(string)
  description = "IAM members granted roles/storage.objectViewer on this bucket (readers)."
  default     = []
}
