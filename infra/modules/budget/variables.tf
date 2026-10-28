variable "name" {
  description = "Budget name."
  type        = string
}

variable "resource_group_id" {
  description = "Resource group resource ID to monitor."
  type        = string
}

variable "amount" {
  description = "Monthly budget amount in the subscription billing currency."
  type        = number

  validation {
    condition     = var.amount > 0
    error_message = "The monthly budget amount must be greater than zero."
  }
}

variable "contact_email" {
  description = "Email address that receives the budget alert."
  type        = string
  sensitive   = true

  validation {
    condition     = can(regex("^[^[:space:]@]+@[^[:space:]@]+\\.[^[:space:]@]+$", var.contact_email))
    error_message = "The budget contact must be a valid email address."
  }
}
