output "search_id" {
  description = "Azure AI Search resource ID, or null when Search is disabled."
  value       = try(azurerm_search_service.this[0].id, null)
}

output "search_endpoint" {
  description = "Azure AI Search endpoint, or null when Search is disabled."
  value       = try("https://${azurerm_search_service.this[0].name}.search.windows.net", null)
}

output "openai_id" {
  description = "Azure OpenAI resource ID, or null when OpenAI is disabled."
  value       = try(azurerm_cognitive_account.openai[0].id, null)
}

output "openai_endpoint" {
  description = "Azure OpenAI endpoint, or null when OpenAI is disabled."
  value       = try(azurerm_cognitive_account.openai[0].endpoint, null)
}

output "openai_chat_deployment" {
  description = "Chat deployment name, or null when OpenAI is disabled."
  value       = try(azurerm_cognitive_deployment.chat[0].name, null)
}

output "openai_embedding_deployment" {
  description = "Embedding deployment name, or null when OpenAI is disabled."
  value       = try(azurerm_cognitive_deployment.embedding[0].name, null)
}

output "document_intelligence_id" {
  description = "Document Intelligence resource ID, or null when disabled."
  value       = try(azurerm_cognitive_account.document_intelligence[0].id, null)
}

output "document_intelligence_endpoint" {
  description = "Document Intelligence endpoint, or null when disabled."
  value       = try(azurerm_cognitive_account.document_intelligence[0].endpoint, null)
}
