resource "azurerm_key_vault" "this" {
  #checkov:skip=CKV2_AZURE_32:Public network access is disabled; add a private endpoint when applications start consuming vault data.
  name                          = var.name
  location                      = var.location
  resource_group_name           = var.resource_group_name
  tenant_id                     = var.tenant_id
  sku_name                      = "standard"
  rbac_authorization_enabled    = true
  purge_protection_enabled      = true
  public_network_access_enabled = false
  soft_delete_retention_days    = 7

  network_acls {
    bypass         = "None"
    default_action = "Deny"
  }

  tags = var.tags
}
