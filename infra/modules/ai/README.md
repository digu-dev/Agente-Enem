# Reusable AI resources module

This module provisions Azure AI Search, Azure OpenAI and Document Intelligence
in the caller's resource group and region. It pins the existing AzureRM
provider version (`5.11.0`); the environment root also pins the provider and
commits its dependency lock file.

## Cost and quota safeguards

- Search defaults to `free`, one replica and one partition. Selecting `basic`
  or `standard` requires `enable_search_paid_tier = true`. Free capacity is
  limited (currently documented as 50 MB and three indexes) and does not
  include semantic ranking; confirm current quotas and availability for the
  subscription and region in the Azure portal.
- The Free tier does not support disabling public network access. The Search
  endpoint therefore remains publicly reachable at the network layer, but
  local/key authentication is disabled and callers require Entra ID/RBAC.
  Paid tiers can set `search_public_network_access_enabled = false` when
  private connectivity is in place.
- OpenAI defaults to disabled. Azure OpenAI has no free account tier: model
  deployments can incur charges, and capacity `1` is only a low initial setting,
  not a spending cap. Set exact model names and versions only after confirming
  regional deployment availability and TPM quota in the portal.
- Document Intelligence defaults to disabled with SKU `F0`. `S0` requires
  `enable_document_intelligence_paid_tier = true`; confirm the current F0
  allowance and regional availability in the portal before enabling it.
- Search replicas and partitions above the Free limits are subject to tier
  limits and can increase hourly charges. `terraform destroy` removes the AI
  resources, but does not refund usage already incurred.
- Existing Key Vault, Container Apps, monitoring and registry resources are
  outside the AI module and may have separate charges.

Before applying, confirm the region, search service quota, model availability,
model-specific TPM quota, and DI F0 allowance in the Azure portal. Azure can
reject a deployment when quota or regional capacity is unavailable; Terraform
surfaces that provider error and does not silently substitute a model or SKU.
References: [Search free tier](https://learn.microsoft.com/en-us/azure/search/search-try-for-free),
[Search limits](https://learn.microsoft.com/en-us/azure/search/search-limits-quotas-capacity),
[Search networking](https://learn.microsoft.com/en-us/azure/search/service-configure-firewall),
[Search RBAC](https://learn.microsoft.com/en-us/azure/search/search-security-enable-roles),
[OpenAI model availability](https://learn.microsoft.com/en-us/azure/ai-services/openai/concepts/models),
[OpenAI quotas](https://learn.microsoft.com/en-us/azure/ai-services/openai/concepts/quotas), and
[Document Intelligence limits](https://learn.microsoft.com/en-us/azure/ai-services/document-intelligence/service-limits).

## Identity and networking

Local/key authentication is disabled for Search and cognitive accounts. The
retrieval managed identity receives only `Search Index Data Reader`; the
dedicated ingestion managed identity receives `Search Index Data Contributor`
and, when enabled, the OpenAI and Document Intelligence user roles. The API
identity receives only `Cognitive Services OpenAI User`. No storage role is
granted to Search, retrieval or ingestion here, and no identity is granted
access to `enem-golden`.

The module does not create API keys or write them to Terraform state or Key
Vault. Endpoints and resource IDs are non-secret Terraform outputs; the
environment separately outputs its Key Vault URI. Model credentials use Entra
ID rather than Key Vault API-key references.

Public network access for OpenAI and Document Intelligence defaults to
disabled. The current Container Apps environment has no VNet/private endpoints,
so those apps cannot reach these endpoints until private connectivity is added.
To permit temporary public connectivity, explicitly set
`ai_public_network_access_enabled = true`; local authentication remains
disabled and role assignments remain required. Do not enable this for
production without an approved network design.

## Verification

From `infra/modules/ai`, run:

```sh
terraform init -backend=false -input=false
terraform test
```

From `infra/envs/dev`, the repository CI runs `terraform fmt`, `validate`,
TFLint, Checkov, mocked cost-guard tests, and a PR-only plan. A real apply and
destroy require the configured Azure OIDC identity and remote state backend.
After apply, manually verify the Search service and any enabled cognitive
accounts/deployments in the portal. A successful ARM deployment alone does not
verify that a model can serve requests or that the app has network reachability.
