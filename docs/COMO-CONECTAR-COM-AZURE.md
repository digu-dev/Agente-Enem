# Como conectar o projeto à Azure (passo a passo)

Este guia leva o repositório de "código pronto" a "push na `main` implanta os 3
serviços em dev". Detalhes e justificativas estão em
[FASE-0-WALKING-SKELETON.md](FASE-0-WALKING-SKELETON.md).

> Nada aqui é executado automaticamente. Cada comando cria recursos ou
> permissões reais na sua conta Azure: leia antes de rodar. Nenhum client secret
> é criado; o GitHub autentica por OIDC.

## Atalho: script interativo

`scripts/azure-setup.sh` executa os passos 0 a 7 abaixo, pedindo confirmação
antes de cada etapa que cria algo (ideal no Azure Cloud Shell, já logado):

```bash
git clone https://github.com/digu-dev/Agente-Enem.git && cd Agente-Enem
scripts/azure-setup.sh        # todas as etapas
scripts/azure-setup.sh 4      # apenas uma etapa (retomada após erro)
```

Os valores ficam em `~/.enem-azure-setup.env` (permissão 600, sem secrets).
Os passos manuais abaixo continuam valendo como referência.

## 0. Pré-requisitos

- Conta Azure com uma **subscription ativa** (a conta gratuita serve para dev) e
  permissão de `Owner` (ou `Contributor` + `User Access Administrator`) nela.
- Permissão no Microsoft Entra para criar App Registrations.
- Admin do repositório `digu-dev/Agente-Enem` no GitHub.
- Ferramentas locais: Azure CLI (`az`), Terraform `1.17.1` e `gh`.

```bash
az login
az account list --output table
export SUBSCRIPTION_ID="<id-da-subscription>"
az account set --subscription "$SUBSCRIPTION_ID"
export TENANT_ID="$(az account show --query tenantId --output tsv)"
```

Registre os providers necessários (uma vez por subscription):

```bash
for ns in Microsoft.App Microsoft.ContainerRegistry Microsoft.OperationalInsights \
          Microsoft.Insights Microsoft.KeyVault Microsoft.Consumption \
          Microsoft.Storage Microsoft.ManagedIdentity; do
  az provider register --namespace "$ns"
done
```

## 1. Escolha os valores

```bash
export REPO="digu-dev/Agente-Enem"
export LOCATION="eastus2"
export SUFFIX="x7k3p"                 # 5-8 caracteres minúsculos/dígitos, único
export STATE_RG="rg-enem-tfstate"
export STATE_SA="enemtfstate${SUFFIX}"  # 3-24 minúsculos/dígitos, global único
```

## 2. Crie as 3 identidades de automação (sem secrets)

```bash
for name in plan apply deploy; do
  app_id="$(az ad app create --display-name "enem-gha-$name" --query appId --output tsv)"
  az ad sp create --id "$app_id" >/dev/null
  echo "$name client id: $app_id"
done
```

Anote os três client IDs. O Object ID do service principal de deploy:

```bash
DEPLOY_CLIENT_ID="<client-id-deploy>"
az ad sp show --id "$DEPLOY_CLIENT_ID" --query id --output tsv
```

## 3. Federação OIDC (GitHub → Azure)

Plan: somente PRs internos. Apply e deploy: somente a branch `main`.

```bash
fed() { # $1=client id, $2=nome, $3=subject
  az ad app federated-credential create --id "$1" --parameters "{
    \"name\": \"$2\",
    \"issuer\": \"https://token.actions.githubusercontent.com\",
    \"subject\": \"$3\",
    \"audiences\": [\"api://AzureADTokenExchange\"]}"
}
# O GitHub pode usar subjects imutáveis (IDs): confira em Settings > Actions >
# OIDC ou pelo erro AADSTS700213 do pipeline. Para este repositório:
PREFIX="repo:digu-dev@207451038/Agente-Enem@1437710208"
fed "<client-id-plan>"   gha-pr   "$PREFIX:pull_request"
fed "<client-id-apply>"  gha-main "$PREFIX:ref:refs/heads/main"
fed "<client-id-deploy>" gha-main "$PREFIX:ref:refs/heads/main"
```

## 4. Bootstrap do state remoto

```bash
cd infra/bootstrap
terraform init -backend=false
terraform apply -target=azurerm_resource_group.state \
  -target=azurerm_storage_account.state \
  -var="resource_group_name=$STATE_RG" \
  -var="storage_account_name=$STATE_SA" \
  -var="location=$LOCATION"
```

Dê ao seu usuário acesso aos dados Blob (o Terraform usa Entra ID, sem chaves) e
crie o container:

```bash
STATE_ID="$(az storage account show -n "$STATE_SA" -g "$STATE_RG" --query id -o tsv)"
ME="$(az ad signed-in-user show --query id -o tsv)"
az role assignment create --assignee-object-id "$ME" --assignee-principal-type User \
  --role "Storage Blob Data Contributor" --scope "$STATE_ID"

terraform apply -var="resource_group_name=$STATE_RG" \
  -var="storage_account_name=$STATE_SA" -var="location=$LOCATION"

terraform init -migrate-state \
  -backend-config="resource_group_name=$STATE_RG" \
  -backend-config="storage_account_name=$STATE_SA" \
  -backend-config="container_name=tfstate" \
  -backend-config="key=bootstrap.terraform.tfstate" \
  -backend-config="use_azuread_auth=true"
```

(A role pode levar alguns minutos para propagar; repita o comando se falhar com
403.)

## 5. Permissões (RBAC) das identidades

