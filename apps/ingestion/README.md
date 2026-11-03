# ENEM ingestion CLI

The ingestion app is local-first. Input files belong in `data/raw` and are
never committed. `manifest.json` is the allowlist: only its entries are read.
The manifest is a JSON array with `nome`, `sha256`, `ano`, `caderno`, `tipo`
(`prova` or `gabarito`), `fonte`, `split` (`train`, `validation`, `index`, or
`holdout`), and `corpus_version`.

```sh
cd apps/ingestion
go run ./cmd/ingestion extract --input ../../data/raw --work-dir ../../data/work
go run ./cmd/ingestion validate --input ../../data/work/questoes.jsonl
go run ./cmd/ingestion label --input ../../data/work/questoes.jsonl --output ../../data/work/labeled.jsonl --dry-run
go run ./cmd/ingestion label --input ../../data/work/questoes.jsonl --output ../../data/work/labeled.jsonl
go run ./cmd/ingestion export --input ../../data/work/labeled.jsonl --output ../../data
```

Install Poppler (`pdftotext`, `pdftoppm`) to extract native PDF text and render
source pages. Missing tools are reported; image rasterization is not silently
skipped. A Document Intelligence fallback is opt-in with
`DOCUMENT_INTELLIGENCE_ENDPOINT` and uses Entra ID via `DefaultAzureCredential`.
Its conservative F0 guardrails are 2 pages and 4 MiB per request; the 500-page
monthly quota is tracked locally in the work directory and must be coordinated
if multiple machines share the same Azure resource. Official quota details:
[service limits](https://learn.microsoft.com/azure/ai-services/document-intelligence/concept-service-limits).
Document Intelligence uses the REST API because the official Go Azure SDK does
not expose this service. The REST API version is `2024-11-30`.

Azure Blob I/O is opt-in through `AZURE_STORAGE_ACCOUNT_URL`. Use
`extract --blob` to read raw inputs and `export --upload` to upload the output.
The containers must already exist and the identity needs Storage Blob Data
Reader on raw and Storage Blob Data Contributor on output. Container names
are set with `RAW_CONTAINER`, `PROCESSED_CONTAINER`, and `GOLDEN_CONTAINER`.
Authentication uses `DefaultAzureCredential` (Azure CLI locally or managed
identity in Azure), never account keys.

The pinned Azure Go modules are listed in `go.mod`; API references:
[azidentity](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity),
[azblob](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/storage/azblob),
and [JSON Schema](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v5).

The `label` command supports Azure OpenAI (`AZURE_OPENAI_ENDPOINT` and
`AZURE_OPENAI_DEPLOYMENT`) or any OpenAI-compatible endpoint
(`LLM_BASE_URL`, optional `LLM_MODEL`). Set `AZURE_OPENAI_ENDPOINT` to use
Entra ID; a local-compatible endpoint requires no API key. The source PDF text
is framed as untrusted data and the response is validated against the question
schema. `--max-tokens` limits the estimated total input tokens for the run;
`--dry-run` prints only aggregate counts and token estimates.

Extraction uses Poppler's layout-preserving text output and page rasterization.
Question parsing recognizes numbered items and A–E alternatives. ENEM booklets
("Questão N" headers) are read column by column, ignore the instruction items,
watermarks and footers, and accept alternatives written as `A texto` or
image-only (empty text). Scan-only
documents therefore need the optional Document Intelligence fallback. Outputs
are deterministic and replaced atomically. Export places holdout records only
under `enem-golden`, never `enem-processed`, and writes a deterministic
approximately 12.5% manual-review sample in `quality-review.jsonl` (within
10–15% for corpus sizes where an integer sample permits it).

Dependencies and indirect dependencies are pinned in `go.mod` and
`go.sum`. Validate with `go test -race ./...`, coverage checks, and
`govulncheck ./...`. Install the vulnerability scanner with
`go install golang.org/x/vuln/cmd/govulncheck@v1.9.0`.
