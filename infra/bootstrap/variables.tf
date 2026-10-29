variable "resource_group_name" {
  description = "Resource group for the remote Terraform state account."
  type        = string
}

variable "storage_account_name" {
  description = "Globally unique 3-24 character lowercase storage account name."
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9]{3,24}$", var.storage_account_name))
    error_message = "Storage account names must be 3-24 lowercase letters or digits."
  }
}

variable "container_name" {
  description = "Private Blob container holding Terraform state."
  type        = string
  default     = "tfstate"
}

variable "location" {
  description = "Azure region."
  type        = string
  default     = "eastus2"
}

variable "tags" {
  description = "Cost and ownership tags."
  type        = map(string)
  default = {
    project     = "enem-platform"
    environment = "shared"
    managed_by  = "terraform"
  }
}
