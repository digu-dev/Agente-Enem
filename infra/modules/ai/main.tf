resource "azurerm_search_service" "this" {
  count = var.enable_search ? 1 : 0

  name                          = var.search_name
  resource_group_name           = var.resource_group_name
  location                      = var.location
  sku                           = var.search_sku
  replica_count                 = var.search_replica_count
  partition_count               = var.search_partition_count
  local_authentication_enabled  = false
  public_network_access_enabled = var.search_public_network_access_enabled
  tags                          = var.tags

  timeouts {
    create = "60m"
    update = "60m"
    delete = "60m"
  }
}

resource "azurerm_role_assignment" "retrieval_search_reader" {
  count = var.enable_search ? 1 : 0

  name                 = uuidv5("url", "${azurerm_search_service.this[0].id}:${var.retrieval_principal_id}:SearchIndexDataReader")
  scope                = azurerm_search_service.this[0].id
  role_definition_name = "Search Index Data Reader"
  principal_id         = var.retrieval_principal_id
  principal_type       = "ServicePrincipal"
}

resource "azurerm_role_assignment" "ingestion_search_writer" {
  count = var.enable_search ? 1 : 0

  name                 = uuidv5("url", "${azurerm_search_service.this[0].id}:${var.ingestion_principal_id}:SearchIndexDataContributor")
  scope                = azurerm_search_service.this[0].id
  role_definition_name = "Search Index Data Contributor"
  principal_id         = var.ingestion_principal_id
  principal_type       = "ServicePrincipal"
}

resource "azurerm_cognitive_account" "openai" {
  count = var.enable_openai ? 1 : 0

  name                          = var.openai_name
  resource_group_name           = var.resource_group_name
  location                      = var.location
  kind                          = "OpenAI"
  sku_name                      = "S0"
  custom_subdomain_name         = var.openai_name
  local_auth_enabled            = false
  public_network_access_enabled = var.cognitive_public_network_access_enabled
  tags                          = var.tags

  lifecycle {
    precondition {
      condition     = length(trimspace(var.chat_model_name)) > 0 && length(trimspace(var.chat_model_version)) > 0
      error_message = "Set the chat model name and version from the Azure portal before enabling OpenAI."
    }
    precondition {
      condition     = length(trimspace(var.embedding_model_name)) > 0 && length(trimspace(var.embedding_model_version)) > 0
      error_message = "Set the embedding model name and version from the Azure portal before enabling OpenAI."
    }
  }

  timeouts {
    create = "60m"
    update = "60m"
    delete = "60m"
  }
}

resource "azurerm_cognitive_deployment" "chat" {
  count = var.enable_openai ? 1 : 0

  name                 = var.chat_deployment_name
  cognitive_account_id = azurerm_cognitive_account.openai[0].id

  model {
    format  = "OpenAI"
    name    = var.chat_model_name
    version = var.chat_model_version
  }

  sku {
    name     = "Standard"
    capacity = var.openai_capacity
  }

  timeouts {
    create = "60m"
    update = "60m"
    delete = "60m"
  }
}

resource "azurerm_cognitive_deployment" "embedding" {
  count = var.enable_openai ? 1 : 0

  name                 = var.embedding_deployment_name
  cognitive_account_id = azurerm_cognitive_account.openai[0].id

  model {
    format  = "OpenAI"
    name    = var.embedding_model_name
    version = var.embedding_model_version
  }

  sku {
    name     = "Standard"
    capacity = var.openai_capacity
  }

  timeouts {
    create = "60m"
    update = "60m"
    delete = "60m"
  }
}

resource "azurerm_role_assignment" "api_openai_user" {
  count = var.enable_openai ? 1 : 0

  name                 = uuidv5("url", "${azurerm_cognitive_account.openai[0].id}:${var.api_principal_id}:CognitiveServicesOpenAIUser")
  scope                = azurerm_cognitive_account.openai[0].id
  role_definition_name = "Cognitive Services OpenAI User"
  principal_id         = var.api_principal_id
  principal_type       = "ServicePrincipal"
}

resource "azurerm_role_assignment" "ingestion_openai_user" {
  count = var.enable_openai ? 1 : 0

  name                 = uuidv5("url", "${azurerm_cognitive_account.openai[0].id}:${var.ingestion_principal_id}:CognitiveServicesOpenAIUser")
  scope                = azurerm_cognitive_account.openai[0].id
  role_definition_name = "Cognitive Services OpenAI User"
  principal_id         = var.ingestion_principal_id
  principal_type       = "ServicePrincipal"
}

resource "azurerm_cognitive_account" "document_intelligence" {
  count = var.enable_document_intelligence ? 1 : 0

  name                          = var.document_intelligence_name
  resource_group_name           = var.resource_group_name
  location                      = var.location
  kind                          = "FormRecognizer"
  sku_name                      = var.document_intelligence_sku
  custom_subdomain_name         = var.document_intelligence_name
  local_auth_enabled            = false
  public_network_access_enabled = var.cognitive_public_network_access_enabled
  tags                          = var.tags

  lifecycle {
    precondition {
      condition = (
        lower(var.document_intelligence_sku) == "f0" ||
        var.enable_document_intelligence_paid_tier
      )
      error_message = "Paid Document Intelligence tiers require enable_document_intelligence_paid_tier=true."
    }
  }

  timeouts {
    create = "60m"
    update = "60m"
    delete = "60m"
  }
}

resource "azurerm_role_assignment" "ingestion_document_intelligence_user" {
  count = var.enable_document_intelligence ? 1 : 0

  name                 = uuidv5("url", "${azurerm_cognitive_account.document_intelligence[0].id}:${var.ingestion_principal_id}:CognitiveServicesUser")
  scope                = azurerm_cognitive_account.document_intelligence[0].id
  role_definition_name = "Cognitive Services User"
  principal_id         = var.ingestion_principal_id
  principal_type       = "ServicePrincipal"
}
