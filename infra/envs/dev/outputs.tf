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
