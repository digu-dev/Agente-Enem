variable "environment" {
  description = "Deployment environment."
  type        = string
  default     = "dev"
  validation {
    condition     = contains(["dev", "prod"], var.environment)
    error_message = "Environment must be dev or prod."
  }
}

variable "name_suffix" {
  description = "Unique lowercase alphanumeric suffix for globally named Azure resources."
  type        = string
  validation {
    condition     = can(regex("^[a-z0-9]{5,8}$", var.name_suffix))
    error_message = "Name suffix must contain 5-8 lowercase letters or digits."
  }
}

variable "location" {
  description = "Azure region."
  type        = string
  default     = "eastus2"
}

variable "tenant_id" {
  description = "Microsoft Entra tenant ID."
  type        = string
}

variable "cost_center" {
  description = "Cost center tag value."
  type        = string
  default     = "platform"
}

variable "min_replicas" {
  description = "Minimum replicas per Container App; zero enables scale-to-zero."
  type        = number
  default     = 0
  validation {
    condition     = var.min_replicas >= 0
    error_message = "Minimum replicas cannot be negative."
  }
}

variable "max_replicas" {
  description = "Maximum replicas per Container App."
  type        = number
  default     = 3
  validation {
    condition     = var.max_replicas >= var.min_replicas && var.max_replicas <= 30
    error_message = "Maximum replicas must be at least minimum replicas and no greater than 30."
  }
}

variable "http_concurrent_requests" {
  description = "Concurrent HTTP requests per replica before scaling."
  type        = number
  default     = 10
  validation {
    condition     = var.http_concurrent_requests >= 1
    error_message = "HTTP concurrency threshold must be at least one."
  }
}

variable "monthly_budget_amount" {
  description = "Monthly Azure budget in the subscription billing currency."
  type        = number
  validation {
    condition     = var.monthly_budget_amount > 0
    error_message = "Monthly budget amount must be greater than zero."
  }
}

variable "budget_contact_email" {
  description = "Recipient for budget alert email."
  type        = string
  sensitive   = true
  validation {
    condition     = can(regex("^[^[:space:]@]+@[^[:space:]@]+\\.[^[:space:]@]+$", var.budget_contact_email))
    error_message = "Budget contact must be a valid email address."
  }
}

variable "deploy_principal_object_id" {
  description = "Microsoft Entra service principal object ID for the deploy workflow identity."
  type        = string

  validation {
    condition     = can(regex("^[0-9a-fA-F-]{36}$", var.deploy_principal_object_id))
    error_message = "Deploy principal object ID must be a UUID."
  }
}
