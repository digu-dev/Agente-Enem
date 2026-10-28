variable "name" {
  description = "Globally unique ACR name containing only lowercase letters and digits."
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

variable "tags" {
  description = "Cost and ownership tags."
  type        = map(string)
}
