#!/usr/bin/env bash
# Configura a conexão GitHub Actions -> Azure (OIDC) passo a passo.
# Rode no Azure Cloud Shell (Bash) ou em uma máquina com az, terraform e gh.
# Cada etapa mostra o que fará e pede confirmação antes de criar algo.
# Não cria client secrets. Os valores (IDs, não segredos) ficam em STATE_FILE.
#
# Uso:  scripts/azure-setup.sh            # executa as etapas 1..7 em ordem
#       scripts/azure-setup.sh 4          # executa somente a etapa 4
set -euo pipefail

STATE_FILE="${STATE_FILE:-$HOME/.enem-azure-setup.env}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO="${REPO:-digu-dev/Agente-Enem}"

# shellcheck disable=SC1090
[ -f "$STATE_FILE" ] && . "$STATE_FILE"

save() { # $1=nome $2=valor
  touch "$STATE_FILE" && chmod 600 "$STATE_FILE"
  { grep -v "^$1=" "$STATE_FILE" || true; } >"$STATE_FILE.tmp"
  printf '%s=%q\n' "$1" "$2" >>"$STATE_FILE.tmp"
  mv "$STATE_FILE.tmp" "$STATE_FILE"
  chmod 600 "$STATE_FILE"
  printf -v "$1" '%s' "$2"
}

say() { printf '\n\033[1m== %s ==\033[0m\n' "$*"; }

confirm() {
  local answer
  read -r -p "$1 [s/N] " answer
  [[ "$answer" =~ ^[sSyY]$ ]] || { echo "Etapa ignorada."; return 1; }
}

ask() { # $1=variável $2=pergunta $3=padrão
  local current="${!1:-${3:-}}" answer
  read -r -p "$2 [$current]: " answer
  save "$1" "${answer:-$current}"
}

need() {
  command -v "$1" >/dev/null || { echo "Falta a ferramenta: $1" >&2; exit 1; }
}

role() { # $1=principal (client id) $2=role $3=escopo
  az role assignment create --assignee "$1" --role "$2" --scope "$3" \
    --output none || echo "Aviso: não foi possível atribuir '$2' (talvez já exista)."
}

step1() {
  say "Etapa 1/7: verificar conta e registrar providers"
  need az; need terraform
  az account show --output table || { echo "Faça login: az login"; exit 1; }
  ask SUBSCRIPTION_ID "ID da subscription de dev" "$(az account show --query id -o tsv)"
  az account set --subscription "$SUBSCRIPTION_ID"
  save TENANT_ID "$(az account show --query tenantId -o tsv)"
  ask LOCATION "Região Azure" "eastus2"
  ask SUFFIX "Sufixo único (5-8 minúsculos/dígitos)" "$(LC_ALL=C tr -dc 'a-z0-9' </dev/urandom | head -c 6)"
  [[ "$SUFFIX" =~ ^[a-z0-9]{5,8}$ ]] || { echo "Sufixo inválido." >&2; exit 1; }
  save STATE_RG "rg-enem-tfstate"
  save STATE_SA "enemtfstate$SUFFIX"
  ask BUDGET_AMOUNT "Alerta de orçamento mensal (número)" "100"
  ask BUDGET_EMAIL "E-mail do alerta de orçamento" ""
  echo "Subscription: $SUBSCRIPTION_ID | Tenant: $TENANT_ID | Storage: $STATE_SA"
  confirm "Registrar os resource providers necessários na subscription?" || return 0
  for ns in Microsoft.App Microsoft.ContainerRegistry Microsoft.OperationalInsights \
    Microsoft.Insights Microsoft.KeyVault Microsoft.Consumption \
    Microsoft.Storage Microsoft.ManagedIdentity; do
    az provider register --namespace "$ns" --output none
    echo "registrado: $ns"
  done
}

step2() {
  say "Etapa 2/7: criar 3 identidades (App Registrations) SEM secrets"
  echo "Serão criadas: enem-gha-plan, enem-gha-apply, enem-gha-deploy."
  confirm "Criar as identidades no Microsoft Entra?" || return 0
  local name app_id upper
  for name in plan apply deploy; do
    app_id="$(az ad app list --display-name "enem-gha-$name" --query '[0].appId' -o tsv)"
    if [ -z "$app_id" ]; then
      app_id="$(az ad app create --display-name "enem-gha-$name" --query appId -o tsv)"
    fi
    az ad sp show --id "$app_id" --output none 2>/dev/null ||
      az ad sp create --id "$app_id" --output none
    upper="$(printf '%s' "$name" | tr '[:lower:]' '[:upper:]')"
    save "${upper}_CLIENT_ID" "$app_id"
    echo "$name client id: $app_id"
  done
  save DEPLOY_OBJECT_ID "$(az ad sp show --id "$DEPLOY_CLIENT_ID" --query id -o tsv)"
}

