module "identity" {
  source = "../managed-identity"

  name                = "${var.name}-identity"
  location            = var.location
  resource_group_name = var.resource_group_name
  tags                = var.tags
}

resource "azurerm_role_assignment" "registry_pull" {
  name                 = uuidv5("url", "${var.registry_id}:${module.identity.principal_id}:AcrPull")
  scope                = var.registry_id
  role_definition_name = "AcrPull"
  principal_id         = module.identity.principal_id
  principal_type       = "ServicePrincipal"
}

resource "azurerm_container_app" "this" {
  name                         = var.name
  resource_group_name          = var.resource_group_name
  container_app_environment_id = var.environment_id
  revision_mode                = "Single"

  identity {
    type         = "UserAssigned"
    identity_ids = [module.identity.id]
  }

  registry {
    server   = var.registry_login_server
    identity = module.identity.id
  }

  ingress {
    external_enabled           = true
    allow_insecure_connections = false
    target_port                = var.target_port
    transport                  = "http"

    traffic_weight {
      percentage      = 100
      latest_revision = true
    }
  }

  template {
    min_replicas = var.min_replicas
    max_replicas = var.max_replicas

    container {
      name   = var.name
      image  = "mcr.microsoft.com/azuredocs/containerapps-helloworld:latest"
      cpu    = 0.25
      memory = "0.5Gi"

      env {
        name  = "PORT"
        value = tostring(var.target_port)
      }
    }

    http_scale_rule {
      name                = "http-requests"
      concurrent_requests = var.http_concurrent_requests
    }
  }

  tags = var.tags

  lifecycle {
    ignore_changes = [template[0].container[0].image]
  }

  depends_on = [azurerm_role_assignment.registry_pull]
}
