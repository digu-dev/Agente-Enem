variable "resource_group_name" {
  description = "Resource group for the AI resources."
  type        = string
}

variable "location" {
  description = "Azure region; model and free-tier availability must be checked for this region."
  type        = string
}

variable "tags" {
  description = "Resource ownership and cost tags."
  type        = map(string)
}

variable "enable_search" {
  description = "Create Azure AI Search."
  type        = bool
  default     = true
}

variable "search_name" {
  description = "Globally unique lowercase Azure AI Search service name."
  type        = string
}

variable "search_sku" {
  description = "Azure AI Search tier. Paid tiers require explicit opt-in."
  type        = string
  default     = "free"

  validation {
    condition = (
      contains(["free", "basic", "standard"], lower(var.search_sku)) &&
      (lower(var.search_sku) == "free" || var.enable_search_paid_tier)
    )
    error_message = "search_sku must be free, basic, or standard; paid tiers require enable_search_paid_tier=true."
  }
}

variable "enable_search_paid_tier" {
  description = "Explicitly acknowledge the hourly cost of a paid Search tier."
  type        = bool
  default     = false
}

variable "search_replica_count" {
  description = "Search replicas; Free requires one replica."
  type        = number
  default     = 1
  validation {
    condition     = var.search_replica_count >= 1 && var.search_replica_count <= 12
    error_message = "Search replica count must be between 1 and 12."
  }
  validation {
    condition     = lower(var.search_sku) != "free" || var.search_replica_count == 1
    error_message = "Azure AI Search Free supports exactly one replica."
  }
}

variable "search_partition_count" {
  description = "Search partitions; Free requires one partition."
  type        = number
  default     = 1
  validation {
    condition     = var.search_partition_count >= 1 && var.search_partition_count <= 12
    error_message = "Search partition count must be between 1 and 12."
  }
  validation {
    condition     = lower(var.search_sku) != "free" || var.search_partition_count == 1
    error_message = "Azure AI Search Free supports exactly one partition."
  }
}

variable "search_public_network_access_enabled" {
  description = "Search Free requires its public endpoint; paid tiers can disable it when private connectivity exists."
  type        = bool
  default     = true
  validation {
    condition     = lower(var.search_sku) != "free" || var.search_public_network_access_enabled
    error_message = "Azure AI Search Free does not support disabling public network access."
  }
}

variable "retrieval_principal_id" {
  description = "Object ID of the retrieval managed identity (read-only Search access)."
  type        = string
}

variable "ingestion_principal_id" {
  description = "Object ID of the dedicated ingestion managed identity (Search write access)."
  type        = string
}

variable "api_principal_id" {
  description = "Object ID of the API managed identity (OpenAI inference access)."
  type        = string
}

variable "cognitive_public_network_access_enabled" {
  description = "Enable public endpoints for cognitive accounts; keep false until private connectivity is configured."
  type        = bool
  default     = false
}

variable "enable_openai" {
  description = "Create Azure OpenAI. Disabled by default because inference and deployment capacity can incur charges."
  type        = bool
  default     = false
}

variable "openai_name" {
  description = "Globally unique Azure OpenAI account name."
  type        = string
}

variable "chat_deployment_name" {
  description = "Deployment name for the selected chat model."
  type        = string
  default     = "enem-chat"
}

variable "chat_model_name" {
  description = "Chat model name available to the subscription in the selected region."
  type        = string
  default     = ""
}

variable "chat_model_version" {
  description = "Exact chat model version available to the subscription in the selected region."
  type        = string
  default     = ""
}

variable "embedding_deployment_name" {
  description = "Deployment name for the selected embedding model."
  type        = string
  default     = "enem-embedding"
}

variable "embedding_model_name" {
  description = "Embedding model name available to the subscription in the selected region."
  type        = string
  default     = ""
}

variable "embedding_model_version" {
  description = "Exact embedding model version available to the subscription in the selected region."
  type        = string
  default     = ""
}

variable "openai_capacity" {
  description = "Low initial Standard deployment capacity. Check model-specific quota in Azure before applying."
  type        = number
  default     = 1
  validation {
    condition     = var.openai_capacity >= 1 && var.openai_capacity <= 10
    error_message = "openai_capacity must be between 1 and 10; higher capacity requires an explicit design review."
  }
}

variable "enable_document_intelligence" {
  description = "Create Document Intelligence; disabled by default."
  type        = bool
  default     = false
}

variable "document_intelligence_name" {
  description = "Globally unique Document Intelligence account name."
  type        = string
}

variable "document_intelligence_sku" {
  description = "Document Intelligence tier. F0 is free; S0 requires explicit paid-tier opt-in."
  type        = string
  default     = "F0"
  validation {
    condition = (
      contains(["f0", "s0"], lower(var.document_intelligence_sku)) &&
      (lower(var.document_intelligence_sku) == "f0" || var.enable_document_intelligence_paid_tier)
    )
    error_message = "document_intelligence_sku must be F0 or S0; S0 requires enable_document_intelligence_paid_tier=true."
  }
}

variable "enable_document_intelligence_paid_tier" {
  description = "Explicitly acknowledge charges for Document Intelligence S0."
  type        = bool
  default     = false
}
