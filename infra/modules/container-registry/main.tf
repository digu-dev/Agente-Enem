resource "azurerm_container_registry" "this" {
  #checkov:skip=CKV_AZURE_165:Dev is single-region and uses the cost-controlled Basic SKU.
  #checkov:skip=CKV_AZURE_139:GitHub-hosted runners need the public endpoint; pushes remain Entra-authenticated and the admin user is disabled.
  #checkov:skip=CKV_AZURE_233:Zone redundancy requires a higher ACR SKU than this dev environment uses.
  #checkov:skip=CKV_AZURE_167:ACR retention policy is Premium-only; images use commit-SHA tags and cleanup is a follow-up.
  #checkov:skip=CKV_AZURE_163:Registry vulnerability scanning requires an additional Defender plan; add before production.
  #checkov:skip=CKV_AZURE_237:Dedicated data endpoints are Premium-only and not needed for this single-region dev setup.
  #checkov:skip=CKV_AZURE_166:Image quarantine is Premium-only; add image verification before production.
  #checkov:skip=CKV_AZURE_164:Image signing/verification is not configured in this walking skeleton.
  name                          = var.name
  location                      = var.location
  resource_group_name           = var.resource_group_name
  sku                           = "Basic"
  admin_enabled                 = false
  public_network_access_enabled = true
  tags                          = var.tags
}
