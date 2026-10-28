# Fase 0 — walking skeleton

Este documento descreve a infraestrutura e os serviços iniciais do repositório
`digu-dev/Agente-Enem`, as decisões tomadas e o que ainda precisa ser configurado
antes de um push na `main` poder implantar no Azure.

> Nenhuma conta, subscription, identidade, Resource Group ou outro recurso Azure
> foi criado nesta execução. Foram preparados apenas arquivos locais de código e
> documentação; nenhum `terraform apply` foi executado e nada foi enviado à `main`.

## Escopo implementado

- `apps/api`: API Spring Boot com `GET /health`, teste de integração e Dockerfile
  multi-stage.
- `apps/retrieval-svc`: serviço Go com `GET /health`, testes com `httptest` e
  Dockerfile multi-stage que gera binário estático em `scratch`.
- `apps/web`: página React/TypeScript, teste com Vitest e Testing Library,
  lint/build e Dockerfile multi-stage servido por Nginx sem usuário root.
- `infra/bootstrap`: Storage Account e container privados para state Terraform,
  com versionamento e retenção de blobs.
- `infra/modules` e `infra/envs/dev`: módulos reutilizáveis para Resource Group,
  Log Analytics, Application Insights, Key Vault com RBAC, ACR, Container Apps
  Environment, identidades gerenciadas, Container Apps, budget e escala HTTP.
- `.github/workflows/pipeline.yml`: validações em PR e `main`, resumo do plan no
  comentário do PR e apply/build/deploy/smoke test na `main`.
- `.github/dependabot.yml`: atualizações semanais para actions, Terraform,
  Go, Maven, npm e imagens Docker.

O walking skeleton implanta **web, api e retrieval-svc**. `apps/ingestion` foi
deixado fora do deploy conforme a decisão de limitar esta etapa a três apps; ele
pode ser conectado numa etapa posterior.

## Decisões de arquitetura e segurança

### Terraform e Azure

- O bootstrap usa Azure Blob Storage com replicação GRS, TLS, criptografia de
  infraestrutura, chave compartilhada desabilitada, acesso privado ao container,
  versionamento e retenção de exclusões por sete dias. Leituras, gravações e
  exclusões no Blob são enviadas a um workspace de auditoria.
- O backend `azurerm` usa Microsoft Entra ID/OIDC. O lease de Blob usado pelo
  backend fornece o lock do state; o versionamento permite recuperar versões
  anteriores do state.
- O endpoint do Storage Account permanece acessível pela rede pública para
  permitir runners hospedados pelo GitHub. A autorização é feita por identidade
  e RBAC, sem access key. Para maior isolamento, use runners privados e
  Private Endpoint antes de bloquear a rede pública.
- O ACR usa SKU Basic e desabilita o usuário administrador. Cada Container App
  recebe uma identidade gerenciada própria, com `AcrPull` no registry.
- O Key Vault usa RBAC, soft delete, purge protection e acesso público desabilitado.
  Nenhum segredo de aplicação é criado ou armazenado nesta fase; será necessário
  configurar Private Endpoint antes de consumir o vault de uma aplicação.
- O Container Apps Environment envia logs ao Log Analytics. Application
  Insights é provisionado, mas os serviços ainda não enviam telemetria para ele.
- Os apps recebem configuração pelo ambiente e são stateless. O mínimo de
  réplicas (padrão `0`), máximo (padrão `3`) e concorrência HTTP (padrão `10`)
  são variáveis Terraform.
- Os Container Apps começam com uma imagem pública de bootstrap. Após o apply,
  o workflow publica e implanta as imagens próprias; Terraform ignora mudanças
  apenas no campo da imagem para não desfazer os deploys subsequentes.

### OIDC, permissões e variáveis GitHub

Configure estes **Actions variables** no repositório; nenhum client secret é
necessário:

