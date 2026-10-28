terraform {
  required_version = "= 1.17.1"

  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "= 5.11.0"
    }
  }
}

provider "azurerm" {
  features {}
}

locals {
  name_prefix = "enem-${var.environment}"
  tags = {
    project     = "enem-platform"
    environment = var.environment
    managed_by  = "terraform"
    cost_center = var.cost_center
  }
}

module "resource_group" {
  source = "../../modules/resource-group"

  name     = "${local.name_prefix}-${var.name_suffix}"
  location = var.location
  tags     = local.tags
}

module "monitoring" {
  source = "../../modules/monitoring"

  workspace_name            = "law-${local.name_prefix}-${var.name_suffix}"
  application_insights_name = "appi-${local.name_prefix}-${var.name_suffix}"
  resource_group_name       = module.resource_group.name
  location                  = var.location
  tags                      = local.tags
}

module "key_vault" {
  source = "../../modules/key-vault"

  name                = "kv-${local.name_prefix}-${var.name_suffix}"
  resource_group_name = module.resource_group.name
  location            = var.location
  tenant_id           = var.tenant_id
  tags                = local.tags
}

module "container_registry" {
  source = "../../modules/container-registry"

  name                = "enem${var.environment}${var.name_suffix}"
  resource_group_name = module.resource_group.name
  location            = var.location
  tags                = local.tags
}

module "container_app_environment" {
  source = "../../modules/container-app-environment"

  name                       = "cae-${local.name_prefix}-${var.name_suffix}"
  resource_group_name        = module.resource_group.name
  location                   = var.location
  log_analytics_workspace_id = module.monitoring.workspace_id
  tags                       = local.tags
}

module "api" {
  source = "../../modules/container-app"

  name                     = "api-${local.name_prefix}"
  resource_group_name      = module.resource_group.name
  location                 = var.location
  environment_id           = module.container_app_environment.id
  registry_id              = module.container_registry.id
  registry_login_server    = module.container_registry.login_server
  min_replicas             = var.min_replicas
  max_replicas             = var.max_replicas
  http_concurrent_requests = var.http_concurrent_requests
  tags                     = local.tags
}

module "retrieval" {
  source = "../../modules/container-app"

  name                     = "retrieval-${local.name_prefix}"
  resource_group_name      = module.resource_group.name
  location                 = var.location
  environment_id           = module.container_app_environment.id
  registry_id              = module.container_registry.id
  registry_login_server    = module.container_registry.login_server
  min_replicas             = var.min_replicas
  max_replicas             = var.max_replicas
  http_concurrent_requests = var.http_concurrent_requests
  tags                     = local.tags
}

module "web" {
  source = "../../modules/container-app"

  name                     = "web-${local.name_prefix}"
  resource_group_name      = module.resource_group.name
  location                 = var.location
  environment_id           = module.container_app_environment.id
  registry_id              = module.container_registry.id
  registry_login_server    = module.container_registry.login_server
  target_port              = 8080
  min_replicas             = var.min_replicas
  max_replicas             = var.max_replicas
  http_concurrent_requests = var.http_concurrent_requests
  tags                     = local.tags
}

module "deployment_access" {
  source = "../../modules/deployment-access"

  principal_id      = var.deploy_principal_object_id
  registry_id       = module.container_registry.id
  resource_group_id = module.resource_group.id
}

module "budget" {
  source = "../../modules/budget"

  name              = "budget-${local.name_prefix}"
  resource_group_id = module.resource_group.id
  amount            = var.monthly_budget_amount
  contact_email     = var.budget_contact_email
}
