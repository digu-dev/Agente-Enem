variable "principal_id" {
  description = "Microsoft Entra service principal object ID for the deploy identity."
  type        = string
}

variable "registry_id" {
  description = "ACR resource ID."
  type        = string
}

variable "resource_group_id" {
  description = "Resource group scope for Container Apps deployment access."
  type        = string
}