| Variable | Uso |
|---|---|
| `AZURE_TENANT_ID` | Tenant Microsoft Entra |
| `AZURE_SUBSCRIPTION_ID` | Subscription Azure |
| `AZURE_PLAN_CLIENT_ID` | Aplicação/identidade federada usada no plan de PR |
| `AZURE_APPLY_CLIENT_ID` | Aplicação/identidade federada usada apenas pelo Terraform apply |
| `AZURE_DEPLOY_CLIENT_ID` | Aplicação/identidade federada usada para publicar imagens e atualizar apps |
| `AZURE_DEPLOY_PRINCIPAL_OBJECT_ID` | Object ID do service principal deploy, usado pelo Terraform para criar seus grants RBAC |
| `TF_STATE_RESOURCE_GROUP` | Resource Group do Storage Account de state |
| `TF_STATE_STORAGE_ACCOUNT` | Nome do Storage Account de state |
| `RESOURCE_NAME_SUFFIX` | Sufixo único, 5–8 caracteres minúsculos ou dígitos |
| `MONTHLY_BUDGET_AMOUNT` | Limite de alerta mensal na moeda da subscription |
| `BUDGET_CONTACT_EMAIL` | Destinatário do alerta de budget |

O bootstrap usa também `resource_group_name`, `storage_account_name` e
`location`, informados no primeiro apply local. O nome da conta de Storage deve
ser globalmente único, com 3–24 caracteres minúsculos ou dígitos.

Crie credenciais federadas no Microsoft Entra para o issuer
`https://token.actions.githubusercontent.com`, audiência `api://AzureADTokenExchange`
e estes subjects:

- Plan de PR interno: `repo:digu-dev/Agente-Enem:pull_request`
- Apply e deploy na branch principal:
  `repo:digu-dev/Agente-Enem:ref:refs/heads/main`

O plan de PR não roda em forks: conceder OIDC a código de terceiros poderia
expor acesso à subscription e ao state. Use três identidades federadas distintas:
plan, apply e deploy. O GitHub guarda apenas client IDs como variables; OIDC
emite tokens curtos e não há client secret.

### Permissões Azure necessárias

| Identidade | Escopo | Role / finalidade |
|---|---|---|
| Operador de bootstrap | Subscription | `Contributor` para criar o Resource Group e Storage Account de state. Um administrador também deve fazer as atribuições RBAC; `Contributor` não pode atribuir roles. |
| Operador de bootstrap | Storage Account de state | `Storage Blob Data Contributor` para criar o container usando Entra ID com shared key desabilitada. |
| Plan (`AZURE_PLAN_CLIENT_ID`) | Subscription de dev | `Reader` para ler recursos durante refresh/plan. |
| Plan (`AZURE_PLAN_CLIENT_ID`) | Container `tfstate` | `Storage Blob Data Contributor` para ler/lockar o blob do state. |
| Apply (`AZURE_APPLY_CLIENT_ID`) | Subscription de dev | `Contributor` porque este root Terraform cria o Resource Group e seus recursos. |
| Apply (`AZURE_APPLY_CLIENT_ID`) | Subscription de dev | `Role Based Access Control Administrator` (ou `User Access Administrator`) para criar os grants `AcrPull` das identidades gerenciadas. |
| Apply (`AZURE_APPLY_CLIENT_ID`) | Container `tfstate` | `Storage Blob Data Contributor` para state e lock. |
| Deploy (`AZURE_DEPLOY_CLIENT_ID`) | ACR `enem<environment><suffix>` | `AcrPush` para fazer push das três imagens; grant criado pelo módulo Terraform `deployment-access`. |
| Deploy (`AZURE_DEPLOY_CLIENT_ID`) | Resource Group de dev | `Container Apps Contributor` para atualizar e consultar as apps; grant criado pelo módulo Terraform `deployment-access`. |
| Identidade gerenciada de cada Container App | ACR | `AcrPull`, criado pelo módulo Terraform da app. |