```bash
SUB_SCOPE="/subscriptions/$SUBSCRIPTION_ID"
CONTAINER_SCOPE="$STATE_ID/blobServices/default/containers/tfstate"
role() { az role assignment create --assignee "$1" --role "$2" --scope "$3"; }

role "<client-id-plan>"  "Reader"                                 "$SUB_SCOPE"
role "<client-id-plan>"  "Storage Blob Data Contributor"          "$CONTAINER_SCOPE"
role "<client-id-apply>" "Contributor"                            "$SUB_SCOPE"
role "<client-id-apply>" "Role Based Access Control Administrator" "$SUB_SCOPE"
role "<client-id-apply>" "Storage Blob Data Contributor"          "$CONTAINER_SCOPE"
```

`AcrPush`/`Container Apps Contributor` (deploy) e `AcrPull` (apps) são criadas
pelo próprio Terraform. Nunca use `Owner` nas identidades do workflow.

## 6. Variáveis do GitHub Actions

```bash
gh variable set AZURE_TENANT_ID                 --repo "$REPO" --body "$TENANT_ID"
gh variable set AZURE_SUBSCRIPTION_ID           --repo "$REPO" --body "$SUBSCRIPTION_ID"
gh variable set AZURE_PLAN_CLIENT_ID            --repo "$REPO" --body "<client-id-plan>"
gh variable set AZURE_APPLY_CLIENT_ID           --repo "$REPO" --body "<client-id-apply>"
gh variable set AZURE_DEPLOY_CLIENT_ID          --repo "$REPO" --body "<client-id-deploy>"
gh variable set AZURE_DEPLOY_PRINCIPAL_OBJECT_ID --repo "$REPO" --body "<object-id-sp-deploy>"
gh variable set TF_STATE_RESOURCE_GROUP         --repo "$REPO" --body "$STATE_RG"
gh variable set TF_STATE_STORAGE_ACCOUNT        --repo "$REPO" --body "$STATE_SA"
gh variable set RESOURCE_NAME_SUFFIX            --repo "$REPO" --body "$SUFFIX"
gh variable set MONTHLY_BUDGET_AMOUNT           --repo "$REPO" --body "100"
gh variable set BUDGET_CONTACT_EMAIL            --repo "$REPO" --body "<seu-email>"
```

São identificadores, não segredos; mesmo assim não os imprima em logs públicos.

## 7. Proteções do repositório (admin)

Em `Settings → Code security`: habilite Dependabot alerts, Dependabot security
updates, Secret scanning e Push protection. Em `Settings → Branches`, proteja a
`main` exigindo PR e os checks do workflow.

## 8. Primeiro ciclo

1. Abra um PR a partir de uma branch do próprio repositório: confira os checks e
   o comentário com o resumo do `terraform plan`.
2. Faça o merge na `main`: o workflow aplica o Terraform, publica as imagens no
   ACR, atualiza os 3 Container Apps e roda o smoke test de `/health`.
3. Confira no portal: Resource Group de dev, 3 Container Apps saudáveis, ACR,
   Log Analytics/App Insights, Key Vault e o alerta de budget.

## 9. Solução de problemas

| Sintoma | Causa provável |
|---|---|
| `AADSTS700213` / `No matching federated identity` | Subject da credencial diferente do emitido (PR vs `main`, ou formato por nome vs por IDs). Rode `scripts/azure-setup.sh 3` |
| 403 ao inicializar o backend | Role `Storage Blob Data Contributor` ausente ou ainda propagando |
| `AuthorizationFailed` em role assignment | Falta `Role Based Access Control Administrator` na identidade apply |
| `MissingSubscriptionRegistration` | Provider do passo 0 não registrado |
| Nome de ACR/Storage/Key Vault já existe | Troque `RESOURCE_NAME_SUFFIX` / `STATE_SA` |

## 10. Ligar e desligar o ambiente (controle de custo)

Os recursos de dev só precisam existir enquanto você trabalha. O state, as
identidades OIDC, as roles e as variáveis do GitHub **não** são apagados, então
religar não repete nenhum passo manual.

- **Desligar:** GitHub → `Actions` → **Destroy dev environment** → `Run workflow`
  na branch `main` → digite `DESTROY`. Apaga os recursos de `infra/envs/dev`.
- **Ligar:** `Actions` → **Walking skeleton pipeline** → `Run workflow` na
  branch `main` (ou faça um push na `main`). Recria a infraestrutura, publica as
  imagens, faz o deploy e roda o smoke test (cerca de 10 a 15 minutos).

Pelo CLI: `gh workflow run destroy-dev.yml -f confirm=DESTROY` e
`gh workflow run pipeline.yml --ref main`.

Notas:
- Os dois workflows só rodam na `main` (a credencial OIDC de apply/deploy aceita
  somente essa branch) e compartilham a fila `azure-dev-terraform`.
- O Key Vault tem purge protection: ao desligar ele fica em soft delete por 7
  dias e, ao religar com o mesmo sufixo, o Terraform o recupera. O workspace do
  Log Analytics é apagado de forma permanente para liberar o nome.
- Logs, métricas e imagens do ACR são perdidos a cada desligamento.
- Com o ambiente desligado sobram só o state (centavos) e as identidades.

## 11. Custos e limpeza total

Os recursos de dev são de SKU básico, mas geram custo. O orçamento apenas
alerta. Para remover tudo de vez: rode o workflow de destroy, depois
`terraform destroy` em `infra/bootstrap` (state remoto) e apague as App
Registrations `enem-gha-*`.
