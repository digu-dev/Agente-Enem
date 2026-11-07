# Retrieval service

The versioned Azure AI Search index schema is `index/questoes-v1.json`. It
contains only text, filterable metadata, provenance and a 256-dimensional
vector. `origem` corresponds to `fonte` in the ingestion question schema.
The embedding deployment must support reduced dimensions and return exactly
256 values. Changing dimensions or other immutable field properties requires
a new index and reindexing.

To create or update the index, authenticate Azure CLI with an identity that has
Search Service Contributor on the search service, enable data-plane RBAC, and
run from this directory:

```sh
go run ./cmd/index --endpoint https://YOUR-SERVICE.search.windows.net
```

The command uses Azure CLI's current login, including managed identity when
`az login --identity` is used. It neither creates the Azure resource nor
uploads documents. Assign the retrieval identity Search Index Data Reader,
and assign Search Index Data Contributor only to the indexing identity.

The [Free service limits](https://learn.microsoft.com/en-us/azure/search/search-limits-quotas-capacity)
include a small storage quota and three indexes; confirm the current quota and
availability for the actual subscription and region in the Azure portal.
Keep semantic ranker disabled unless an explicitly approved paid experiment is
conducted and cleaned up the same day. RBAC for requests to Search is distinct
from enabling a managed identity on the Search service itself for outbound
connections; see [RBAC](https://learn.microsoft.com/en-us/azure/search/search-security-enable-roles)
and [managed identities](https://learn.microsoft.com/en-us/azure/search/search-how-to-managed-identities).

This schema and command do not yet implement document ingestion, capacity
guardrails, hybrid search, or evaluation. Do not populate the index until
holdout exclusion and capacity enforcement have been implemented.