**Atenção ao escopo:** as roles de `Contributor` e `Role Based Access Control
Administrator` em subscription são o conjunto built-in mais simples para o
workflow Terraform atual, mas são mais amplas que o ideal. Não dê `Owner` ao
workflow. Para endurecer, separe o bootstrap do ambiente: um operador provisiona
o Resource Group/ACR, a identidade Terraform recebe `Contributor` apenas no
Resource Group e `Role Based Access Control Administrator` apenas no ACR, com
condição para permitir somente atribuição de `AcrPull`. O root atual ainda
gerencia o Resource Group; portanto, antes de remover permissões de subscription,
valide as operações de leitura/tag/update e role-assignment com um plan/apply
sob o escopo reduzido ou extraia o bootstrap para um state separado.

O plan precisa de `Reader` porque o Terraform atualiza o state lendo os recursos.
O `Storage Blob Data Contributor` não é substituído por `Contributor`: são
permissões de dados do Storage, separadas do plano de controle do Azure.

### Passo a passo de configuração

1. **Escolha a subscription e a região.** Confirme uma subscription habilitada
   para Container Apps, ACR Basic, Log Analytics, Application Insights, Key Vault
   e budget. Defina `location`, sufixo globalmente único, valor/moeda do budget e
   e-mail de alertas. O budget é um alerta, não um limite de gasto.
2. **Crie as identidades de automação no Microsoft Entra.** Em
   `entra.microsoft.com → App registrations → New registration`, crie três
   aplicações/service principals distintos, por exemplo `enem-gha-plan`,
   `enem-gha-apply` e `enem-gha-deploy`. Copie os client IDs, tenant ID e
   subscription ID; não crie client secrets. Em `Enterprise applications`, abra
   o service principal deploy e copie seu **Object ID** para
   `AZURE_DEPLOY_PRINCIPAL_OBJECT_ID`.
3. **Configure federação OIDC.** Em cada App Registration, abra
   `Certificates & secrets → Federated credentials → Add credential → GitHub
   Actions deploying Azure resources`. Configure organization `digu-dev`,
   repository `Agente-Enem`, issuer
   `https://token.actions.githubusercontent.com` e audience
   `api://AzureADTokenExchange`. Escolha entidade `Pull request` apenas para
   `enem-gha-plan`; escolha `Branch` / `main` para apply e deploy. Não crie
   client secrets nem credenciais de longa duração.
4. **Prepare o state.** Com uma identidade administrativa, rode o bootstrap
   descrito abaixo uma vez. O primeiro apply cria somente Resource Group e
   Storage Account. Depois um administrador concede `Storage Blob Data
   Contributor` ao operador no Storage Account; o segundo apply cria o container
   usando Entra ID. Migre então o state local para o Blob e conceda a mesma role
   às identidades plan/apply somente no container `tfstate`.
5. **Atribua RBAC do plan/apply.** No portal Azure, abra
   `Subscriptions → <dev> → Access control (IAM) → Add role assignment`.
   Atribua `Reader` à identidade plan e `Contributor` mais
   `Role Based Access Control Administrator` à identidade apply. Mantenha
   `Storage Blob Data Contributor` das duas identidades somente no container
   `tfstate`. O Terraform criará `AcrPush` e `Container Apps Contributor` para a
   identidade deploy e `AcrPull` para cada identidade de runtime.
6. **Configure GitHub Actions variables.** Em
   `Settings → Secrets and variables → Actions → Variables`, adicione todos os
   nomes da tabela acima. `RESOURCE_NAME_SUFFIX` deve ter 5–8 caracteres
   minúsculos/dígitos. `MONTHLY_BUDGET_AMOUNT` deve ser número positivo na moeda
   da subscription; `BUDGET_CONTACT_EMAIL` deve ser um e-mail válido.
7. **Habilite proteções de repositório.** Em
   `Settings → Code security and analysis`, habilite Dependabot alerts, security
   updates, secret scanning e push protection. `.github/dependabot.yml` já
   define os ecossistemas e a frequência de atualização. A ativação das opções
   do repositório não foi possível nesta sessão porque a GitHub API recusou a
   credencial atual com HTTP 403; confirme/ative-as com uma conta admin. Também
   confirme que a policy de Actions permite as actions fixadas por SHA e que
   PRs de forks não recebem OIDC.
