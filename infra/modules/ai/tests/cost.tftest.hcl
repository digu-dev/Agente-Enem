mock_provider "azurerm" {}

run "free_defaults" {
  command = plan

  variables {
    resource_group_name        = "rg-test"
    location                   = "eastus2"
    tags                       = {}
    search_name                = "enemtestsearch"
    openai_name                = "enemtestopenai"
    document_intelligence_name = "enemtestdocument"
    retrieval_principal_id     = "00000000-0000-0000-0000-000000000001"
    ingestion_principal_id     = "00000000-0000-0000-0000-000000000002"
    api_principal_id           = "00000000-0000-0000-0000-000000000003"
  }

  assert {
    condition     = azurerm_search_service.this[0].sku == "free"
    error_message = "Search must default to the Free tier."
  }

  assert {
    condition     = var.search_public_network_access_enabled
    error_message = "Search Free requires a public endpoint."
  }

  assert {
    condition     = var.enable_openai == false && var.enable_document_intelligence == false
    error_message = "Chargeable AI services must be disabled by default."
  }

  assert {
    condition     = var.openai_capacity == 1
    error_message = "OpenAI capacity must default to its minimum."
  }
}

run "paid_search_requires_opt_in" {
  command = plan

  variables {
    resource_group_name        = "rg-test"
    location                   = "eastus2"
    tags                       = {}
    search_name                = "enemtestsearch"
    openai_name                = "enemtestopenai"
    document_intelligence_name = "enemtestdocument"
    retrieval_principal_id     = "00000000-0000-0000-0000-000000000001"
    ingestion_principal_id     = "00000000-0000-0000-0000-000000000002"
    api_principal_id           = "00000000-0000-0000-0000-000000000003"
    search_sku             = "basic"
  }

  expect_failures = [var.search_sku]
}

run "free_search_requires_public_network" {
  command = plan

  variables {
    resource_group_name                 = "rg-test"
    location                            = "eastus2"
    tags                                = {}
    search_name                         = "enemtestsearch"
    openai_name                         = "enemtestopenai"
    document_intelligence_name          = "enemtestdocument"
    retrieval_principal_id              = "00000000-0000-0000-0000-000000000001"
    ingestion_principal_id              = "00000000-0000-0000-0000-000000000002"
    api_principal_id                    = "00000000-0000-0000-0000-000000000003"
    search_public_network_access_enabled = false
  }

  expect_failures = [var.search_public_network_access_enabled]
}

run "paid_search_opt_in" {
  command = plan

  variables {
    resource_group_name        = "rg-test"
    location                   = "eastus2"
    tags                       = {}
    search_name                = "enemtestsearch"
    openai_name                = "enemtestopenai"
    document_intelligence_name = "enemtestdocument"
    retrieval_principal_id     = "00000000-0000-0000-0000-000000000001"
    ingestion_principal_id     = "00000000-0000-0000-0000-000000000002"
    api_principal_id           = "00000000-0000-0000-0000-000000000003"
    search_sku                 = "basic"
    enable_search_paid_tier    = true
  }

  assert {
    condition     = azurerm_search_service.this[0].sku == "basic"
    error_message = "Paid Search should be allowed only after explicit opt-in."
  }
}

run "paid_document_intelligence_requires_opt_in" {
  command = plan

  variables {
    resource_group_name        = "rg-test"
    location                   = "eastus2"
    tags                       = {}
    search_name                = "enemtestsearch"
    openai_name                = "enemtestopenai"
    document_intelligence_name = "enemtestdocument"
    retrieval_principal_id     = "00000000-0000-0000-0000-000000000001"
    ingestion_principal_id     = "00000000-0000-0000-0000-000000000002"
    api_principal_id           = "00000000-0000-0000-0000-000000000003"
    enable_document_intelligence = true
    document_intelligence_sku    = "S0"
  }

  expect_failures = [var.document_intelligence_sku]
}