step3() {
  say "Etapa 3/7: federação OIDC (GitHub -> Azure)"
  # O GitHub pode emitir subjects imutáveis (com IDs de dono e repositório).
  local info prefix
  info="$(curl -fsSL "https://api.github.com/repos/$REPO")" || info=""
  prefix="repo:$REPO"
  if [ -n "$info" ]; then
    prefix="repo:${REPO%%/*}@$(printf '%s' "$info" | jq -r .owner.id)/${REPO#*/}@$(printf '%s' "$info" | jq -r .id)"
  else
    echo "Não foi possível consultar os IDs do repositório; usando o subject por nome."
  fi
  echo "plan   -> $prefix:pull_request"
  echo "apply  -> $prefix:ref:refs/heads/main"
  echo "deploy -> $prefix:ref:refs/heads/main"
  confirm "Criar/atualizar as credenciais federadas?" || return 0
  fed() { # $1=client id $2=nome $3=subject
    local body existing
    body="$(printf \
      '{"name":"%s","issuer":"https://token.actions.githubusercontent.com","subject":"%s","audiences":["api://AzureADTokenExchange"]}' \
      "$2" "$3")"
    existing="$(az ad app federated-credential list --id "$1" --query "[?name=='$2'].name" -o tsv)"
    if [ -n "$existing" ]; then
      az ad app federated-credential update --id "$1" --federated-credential-id "$2" \
        --parameters "$body" --output none
    else
      az ad app federated-credential create --id "$1" --parameters "$body" --output none
    fi
  }
  fed "$PLAN_CLIENT_ID" gha-pr "$prefix:pull_request"
  fed "$APPLY_CLIENT_ID" gha-main "$prefix:ref:refs/heads/main"
  fed "$DEPLOY_CLIENT_ID" gha-main "$prefix:ref:refs/heads/main"
}

step4() {
  say "Etapa 4/7: bootstrap do state remoto (Resource Group + Storage Account)"
  echo "Cria $STATE_RG e $STATE_SA em $LOCATION (GRS, versionamento, sem chave compartilhada)."
  confirm "Rodar o terraform apply do bootstrap?" || return 0
  cd "$REPO_ROOT/infra/bootstrap"
  local vars=(-var="resource_group_name=$STATE_RG" -var="storage_account_name=$STATE_SA" -var="location=$LOCATION")
  # Sem o backend remoto ainda existente, as duas primeiras fases usam state local.
  [ -f backend.tf ] || mv backend.tf.off backend.tf
  mv backend.tf backend.tf.off
  trap 'mv -f backend.tf.off backend.tf 2>/dev/null || true' EXIT
  rm -rf .terraform
  terraform init

  # Retomada após perda do state local: importa o que já existe na Azure.
  local rg_id="/subscriptions/$SUBSCRIPTION_ID/resourceGroups/$STATE_RG"
  if az group show -n "$STATE_RG" --output none 2>/dev/null; then
    echo "Resource Group já existe; importando para o state local."
    terraform import "${vars[@]}" azurerm_resource_group.state "$rg_id"
    if az storage account show -n "$STATE_SA" -g "$STATE_RG" --output none 2>/dev/null; then
      terraform import "${vars[@]}" azurerm_storage_account.state \
        "$rg_id/providers/Microsoft.Storage/storageAccounts/$STATE_SA"
    fi
  fi
  terraform apply "${vars[@]}" -target=azurerm_resource_group.state -target=azurerm_storage_account.state

  local state_id me
  state_id="$(az storage account show -n "$STATE_SA" -g "$STATE_RG" --query id -o tsv)"
  me="$(az ad signed-in-user show --query id -o tsv)"
  echo "Concedendo 'Storage Blob Data Contributor' ao seu usuário na Storage Account..."
  az role assignment create --assignee-object-id "$me" --assignee-principal-type User \
    --role "Storage Blob Data Contributor" --scope "$state_id" --output none ||
    echo "Aviso: role talvez já exista."
  echo "Aguardando propagação do RBAC (90s)..."
  sleep 90
  terraform apply "${vars[@]}"
  mv backend.tf.off backend.tf
  terraform init -migrate-state -force-copy \
    -backend-config="resource_group_name=$STATE_RG" \
    -backend-config="storage_account_name=$STATE_SA" \
    -backend-config="container_name=tfstate" \
    -backend-config="key=bootstrap.terraform.tfstate" \
    -backend-config="use_azuread_auth=true"
  echo "Se algum comando falhou com 403, aguarde alguns minutos e rode: $0 4"
}

