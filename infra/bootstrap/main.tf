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
  storage_use_azuread = true
}

resource "azurerm_resource_group" "state" {
  name     = var.resource_group_name
  location = var.location
  tags     = var.tags
}

resource "azurerm_storage_account" "state" {
  #checkov:skip=CKV_AZURE_33:Queue service is not used by the Terraform state backend.
  #checkov:skip=CKV_AZURE_59:GitHub-hosted runners require a public endpoint; Blob access uses Entra ID RBAC and shared keys are disabled.
  #checkov:skip=CKV2_AZURE_33:GitHub-hosted runners cannot use a private endpoint; use private runners before restricting this endpoint.
  #checkov:skip=CKV2_AZURE_1:Infrastructure encryption is enabled; customer-managed keys require a separate key lifecycle and bootstrap.
  name                              = var.storage_account_name
  resource_group_name               = azurerm_resource_group.state.name
  location                          = azurerm_resource_group.state.location
  account_tier                      = "Standard"
  account_replication_type          = "GRS"
  account_kind                      = "StorageV2"
  min_tls_version                   = "TLS1_2"
  https_traffic_only_enabled        = true
  public_network_access             = "Enabled"
  shared_access_key_enabled         = false
  allow_nested_items_to_be_public   = false
  infrastructure_encryption_enabled = true

  blob_properties {
    versioning_enabled = true

    delete_retention_policy {
      days = 7
    }

    container_delete_retention_policy {
      days = 7
    }
  }

  tags = var.tags
}

resource "azurerm_log_analytics_workspace" "state_audit" {
  name                = "law-${var.resource_group_name}"
  location            = azurerm_resource_group.state.location
  resource_group_name = azurerm_resource_group.state.name
  sku                 = "PerGB2018"
  retention_in_days   = 30
  tags                = var.tags
}

resource "azurerm_monitor_diagnostic_setting" "state_blob" {
  name                       = "state-blob-audit"
  target_resource_id         = "${azurerm_storage_account.state.id}/blobServices/default"
  log_analytics_workspace_id = azurerm_log_analytics_workspace.state_audit.id

  enabled_log {
    category = "StorageRead"
  }

  enabled_log {
    category = "StorageWrite"
  }

  enabled_log {
    category = "StorageDelete"
  }
}

resource "azurerm_storage_container" "state" {
  #checkov:skip=CKV2_AZURE_21:Blob read/write/delete diagnostics are enabled on this container's Blob service; Checkov does not associate the child-service target with this container.
  name                  = var.container_name
  storage_account_id    = azurerm_storage_account.state.id
  container_access_type = "private"
}