8. **Abra um PR interno.** Verifique lint/testes/validate, TFLint/Checkov e o
   comentário de resumo de plan. Plan remoto é deliberadamente ignorado para
   forks.
9. **Faça merge em `main`.** O workflow aplica Terraform, publica imagens com o
   SHA do commit, atualiza os Container Apps e testa `/health`. Não faça push
   direto à branch caso a política exija revisão.
10. **Revise o primeiro deploy.** Confirme três apps saudáveis, push autenticado
    no ACR, logs no workspace, alerta de budget e role assignments. Só depois de
    revisar e aprovar as permissões execute o primeiro push de deploy à `main`.
    Apague o state local do bootstrap somente depois de confirmar a migração e
    o backup do state remoto.

> O workflow é código, mas não consegue criar a configuração de OIDC nem
> conceder RBAC sem uma identidade administrativa inicial. Esses passos são
> pré-requisitos operacionais e não são simulados nem armazenados no repositório.

## Bootstrap do state remoto

Faça o primeiro bootstrap com uma identidade Azure autorizada a criar o Resource
Group e a Storage Account. O primeiro state é local por necessidade: o backend
remoto ainda não existe.

```bash
az login
az account set --subscription "$AZURE_SUBSCRIPTION_ID"
cd infra/bootstrap
terraform init -backend=false
terraform apply -target=azurerm_resource_group.state \
  -target=azurerm_storage_account.state \
  -var='resource_group_name=rg-enem-tfstate' \
  -var='storage_account_name=uniqueaccount123' \
  -var='location=eastus2'
```

Um administrador deve atribuir `Storage Blob Data Contributor` ao operador no
escopo da Storage Account. A role de controle de plano `Contributor` não concede
acesso aos dados Blob. Reautentique para atualizar o token e rode o apply normal:

```bash
storage_id="$(az storage account show --name uniqueaccount123 \
  --resource-group rg-enem-tfstate --query id --output tsv)"
operator_object_id="<object-id-do-operador>"
az role assignment create \
  --assignee-object-id "$operator_object_id" \
  --assignee-principal-type User \
  --role "Storage Blob Data Contributor" \
  --scope "$storage_id"

terraform apply -var='resource_group_name=rg-enem-tfstate' \
  -var='storage_account_name=uniqueaccount123' \
  -var='location=eastus2'
```

Depois de provisionar o container, migre o state local para o backend remoto:

```bash
terraform init -migrate-state \
  -backend-config='resource_group_name=rg-enem-tfstate' \
  -backend-config='storage_account_name=uniqueaccount123' \
  -backend-config='container_name=tfstate' \
  -backend-config='key=bootstrap.terraform.tfstate' \
  -backend-config='use_azuread_auth=true'
```

Guarde os nomes do Resource Group e Storage Account nos Actions variables.
Conceda `Storage Blob Data Contributor` às identidades plan/apply no escopo do
container `tfstate`; não habilite autenticação por chave compartilhada.

`infra/envs/dev/terraform.tfvars.example` é apenas um exemplo, não contém valores
reais. Os valores requeridos pelo workflow são recebidos por variáveis do
GitHub Actions. Nunca comite um `.tfvars` real.

## Pipeline

### Pull requests para `main`

1. Verifica formatação e roda lint/test/build de Go, Spring e React.
2. Inicializa Terraform sem backend remoto e executa `validate`.
3. Executa TFLint e Checkov.
4. Em PRs da própria origem, autentica com OIDC, executa `terraform plan` no
   backend remoto e comenta apenas os totais de add/change/destroy.

O plano completo não é publicado no comentário, para evitar exposição acidental
de dados do state ou detalhes internos da infraestrutura. PRs de forks mantêm
as verificações sem credenciais, mas não fazem plan remoto.

### Push para `main`