step5() {
  say "Etapa 5/7: permissões (RBAC) das identidades"
  local sub="/subscriptions/$SUBSCRIPTION_ID"
  local state_id container
  state_id="$(az storage account show -n "$STATE_SA" -g "$STATE_RG" --query id -o tsv)"
  container="$state_id/blobServices/default/containers/tfstate"
  cat <<EOF
plan   : Reader (subscription) + Storage Blob Data Contributor (container tfstate)
apply  : Contributor + Role Based Access Control Administrator (subscription)
         + Storage Blob Data Contributor (container tfstate)
deploy : sem roles aqui; o Terraform cria AcrPush e Container Apps Contributor.
Contributor em subscription é amplo: use somente em subscription de dev.
EOF
  confirm "Atribuir estas roles?" || return 0
  role "$PLAN_CLIENT_ID" "Reader" "$sub"
  role "$PLAN_CLIENT_ID" "Storage Blob Data Contributor" "$container"
  role "$APPLY_CLIENT_ID" "Contributor" "$sub"
  role "$APPLY_CLIENT_ID" "Role Based Access Control Administrator" "$sub"
  role "$APPLY_CLIENT_ID" "Storage Blob Data Contributor" "$container"
}

step6() {
  say "Etapa 6/7: variáveis do GitHub Actions em $REPO"
  need gh
  gh auth status >/dev/null 2>&1 || { echo "Autentique o gh: gh auth login"; return 1; }
  echo "São IDs de configuração, não segredos. Serão definidas 11 variáveis."
  confirm "Criar as variáveis no repositório?" || return 0
  local pair
  for pair in \
    "AZURE_TENANT_ID=$TENANT_ID" \
    "AZURE_SUBSCRIPTION_ID=$SUBSCRIPTION_ID" \
    "AZURE_PLAN_CLIENT_ID=$PLAN_CLIENT_ID" \
    "AZURE_APPLY_CLIENT_ID=$APPLY_CLIENT_ID" \
    "AZURE_DEPLOY_CLIENT_ID=$DEPLOY_CLIENT_ID" \
    "AZURE_DEPLOY_PRINCIPAL_OBJECT_ID=$DEPLOY_OBJECT_ID" \
    "TF_STATE_RESOURCE_GROUP=$STATE_RG" \
    "TF_STATE_STORAGE_ACCOUNT=$STATE_SA" \
    "RESOURCE_NAME_SUFFIX=$SUFFIX" \
    "MONTHLY_BUDGET_AMOUNT=$BUDGET_AMOUNT" \
    "BUDGET_CONTACT_EMAIL=$BUDGET_EMAIL"; do
    gh variable set "${pair%%=*}" --repo "$REPO" --body "${pair#*=}" ||
      echo "Falha em ${pair%%=*}: crie manualmente em Settings > Secrets and variables > Actions > Variables."
  done
}

step7() {
  say "Etapa 7/7: manual (precisa ser admin do repositório)"
  cat <<EOF
1. Settings > Code security: ative Dependabot alerts, security updates,
   Secret scanning e Push protection.
2. Settings > Branches: proteja a main (PR obrigatório + checks do workflow).
3. Abra um PR interno e confira o comentário com o terraform plan.
4. Faça merge na main: o workflow aplica o Terraform e implanta os 3 serviços.
EOF
}

main() {
  local steps=("$@") n
  [ "${#steps[@]}" -gt 0 ] || steps=(1 2 3 4 5 6 7)
  for n in "${steps[@]}"; do
    [[ "$n" =~ ^[1-7]$ ]] || { echo "Etapa inválida: $n" >&2; exit 1; }
    [ "$n" = 1 ] || [ -n "${SUBSCRIPTION_ID:-}" ] ||
      { echo "Rode a etapa 1 primeiro." >&2; exit 1; }
    "step$n"
  done
  say "Concluído"
}

main "$@"
