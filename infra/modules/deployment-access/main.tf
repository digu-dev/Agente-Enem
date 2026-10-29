resource "azurerm_role_assignment" "registry_push" {
  name                 = uuidv5("url", "${var.registry_id}:${var.principal_id}:AcrPush")
  scope                = var.registry_id
  role_definition_name = "AcrPush"
  principal_id         = var.principal_id
  principal_type       = "ServicePrincipal"
}

resource "azurerm_role_assignment" "container_apps_update" {
  name                 = uuidv5("url", "${var.resource_group_id}:${var.principal_id}:ContainerAppsContributor")
  scope                = var.resource_group_id
  role_definition_name = "Container Apps Contributor"
  principal_id         = var.principal_id
  principal_type       = "ServicePrincipal"
}
