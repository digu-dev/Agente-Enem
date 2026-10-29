variable "name" {
  description = "Container App name."
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

variable "environment_id" {
  description = "Container Apps Environment resource ID."
  type        = string
}

variable "registry_id" {
  description = "ACR resource ID."
  type        = string
}

variable "registry_login_server" {
  description = "ACR login server."
  type        = string
}

variable "target_port" {
  description = "HTTP port exposed by the app."
  type        = number
  default     = 8080
}

variable "min_replicas" {
  description = "Minimum replica count."
  type        = number
}

variable "max_replicas" {
  description = "Maximum replica count."
  type        = number
}

variable "http_concurrent_requests" {
  description = "Concurrent requests per replica before HTTP scaling."
  type        = number
}

variable "tags" {
  description = "Cost and ownership tags."
  type        = map(string)
}
