variable "name" {
  description = "Globally unique Key Vault name."
  type        = string
}

variable "location" {
  description = "Azure region."
  type        = string
}

variable "resource_group_name" {
  description = "Resource group name."
  type        = string
}

variable "tenant_id" {
  description = "Microsoft Entra tenant ID."
  type        = string
}

variable "tags" {
  description = "Cost and ownership tags."
  type        = map(string)
}
