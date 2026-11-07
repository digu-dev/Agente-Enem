output "resource_group_name" {
  value = module.resource_group.name
}

output "container_registry_name" {
  value = module.container_registry.name
}

output "container_registry_login_server" {
  value = module.container_registry.login_server
}

output "api_app_name" {
  value = module.api.name
}

output "retrieval_app_name" {
  value = module.retrieval.name
}

output "web_app_name" {
  value = module.web.name
}

output "api_fqdn" {
  value = module.api.fqdn
}

output "retrieval_fqdn" {
  value = module.retrieval.fqdn
}

output "web_fqdn" {
  value = module.web.fqdn
}

output "ingestion_identity_id" {
  value = module.ingestion_identity.id
}

output "ingestion_identity_principal_id" {
  value = module.ingestion_identity.principal_id
}

output "search_endpoint" {
  value = module.ai.search_endpoint
}

output "search_id" {
  value = module.ai.search_id
}

output "openai_id" {
  value = module.ai.openai_id
}

output "openai_endpoint" {
  value = module.ai.openai_endpoint
}

output "openai_chat_deployment" {
  value = module.ai.openai_chat_deployment
}

output "openai_embedding_deployment" {
  value = module.ai.openai_embedding_deployment
}

output "document_intelligence_endpoint" {
  value = module.ai.document_intelligence_endpoint
}

output "document_intelligence_id" {
  value = module.ai.document_intelligence_id
}

output "key_vault_uri" {
  value = module.key_vault.vault_uri
}
