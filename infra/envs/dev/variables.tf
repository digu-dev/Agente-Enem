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

variable "enable_search" {
  description = "Create Azure AI Search."
  type        = bool
  default     = true
}

variable "search_sku" {
  description = "Azure AI Search tier; Free is the default."
  type        = string
  default     = "free"
  validation {
    condition     = contains(["free", "basic", "standard"], lower(var.search_sku))
    error_message = "Search tier must be free, basic, or standard."
  }
}

variable "enable_search_paid_tier" {
  description = "Explicit opt-in required before selecting a paid Search tier."
  type        = bool
  default     = false
}

variable "search_replica_count" {
  description = "Azure AI Search replica count."
  type        = number
  default     = 1
  validation {
    condition     = var.search_replica_count >= 1 && var.search_replica_count <= 12
    error_message = "Search replicas must be between 1 and 12."
  }
}

variable "search_partition_count" {
  description = "Azure AI Search partition count."
  type        = number
  default     = 1
  validation {
    condition     = var.search_partition_count >= 1 && var.search_partition_count <= 12
    error_message = "Search partitions must be between 1 and 12."
  }
}

variable "search_public_network_access_enabled" {
  description = "Search Free requires its public endpoint; paid tiers can disable it when private connectivity exists."
  type        = bool
  default     = true
}

variable "ai_public_network_access_enabled" {
  description = "Enable public networking for AI endpoints; requires Entra ID/RBAC and is disabled by default."
  type        = bool
  default     = false
}

variable "enable_openai" {
  description = "Create Azure OpenAI and its model deployments; disabled by default because it can incur charges."
  type        = bool
  default     = false
}

variable "chat_model_name" {
  description = "Exact chat model name available in the selected Azure region and subscription."
  type        = string
  default     = ""
}

variable "chat_model_version" {
  description = "Exact chat model version available in the selected Azure region and subscription."
  type        = string
  default     = ""
}

variable "embedding_model_name" {
  description = "Exact embedding model name available in the selected Azure region and subscription."
  type        = string
  default     = ""
}

variable "embedding_model_version" {
  description = "Exact embedding model version available in the selected Azure region and subscription."
  type        = string
  default     = ""
}

variable "openai_capacity" {
  description = "Low initial Azure OpenAI deployment capacity; confirm model-specific TPM quota in the portal."
  type        = number
  default     = 1
  validation {
    condition     = var.openai_capacity >= 1 && var.openai_capacity <= 10
    error_message = "OpenAI capacity must be between 1 and 10."
  }
}

variable "enable_document_intelligence" {
  description = "Create Document Intelligence; disabled by default."
  type        = bool
  default     = false
}

variable "document_intelligence_sku" {
  description = "Document Intelligence tier. F0 is the default; S0 needs explicit paid-tier opt-in."
  type        = string
  default     = "F0"
  validation {
    condition     = contains(["f0", "s0"], lower(var.document_intelligence_sku))
    error_message = "Document Intelligence tier must be F0 or S0."
  }
}

variable "enable_document_intelligence_paid_tier" {
  description = "Explicit opt-in required before selecting paid Document Intelligence S0."
  type        = bool
  default     = false
}