Após todas as verificações, o workflow autentica por OIDC, aplica Terraform,
constrói e publica as três imagens com tag do SHA do commit, atualiza os
Container Apps e tenta `GET /health` por até 12 vezes, com timeout por chamada.
Falha de lint, testes, validação, apply, build, deploy ou smoke test falha o job.

## Dependências e versões fixadas

- Terraform CLI `1.17.1` e provider `hashicorp/azurerm` `5.11.0`.
- Actions são referenciadas por SHA completo; a versão legível está no comentário
  do workflow.
- TFLint `0.65.0`, ruleset AzureRM `0.33.0` e Checkov `3.3.30`.
- Go `1.27.3`, Java 21, Spring Boot `4.1.2`, Node `26.12.1` para CI/imagem.
- Dependências npm têm versões exatas e lockfile; Maven e Terraform também são
  verificados no pipeline.

## Melhorias e riscos a tratar depois do walking skeleton

1. **Permissões Azure do Terraform:** criar Resource Group e role assignments
   exige privilégios superiores aos necessários para um plan. Validar com o
   administrador Azure a possibilidade de roles customizadas e escopos menores;
   o workflow não deve receber Owner na subscription por conveniência.
2. **State e rede:** o endpoint público do Storage Account é necessário para
   GitHub-hosted runners neste desenho. Em produção, usar runner privado,
   Private Endpoint, regras de rede e controles de recuperação testados.
3. **Images da aplicação:** os nomes de tag usam SHA, mas o ACR Basic não oferece
   algumas políticas premium de retenção/quarentena e o pipeline ainda não
   aplica imutabilidade, assinatura, SBOM nem verificação de vulnerabilidades.
   Adicionar essas proteções antes de dados ou tráfego reais.
4. **Escala e custo:** budget é alerta, não bloqueia gasto. SKU Basic, escala
   para zero e limites atuais são escolhas de walking skeleton; confirmar a moeda,
   o valor e o contato do budget, acompanhar cold starts e carga real antes de
   definir produção.
5. **Observabilidade:** provisionar Application Insights não instrumenta os
   serviços. Adicionar telemetria, correlação de requests, alertas operacionais
   e retenção aprovada antes de depender de diagnósticos em produção.
6. **Segredos e Key Vault:** o vault está pronto com RBAC, mas ainda não existe
   necessidade de segredos no app. Ao introduzi-los, conceder acesso por
   identidade específica e nunca materializar valores em outputs ou logs.
7. **Deploy e recuperação:** o workflow aplica diretamente na `main`, sem
   aprovação manual nem rollback automatizado. Para produção, separar ambientes,
   exigir aprovação, avaliar deployment slots/revisions e documentar rollback.
8. **Escopo funcional:** `ingestion`, banco de dados, autenticação de usuários,
   contratos de API e testes end-to-end não fazem parte deste walking skeleton.
   Adicioná-los com contratos e critérios próprios, sem expor `/health` como
   substituto de readiness funcional.

## Referências oficiais consultadas

- [Terraform AzureRM backend](https://developer.hashicorp.com/terraform/language/backend/azurerm)
- [Microsoft Learn — Terraform state no Azure Storage](https://learn.microsoft.com/en-us/azure/developer/terraform/store-state-in-azure-storage)
- [Microsoft Learn — autenticação Azure de GitHub Actions via OIDC](https://learn.microsoft.com/en-us/azure/developer/github/connect-from-azure)
- [Microsoft Learn — roles de Container Apps](https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles/containers)
- [Microsoft Learn — roles e RBAC do ACR](https://learn.microsoft.com/en-us/azure/container-registry/container-registry-rbac-built-in-roles-directory-reference)
- [Microsoft Learn — roles de administrador RBAC](https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles/privileged)
- [Terraform Registry — AzureRM Container App](https://registry.terraform.io/providers/hashicorp/azurerm/5.11.0/docs/resources/container_app)
- [Terraform Registry — AzureRM Key Vault](https://registry.terraform.io/providers/hashicorp/azurerm/5.11.0/docs/resources/key_vault)
- [GitHub Docs — secure use de Actions](https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions)
