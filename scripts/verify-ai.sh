#!/usr/bin/env bash
set -euo pipefail

terraform_dir="${1:-infra/envs/dev}"
outputs="$(terraform -chdir="$terraform_dir" output -json)"

while IFS= read -r resource_id; do
  [[ -n "$resource_id" ]] || continue

  state="$(az resource show --ids "$resource_id" --query 'properties.provisioningState || provisioningState' --output tsv)"
  if [[ "${state,,}" != "succeeded" ]]; then
    printf 'AI resource provisioning is not Succeeded (state=%s): %s\n' "$state" "$resource_id" >&2
    exit 1
  fi
  printf 'AI resource provisioning verified: %s\n' "$resource_id"
done < <(python3 -c 'import json,sys; outputs=json.load(sys.stdin); [print(outputs[k]["value"]) for k in ("search_id","openai_id","document_intelligence_id") if outputs.get(k,{}).get("value")]' <<<"$outputs")
