# Prompts para GitHub Copilot · Agente ENEM com GraphRAG

Um prompt por etapa do plano de execução. Todos já pedem segurança,
escalabilidade, testes automatizados, uso do corpus e controle de custos para
um projeto acadêmico com crédito limitado.

> **Atenção:** o RAG só pode usar os arquivos do corpus ENEM que você colocou
> no Blob Storage e registrou no `manifest.json`. Veja as seções **Corpus no
> bucket** e **Configuração manual por etapa** antes de iniciar.

## Como usar

- Use o Copilot em modo agente (ou abra uma issue e atribua ao Copilot coding agent). Cole o prompt da etapa e preencha os campos {{entre chaves}}.
- Trabalhe uma etapa por vez e peça o plano antes do código. Revise o plano e depois o diff como se fosse de outra pessoa.
- Agentes erram versões, nomes de SDK e configurações de nuvem. Por isso cada prompt manda consultar a documentação atual e rodar os testes.
- Depois de validar um prompt, guarde-o em `.github/prompts/` e mova as regras globais (seção abaixo) para `.github/copilot-instructions.md`. Confirme na documentação atual do Copilot os nomes e o suporte desses recursos.

## Orçamento acadêmico e conta Azure

O plano prioriza desenvolvimento local e uso da nuvem em janelas curtas. Os
valores e cotas abaixo foram consultados em 28/10/2026; confirme preços,
franquias, disponibilidade regional e elegibilidade no portal antes de criar
recursos. Relatos de cotas em fóruns não são garantias da Microsoft.

| Serviço | Tier ou limite de referência | Como o plano controla o uso |
|---|---|---|
| Azure OpenAI | Cobrança por token; cotas podem ser baixas ou zero em contas de teste/estudante. Relatos de estudantes citam 1K TPM em alguns casos, não é garantia oficial | Conferir cota na Fase 0; modelo pequeno, cache e provedor substituível |
| Azure AI Search | Free: 50 MB, até 3 índices e sem semantic ranker; confirme disponibilidade | Vetores compactos, limite de indexação e semantic ranker desligado por padrão |
| Document Intelligence | F0: 500 páginas/mês, até 4 MB por arquivo e primeiras 2 páginas por requisição | Extração local como padrão; serviço opcional e com arquivos fatiados |
| Container Apps | Consumption, escala a zero; franquia mensal por subscription indicada como 180 mil vCPU-s, 360 mil GiB-s e 2 milhões de requisições | Mínimo de réplicas zero e um ambiente; acompanhe o consumo no portal |
| Neo4j | Docker local; AuraDB Free para demonstração | Fontes consultadas divergem: GraphAcademy cita 200 mil nós/400 mil relações, enquanto página pública cita 50 mil/175 mil; confirme no console e configure o teto |
| Registry de imagens | GHCR; ACR não é tratado como gratuito neste plano | Usar GHCR e conferir as condições de visibilidade/autenticação |
| Static Web Apps | Plano Free | Confirmar limites atuais antes do deploy |
| Log Analytics / App Insights | Franquia gratuita depende do serviço e da região | Retenção mínima, amostragem e limite diário de ingestão |
| Key Vault / Storage | Standard / Standard LRS | Custo estimado baixo para o volume acadêmico; conferir no Cost Management |

O Azure for Students é anunciado com US$ 100 em crédito, sem cartão, sujeito
à verificação e renovação elegível. A conta gratuita costuma oferecer US$ 200
por 30 dias. Relatos sobre migração para pay-as-you-go dizem que a conta
gratuita pode manter o crédito restante, enquanto a migração de Student pode
remover o crédito; isso não é garantia oficial, então verifique os termos da
sua assinatura antes de migrar. Depois, configure alertas de orçamento e
lembre que o consumo acima das franquias pode ser cobrado no cartão.

Fontes consultadas no documento de referência: Azure for Students, Azure free
account, Container Apps, AI Search, Document Intelligence, AuraDB/GraphAcademy,
Neo4j AuraDB Free e discussões Microsoft Q&A sobre cotas. As cotas de OpenAI
e AuraDB citadas aqui incluem relatos e páginas que podem divergir; valide no
portal/console e na documentação atual antes de tomar decisões.

### Estratégia: local primeiro, nuvem por janelas

- Desenvolva e teste com Docker Compose, Neo4j Community, Azurite e mocks de
  LLM. Só os serviços sem emulador adequado (por exemplo, Azure AI Search)
  precisam de uma janela na nuvem.
- Se usar o crédito de 30 dias, crie/ative a conta quando estiver pronto para
  etiquetagem, indexação, deploy e avaliação.
- Ao encerrar uma sessão, destrua os recursos de dev que cobram por hora e
  confira a execução no GitHub Actions e os custos no portal.
- O orçamento do Azure alerta; ele não bloqueia gastos automaticamente.

### Corpus no bucket (Blob Storage)

O Terraform cria um Storage Account com containers de Blob. O bucket define o
escopo de evidências que o RAG pode recuperar; o upload das provas oficiais é
manual e não faz parte do repositório.

| Container | Conteúdo | Escrita | Entra em recuperação? |
|---|---|---|---|
| `enem-raw` | PDFs oficiais do INEP, Matriz de Referência e `manifest.json` | Upload manual | Não; é a fonte de ingestão |
| `enem-processed` | `questoes.jsonl`, imagens extraídas e chunks | Ingestão Go | Sim, apenas `split=index` |
| `enem-golden` | Itens de holdout para avaliação | Ingestão, conforme manifest | Nunca |
| `enem-generated` | Questões geradas pelo agente | API | Só após revisão manual |

O `manifest.json` lista nome, SHA-256, ano, caderno/cor, tipo, URL de origem,
split (`index` ou `holdout`) e `corpus_version`. A ingestão processa somente
arquivos manifestados, confere o hash e ignora/reporta arquivos fora da lista.
Cada questão, chunk, nó do grafo e documento indexado registra arquivo de
origem, página e `corpus_version`; as consultas filtram pela versão vigente.
O agente cita arquivo, página e id da questão e se abstém quando o corpus não
oferece suporte. Holdout nunca entra no índice nem no grafo de recuperação.

Use containers privados, sem acesso anônimo, desabilite chave compartilhada
quando suportado e aplique Managed Identity/RBAC com privilégio mínimo.
Ative versionamento de blobs. Em dev, espelhe a estrutura sob `data/` com
Azurite; mantenha PDFs fora do Git (`data/` no `.gitignore`). Para economizar
o limite do Search, indexe texto, metadados e vetores, não PDFs ou imagens.

### Configuração manual por etapa

| Quando | Ação manual | Motivo |
|---|---|---|
| Antes da Fase 0 | Criar o repositório, ativar o Copilot e criar a conta Azure; anotar subscription/tenant IDs | Conta, identidade e cobrança pertencem a você; adie uma conta de 30 dias até estar pronto |
| Antes da Fase 0 | Instalar Git, Docker, Go, JDK LTS, Node, Terraform e Azure CLI; executar `az login` | A máquina e a autenticação são suas |
| Antes da Fase 0 | Confirmar permissões para criar roles e app registrations no Entra; conferir cota/região do OpenAI | Permissões e cotas da subscription não são criadas por Terraform |
| Antes da Fase 0 | Criar alertas de orçamento em 50% e 80% do crédito | A cobrança é sua; os alertas não limitam gastos |
| Fase 0 | Aplicar `infra/bootstrap` uma vez com login local; configurar OIDC, branch protection, variáveis do GitHub e segurança do repositório | O backend remoto precisa existir antes; Entra e GitHub exigem ações manuais |
| Fase 1 | Baixar provas/gabaritos oficiais de Matemática (3–5 anos), subir em `enem-raw`, preencher e validar o manifest, revisar 10–15% das etiquetas | Fonte, split de holdout e revisão pedagógica são decisões humanas |
| Fase 2 | Resolver eventual limite de AI Search Free por subscription/região e aceitar termos de modelos se solicitado | Limite de serviço/aceite humano |
| Fase 3 | Criar AuraDB Free, guardar credenciais com segurança, retomar a instância pausada e revisar relações pedagógicas | Aura não é criado pelo Terraform Azure; pré-requisitos requerem revisão |
| Fases 4A e 5 | Registrar apps de API e front no Entra, audiences/scopes/redirect URIs e usuários de teste | URLs e permissões do tenant exigem configuração manual |
| Fase 7 | Conferir Cost Management semanalmente | O crédito e os gastos são seus |
| Fase 8 | Executar teardown e conferir no portal que nada cobrável sobrou; pausar/apagar Aura | Verificação final de cobrança |

## Modelo base

```
CONTEXTO
Projeto: {{nome}}. Stack: {{stack}}. Etapa: {{fase}}.
Arquivos e pastas envolvidos: {{caminhos}}.
O que já existe: {{resumo}}.
Corpus (bucket): {{containers e manifest que esta etapa lê ou grava}}.

TAREFA
{{o que deve ser feito, em passos numerados}}

SEGURANÇA
{{entradas a validar, autenticação e autorização, segredos,
dependências, ameaças específicas}}

ESCALABILIDADE
{{stateless, timeouts, retries, paginação, limites,
escala horizontal, gargalos esperados}}

TESTES AUTOMATIZADOS
{{unitários, integração, contrato, e2e; cobertura mínima;
o que o CI deve bloquear}}

CORPUS (bucket)
{{o que esta etapa lê e grava nos containers e as regras do manifest}}

RESTRIÇÕES DE CUSTO (projeto acadêmico, free tier)
{{tiers e limites usados, o que escala a zero, teto de gasto e como destruir
os recursos pagos}}

CRITÉRIO DE PRONTO
{{resultado verificável}}

FORMA DE TRABALHO
Apresente um plano antes de codar. Faça mudanças pequenas e focadas.
Escreva os testes junto com o código, rode-os e mostre o resultado.
Não invente APIs, versões ou nomes de SDK: se tiver dúvida, diga e
consulte a documentação. Liste ao final riscos, pendências e decisões.
```

## Regras globais (copilot-instructions.md)

```
- Segurança primeiro: valide toda entrada, aplique privilégio mínimo, nunca
  coloque segredos no código ou nos logs, use queries parametrizadas e trate
  texto de documentos e de LLM como não confiável.
- Escalabilidade: serviços stateless, timeouts, retries com backoff, paginação
  e limites explícitos em tudo que consome recursos.
- Testes: todo código novo vem com testes automatizados (unitários e, onde
  houver I/O, de integração). Rode e mostre os resultados.
- Não invente APIs, versões ou SDKs; confirme na documentação atual.
- Mudanças pequenas e focadas, com explicação das decisões e dos riscos.
- Custo: prefira tiers gratuitos, escala a zero, modelos menores e cache;
  nenhum recurso pago sem variável explícita de opt-in; documente o destroy
  de tudo que cobra por hora.
- Corpus: o RAG usa apenas arquivos listados no manifest do bucket ENEM.
  Registre origem, página e `corpus_version`; holdout nunca entra no índice
  nem no grafo de recuperação.
```

## Prompt 0 · Fundação e walking skeleton

```
CONTEXTO
Monorepo enem-platform: apps/web (React + TS), apps/api (Java Spring Boot),
apps/retrieval-svc e apps/ingestion (Go), infra/ (Terraform), contracts/.
Cloud: Azure. CI/CD: GitHub Actions. Etapa: Fase 0, walking skeleton.

TAREFA
1. infra/bootstrap: Storage Account para o state remoto, com versionamento e
   lock; configure o backend azurerm nos ambientes.
2. Módulos em infra/modules e ambiente infra/envs/dev: resource group,
   Log Analytics e App Insights, Key Vault (RBAC), Storage Account privado
   para o corpus ENEM, Container Apps Environment, Managed Identities,
   alerta de orçamento e tags de custo.
3. Hello-world com GET /health em Spring e em Go, e uma página React, cada
   um com Dockerfile multi-stage.
4. Workflows: terraform fmt/validate/plan no PR (plan comentado no PR),
   apply na main, build e deploy dos 3 serviços em dev, com imagens no GHCR.

SEGURANÇA
- Autenticação GitHub para Azure somente por OIDC, sem client secret.
- Privilégio mínimo por identidade; nenhum segredo no repo ou nos logs.
- Containers com imagem base mínima, usuário não-root e portas mínimas.
- tflint e checkov no pipeline; providers e actions com versão pinada.
- Configure Dependabot e secret scanning.

ESCALABILIDADE
- Container Apps com mínimo e máximo de réplicas e regra de escala HTTP,
  parametrizados por variável.
- Serviços stateless, configuração por variáveis de ambiente.
- Módulos Terraform reutilizáveis, parametrizados por ambiente (dev/prod).

TESTES AUTOMATIZADOS
- Spring: teste do /health. Go: teste do handler com httptest.
  React: teste com Vitest e Testing Library.
- O pipeline falha se lint, testes ou terraform validate falharem.
- Smoke test pós-deploy chamando /health nos 3 serviços.

CORPUS (bucket)
- Terraform cria o Storage Account Standard LRS privado, sem acesso anônimo,
  com chave compartilhada desabilitada e versionamento de blobs; crie os
  containers enem-raw, enem-processed, enem-golden e enem-generated.
- RBAC mínimo: ingestão lê enem-raw e escreve processed/golden; retrieval lê
  processed. Não conceda acesso do retrieval ao holdout.
- Use endpoint configurável (Azurite local ou Blob na nuvem). Upload manual
  dos arquivos ENEM em enem-raw fica para a Fase 1; nesta fase o bucket nasce
  vazio.

RESTRIÇÕES DE CUSTO (projeto acadêmico, free tier)
- Prefira GHCR; não crie ACR sem uma variável explícita de opt-in e análise
  de preço.
- Container Apps Consumption com min_replicas = 0 e um ambiente; acompanhe
  a franquia por subscription.
- Retenção mínima, amostragem e daily cap no Log Analytics; Key Vault
  Standard.
- Alerta de orçamento em 50% e 80%. Documente e teste o workflow manual
  destroy-dev para recursos cobrados por hora.
- Inclua `scripts/check-azure` para login, permissões e cota do OpenAI;
  chamada mínima ao serviço apenas com consentimento/execução manual.

CRITÉRIO DE PRONTO
Um push na main implanta os 3 serviços em dev e o PR exibe o terraform plan.

FORMA DE TRABALHO
Plano antes de codar, mudanças pequenas, testes rodados e mostrados.
Não invente versões ou nomes de SDK; consulte a documentação.
```

## Prompt 1 · Dados com Go (ingestão)

```
CONTEXTO
apps/ingestion em Go. Esquema da questão em contracts/questao.schema.json.
Etapa: Fase 1. Entrada: PDFs de provas e gabaritos do INEP (Matemática).

TAREFA
1. CLI com subcomandos extract, validate, label e export, cada etapa
   idempotente.
2. extract: extração local primeiro (texto nativo e rasterização de páginas
   com figuras); Document Intelligence F0 como fallback. Imagens no Blob,
   referenciadas pelo id da questão.
3. label: habilidade, competência e conteúdos com Azure OpenAI e saída
   estruturada, validada contra o JSON Schema.
4. Saída em questoes.jsonl e relatório de qualidade com amostra de 10 a 15%
   para revisão manual.

SEGURANÇA
- DefaultAzureCredential / Managed Identity, sem chaves no código.
- Texto dos PDFs é dado não confiável: nunca vira instrução para o LLM
  (delimite o conteúdo e valide a saída contra o schema).
- Valide tipo e tamanho dos arquivos; sem dados pessoais nos logs.
- Rode govulncheck e fixe as dependências.

ESCALABILIDADE
- Pool de workers configurável, com backpressure e rate limit respeitando
  as cotas (RPM e TPM) do Azure OpenAI.
- Retry com backoff exponencial e jitter em 429 e 5xx.
- Checkpoint e retomada (idempotência por hash do arquivo).
- Streaming de jsonl para manter memória limitada.

TESTES AUTOMATIZADOS
- Testes de tabela e golden files para parser e validador.
- Cliente do Azure OpenAI atrás de interface, com mock nos testes.
- Teste de idempotência: rodar duas vezes gera o mesmo resultado.
- go test -race; cobertura mínima de 80% em parse e validate.

CORPUS (bucket)
- Entrada: enem-raw (ou data/raw em dev). Leia apenas os arquivos do
  manifest.json (nome, sha256, ano, caderno, tipo, fonte, split,
  corpus_version) e confira o hash.
- Arquivo fora do manifest é ignorado e reportado; hash divergente falha.
- Saída em enem-processed (questoes.jsonl e imagens). Itens holdout vão para
  enem-golden e nunca para enem-processed. Cada questão guarda arquivo,
  página e corpus_version.
- Os arquivos oficiais e o manifest são preparados por mim antes da execução;
  não versione PDFs (data/ no .gitignore).

RESTRIÇÕES DE CUSTO (projeto acadêmico, free tier)
- Extração local é o padrão. Document Intelligence F0 é opcional, com limites
  atuais confirmados na documentação; fragmente documentos se necessário.
- Use modelo pequeno, cache por hash, `--dry-run` para estimar tokens e
  `--max-tokens` para abortar acima do teto. Comece por uma prova.
- Azure OpenAI fica atrás de interface; permita provedor alternativo ou
  modelo local se a cota Azure for zero.

CRITÉRIO DE PRONTO
3 a 5 anos de prova processados, pelo menos 90% das questões extraídas
corretamente e todo o jsonl válido contra o schema.

FORMA DE TRABALHO
Plano antes de codar, mudanças pequenas, testes rodados e mostrados.
Não invente versões ou nomes de SDK; consulte a documentação.
```

## Prompt 2 · Índice, retrieval service e baseline

```
CONTEXTO
apps/retrieval-svc (Go), evaluation/ e contracts/openapi.yaml.
Etapa: Fase 2. questoes.jsonl já validado.

TAREFA
1. Schema do índice do Azure AI Search versionado no repo e comando para
   criar ou atualizar o índice.
2. Indexação em lote com embeddings; campos de filtro (ano, habilidade,
   conteudos, origem) e campo vetorial.
3. Endpoint POST /v1/search com busca híbrida (vetor + BM25) e semantic
   ranker opcional (flag, desligado no tier Free); contrato publicado no
   OpenAPI.
4. Harness em evaluation/ com golden set de holdout, rodando o baseline A
   (LLM puro) e o baseline B (RAG híbrido): acurácia, custo e latência.

SEGURANÇA
- Managed Identity e RBAC: leitura para o serviço de busca, escrita só
  para a ingestão.
- Valide e limite parâmetros (top, filtros). Use allowlist de campos e
  escape nas expressões de filtro OData.
- Rate limit por cliente, timeouts, erros sem detalhes internos.
- Limite de tamanho de payload.

ESCALABILIDADE
- Serviço stateless com readiness e liveness.
- Timeouts, retry e circuit breaker nos clientes externos.
- Cache com TTL para embeddings de consultas repetidas.
- Paginação e indexação em lotes.

TESTES AUTOMATIZADOS
- Unitários e testes de contrato contra o OpenAPI.
- Integração com fake ou mock do AI Search.
- Teste garantindo que nenhuma questão do holdout entra no índice.
- Harness reprodutível, gravando versão do prompt, modelo e seed.

CORPUS (bucket)
- Indexe somente enem-processed com split=index e corpus_version vigente:
  texto, metadados e vetores; nunca PDFs brutos.
- Cada documento registra corpus_version, arquivo e página; a busca filtra
  pela versão vigente. enem-golden nunca é indexado; teste os ids.
- Reindexação deve ser reprodutível a partir do manifest.

RESTRIÇÕES DE CUSTO (projeto acadêmico, free tier)
- AI Search Free tem limite pequeno (referência: 50 MB, 3 índices e sem
  semantic ranker); confirme cota e disponibilidade no portal.
- Use dimensões de embedding reduzidas e recuse indexação acima de 80% do
  limite configurado, com teste.
- Cache embeddings/respostas; limite tokens e tamanho do golden set.
- Semantic ranker só em experimento explicitamente pago, curto e destruído
  no mesmo dia.

CRITÉRIO DE PRONTO
Um comando gera a tabela comparando A e B, e o CI roda os testes.

FORMA DE TRABALHO
Plano antes de codar, mudanças pequenas, testes rodados e mostrados.
Não invente versões ou nomes de SDK; consulte a documentação.
```

## Prompt 2T · Terraform dos serviços de IA

```
CONTEXTO
infra/modules/ai e infra/envs/dev. Etapa: Fase 2, antes ou junto do Prompt 2.
Já existem os módulos base da Fase 0, inclusive o Storage do corpus.
Conta Azure com crédito limitado.

TAREFA
1. Criar módulo de IA com Azure OpenAI (deployment de modelo pequeno de chat
   e embeddings, nome/versão por variável), Azure AI Search (SKU variável,
   padrão Free) e Document Intelligence (SKU variável, F0 opcional).
2. Role assignments: leitura do Search para retrieval, escrita para ingestão,
   uso do OpenAI pela API e ingestão.
3. Outputs com endpoints e referências no Key Vault; nunca expor chaves.
4. Variáveis de proteção: enable_search_paid_tier,
   enable_document_intelligence, enable_openai e openai_capacity baixo.
   SKU pago exige opt-in explícito.
5. Região por variável; documentar disponibilidade e cota.

SEGURANÇA
- Entra ID e RBAC; desabilite autenticação por chave local e restrinja acesso
  público quando suportado pelo tier.
- Privilégio mínimo, outputs sensíveis e state remoto protegido.
- Checkov e tflint sem achados altos; providers pinados.

ESCALABILIDADE
- TPM do OpenAI, réplicas e partições do Search por variável, com padrão
  mínimo e documentação para aumentar.
- Módulo reutilizável sem valores fixos; dependências explícitas e timeouts.

TESTES AUTOMATIZADOS
- fmt, validate, tflint, checkov e plan comentado no PR.
- Teste do plan garantindo SKUs gratuitos por padrão e opt-in para pagos.
- Verificação pós-apply mínima e manual para OpenAI/Search; erro claro se
  faltar cota.

CORPUS (bucket)
- Search/OpenAI recebem apenas dados derivados de enem-processed. Nenhuma
  identidade do Search/retrieval acessa enem-golden.

RESTRIÇÕES DE CUSTO (projeto acadêmico, free tier)
- Search Free por padrão, Document Intelligence F0 opcional e OpenAI com
  modelos pequenos/TPM baixo.
- Permita aplicar sem OpenAI (`enable_openai=false`) se não houver cota.
- Tier pago do Search só com opt-in, janela curta e destroy documentado.
- Liste no PR os recursos que podem cobrar antes do apply.

CRITÉRIO DE PRONTO
terraform apply cria o ambiente com SKUs gratuitos; verificação passa e
terraform destroy remove os recursos.

FORMA DE TRABALHO
Plano antes de codar, mudanças pequenas, testes rodados e mostrados.
Não invente nomes de recursos, SKUs ou versões; consulte a documentação.
```

## Prompt 3 · Grafo no Neo4j

```
CONTEXTO
apps/ingestion (loader) e apps/retrieval-svc (consultas), em Go.
Etapa: Fase 3. Pré-requisitos entre conteúdos revisados por mim em
data/processed/prerequisitos.csv.

TAREFA
1. Constraints e índices (Questao.id, Habilidade.codigo, Conteudo.id).
2. Loader idempotente (MERGE com UNWIND em lotes): Area, Competencia,
   Habilidade, Questao, AVALIA, ABORDA e PRE_REQUISITO_DE.
3. Endpoints em /v1/graph/ com Cypher parametrizado: questões por
   habilidade e dificuldade, pré-requisitos até N níveis e cobertura de
   habilidades de um simulado.

SEGURANÇA
- Somente parâmetros ($param); nunca concatenar texto em Cypher.
- Profundidade N com teto máximo; rejeite valores acima dele.
- Usuário Neo4j somente leitura no retrieval; escrita só na ingestão e na API.
- Credenciais no Key Vault, conexão com TLS (neo4j+s).
- Sem text2cypher livre neste serviço.

ESCALABILIDADE
- Pool de conexões configurado e transações de leitura explícitas.
- LIMIT e paginação obrigatórios; timeout por query.
- Índices para os padrões de consulta, verificados com EXPLAIN e PROFILE.
- Carga em lotes.

TESTES AUTOMATIZADOS
- Testcontainers com Neo4j e fixtures pequenas.
- 10 casos com resultado verificado à mão.
- Teste de idempotência do loader (rodar duas vezes não duplica).
- Teste de injeção: entrada maliciosa não altera a query.
- Teste de que profundidade acima do teto é rejeitada.

CORPUS (bucket)
- Cada Questao tem corpus_version, arquivo e página de origem.
- Loader lê apenas enem-processed; holdout nunca vai para o grafo de busca.

RESTRIÇÕES DE CUSTO (projeto acadêmico, free tier)
- Neo4j Community em Docker no dev/CI; AuraDB Free apenas para demo.
- Configure limites de nós/relações; interrompa a carga ao exceder 80% do
  teto escolhido e teste esse comportamento.
- Aura não é criado pelo Terraform Azure. Documente criação manual, retomada
  após pausa e armazenamento seguro da senha.

CRITÉRIO DE PRONTO
As consultas retornam o esperado nos 10 casos e o CI roda os testes
com Testcontainers.

FORMA DE TRABALHO
Plano antes de codar, mudanças pequenas, testes rodados e mostrados.
Não invente versões ou nomes de SDK; consulte a documentação.
```

## Prompt 4A · API Spring Boot

```
CONTEXTO
apps/api em Java (LTS atual) com Spring Boot (versão estável atual).
Etapa: Fase 4, parte A. Contratos em contracts/openapi.yaml.

TAREFA
1. Módulos auth, questoes, simulados, tentativas e relatorios.
2. Spring Security como resource server validando JWT do Entra ID
   (issuer, audience e escopos).
3. Cliente do retrieval-svc gerado a partir do OpenAPI.
4. Persistência de tentativas no Neo4j.
5. Erros no formato ProblemDetail (RFC 7807).

SEGURANÇA
- Bean Validation em todo input.
- Autorização por dono do recurso: o aluno só acessa as próprias tentativas
  (cubra IDOR em testes).
- CORS restrito, headers de segurança e rate limiting.
- Sem stack trace na resposta; logs sem tokens nem dados pessoais.
- Segredos por Managed Identity e Key Vault; verificação de dependências.

ESCALABILIDADE
- Stateless, sem sessão em memória.
- Timeouts, retry e circuit breaker (ex.: Resilience4j) nos clientes.
- Pool de conexões, paginação e idempotency key ao registrar tentativa.
- Actuator com liveness e readiness; configuração 12-factor.

TESTES AUTOMATIZADOS
- JUnit 5; WebMvcTest para controllers.
- Testcontainers (Neo4j) para repositórios; WireMock para o retrieval-svc.
- Testes de segurança: 401, 403 e IDOR.
- JaCoCo com mínimo de 80% na camada de domínio; CI bloqueia abaixo disso.

RESTRIÇÕES DE CUSTO (projeto acadêmico, free tier)
- Rode local via Docker Compose (Neo4j Community, retrieval real ou mock,
  emissor JWT de teste/Keycloak). Entra ID entra apenas no perfil cloud.
- Não crie recurso pago novo; trate cold start do Container Apps com probes,
  timeouts e retries compatíveis.

CRITÉRIO DE PRONTO
Via API você registra tentativas, monta um simulado de 10 itens e consulta
somente os próprios dados.

FORMA DE TRABALHO
Plano antes de codar, mudanças pequenas, testes rodados e mostrados.
Não invente versões ou nomes de SDK; consulte a documentação.
```

## Prompt 4B · Agente e ferramentas

```
CONTEXTO
Módulo agente em apps/api. Etapa: Fase 4, parte B.
Orquestrador: {{Spring AI | LangChain4j | Semantic Kernel Java | loop próprio
com o SDK do Azure OpenAI}}. Antes de escolher, verifique na documentação
atual o suporte a function calling e a streaming.

TAREFA
1. Ferramentas tipadas: buscar_questoes_similares e consultar_grafo (via
   retrieval-svc), calcular (sandbox), registrar_resposta, gerar_simulado
   e gerar_relatorio.
2. Fluxos: Resolver, Gerar questão, Simulado e Relatório.
3. Gerar questão: um LLM-juiz resolve sem ver o gabarito e a questão é
   descartada se ele discordar; grava com origem gerada.
4. Política de abstenção: sem contexto suficiente, o agente diz que não sabe.
5. Respostas com citações (ids dos chunks e dos nós do grafo) e streaming SSE.

SEGURANÇA
- Prompt injection: documentos recuperados e entrada do usuário são dados,
  não instruções. Use delimitadores e allowlist de ferramentas.
- Valide os argumentos de cada ferramenta contra schema.
- Ferramentas somente leitura, exceto registrar_resposta.
- Limite de iterações e de tokens por requisição.
- Sandbox do calcular sem rede e com timeout.
- Não exponha o system prompt; registre tool calls para auditoria sem
  dados pessoais.

ESCALABILIDADE
- Stateless: histórico da conversa em store externo, não em memória.
- Timeout por ferramenta; retry com backoff em 429.
- Limite de concorrência por usuário (bulkhead) e orçamento de tokens.
- Cache de buscas idênticas.
- Métricas de latência e custo por requisição.

TESTES AUTOMATIZADOS
- Unitários por ferramenta e fluxos com LLM mockado (determinísticos).
- Teste de abstenção com contexto insuficiente.
- Teste de injeção: documento com instruções maliciosas não muda o
  comportamento.
- Teste do juiz: questão com gabarito errado é descartada.
- Teste do limite de iterações.
- Avaliação com o golden set em job separado, fora do CI rápido.

CORPUS (bucket)
- Só afirme o que retrieval/grafo sustentam; sem suporte, abstenha-se.
  Citações incluem arquivo, página, id e corpus_version.
- Questões geradas vão para enem-generated com origem=gerada e só entram no
  índice após revisão manual.

RESTRIÇÕES DE CUSTO (projeto acadêmico, free tier)
- Limite tokens por requisição/usuário/dia; modelo pequeno por padrão.
- Cache por hash de pergunta + contexto. Prefira cálculo determinístico local;
  sessões dinâmicas pagas exigem opt-in.
- Testes usam mocks; chamadas reais apenas em job manual com orçamento
  definido. Registre estimativa de custo por requisição.

CRITÉRIO DE PRONTO
Ciclo completo via API: resolver uma questão, simulado de 10 itens,
registrar respostas e receber o relatório.

FORMA DE TRABALHO
Plano antes de codar, mudanças pequenas, testes rodados e mostrados.
Não invente versões ou nomes de SDK; consulte a documentação.
```

## Prompt 5 · Front-end React

```
CONTEXTO
apps/web com Vite e TypeScript em modo strict. API descrita em
contracts/openapi.yaml. Etapa: Fase 5.

TAREFA
1. React Router, TanStack Query, Tailwind e Recharts.
2. Cliente TypeScript gerado do OpenAPI; sem tipos de API escritos à mão.
3. Login com Entra ID (MSAL React).
4. Telas: Resolver (chat com streaming SSE e citações clicáveis),
   Gerar questão (badge gerada e resultado do juiz), Simulado (timer,
   navegação, marcar para revisão, envio), Relatório (por área e
   habilidade, tendência, plano de estudo) e Histórico.
5. Deploy via módulo Terraform de Azure Static Web Apps (Free), URL da API
   como variável de build e CORS da API permitindo somente essa origem.

SEGURANÇA
- Nunca usar dangerouslySetInnerHTML com saída do LLM; renderize markdown
  sanitizado (ex.: DOMPurify).
- Revise onde o MSAL guarda tokens; sem segredos no bundle (só variáveis
  públicas VITE_).
- Valide respostas da API com zod.
- CSP e headers de segurança no deploy; rode npm audit no CI.

ESCALABILIDADE
- Code splitting por rota e lazy loading.
- Cache e invalidação com TanStack Query; virtualização de listas longas.
- Estados de loading e erro com retry; reconexão do SSE.
- Orçamento de tamanho do bundle verificado no CI.

TESTES AUTOMATIZADOS
- Vitest e Testing Library para componentes e hooks; MSW para mockar a API.
- Playwright e2e: login, simulado e relatório.
- Testes de acessibilidade com axe.
- Teste de que HTML malicioso na resposta do agente não executa.

RESTRIÇÕES DE CUSTO (projeto acadêmico, free tier)
- Static Web Apps no plano Free; sem analytics, CDN ou fontes pagas.
- Teste local com API; e2e cloud apenas manualmente.
- Terraform validate/plan do módulo frontend no CI.

CRITÉRIO DE PRONTO
Um usuário faz login, resolve uma questão, faz um simulado e vê o relatório
em dev, com os testes verdes no CI.

FORMA DE TRABALHO
Plano antes de codar, mudanças pequenas, testes rodados e mostrados.
Não invente versões ou nomes de biblioteca; consulte a documentação.
```

## Prompt 6 · CI/CD completo e produção

```
CONTEXTO
.github/workflows e infra/envs/prod. Etapa: Fase 6.
Já existem pipelines básicos de infra e deploy em dev.

TAREFA
1. Workflows por componente (infra, go, java, web, contracts) com path
   filters, cache e workflows reutilizáveis (workflow_call).
2. contracts: lint com Spectral e checagem de breaking changes com oasdiff.
3. Gate de avaliação: quando prompts/**, o agente ou o schema do grafo
   mudam, roda o golden set (ou subconjunto), comenta métricas e custo no
   PR e falha se a queda passar de {{limite}}.
4. Ambiente prod como GitHub Environment com aprovação manual. No free tier,
   prod pode ser uma segunda revisão do mesmo ambiente; crie ambiente
   separado só se o orçamento permitir.
5. Imagens imutáveis por SHA no GHCR; deploy por revisão do Container Apps
   com divisão de tráfego e rollback.
6. Release por tag semver e Dependabot ou Renovate.

SEGURANÇA
- OIDC, e permissions mínimas por job.
- Actions pinadas por SHA; PRs de forks sem acesso a secrets.
- Trivy nas imagens, govulncheck, verificação de dependências Java e npm,
  checkov na infra, secret scanning.
- Proteção da branch main e CODEOWNERS.
- Nada de secrets em logs.

ESCALABILIDADE
- Jobs paralelos, matriz quando fizer sentido, cache de dependências e de
  camadas Docker.
- Concurrency groups cancelando execuções obsoletas.
- Feedback do PR em poucos minutos.

TESTES AUTOMATIZADOS
- O pipeline executa unitários, integração e e2e.
- Smoke tests pós-deploy bloqueiam a promoção para prod.
- Valide o rollback em dev.
- Teste o gate de avaliação com uma regressão proposital.

CORPUS (bucket)
- CI valida schema, SHA-256 sem duplicatas e splits válidos no manifest
  quando ele mudar. O pipeline nunca baixa nem publica PDFs do bucket.

RESTRIÇÕES DE CUSTO (projeto acadêmico, free tier)
- Registry GHCR por padrão, sem ACR.
- Gate de avaliação em PR usa subconjunto pequeno, cache e modelo barato;
  avaliação completa apenas por workflow_dispatch com orçamento.
- Use path filters e cancele execuções obsoletas; monitore a cota de Actions.
- Workflow manual destroy-dev documentado para recursos que cobram por hora.

CRITÉRIO DE PRONTO
PR que altera prompt dispara a avaliação e é bloqueado se a métrica cair;
merge faz deploy em dev; prod exige aprovação; rollback testado.

FORMA DE TRABALHO
Plano antes de codar, mudanças pequenas, testes rodados e mostrados.
Não invente nomes de actions ou versões; consulte a documentação.
```

## Prompt 7 · Avaliação e hardening

```
CONTEXTO
evaluation/, apps/api, apps/retrieval-svc e infra/. Etapa: Fase 7.
Agente completo funcionando em dev.

TAREFA
1. Comparativo A (LLM puro), B (RAG híbrido), C (RAG + código) e
   D (agente completo): acurácia, custo e latência.
2. Gerar 50 questões e medir a taxa de aprovação do LLM-juiz.
3. 3 a 5 alunos sintéticos com lacunas conhecidas; verificar se o relatório
   identifica a lacuna raiz.
4. OpenTelemetry em Java e Go, com propagação de traceparent do React ao
   Neo4j, e Application Insights; custo por requisição.
5. Teste de carga com k6 nos cenários resolver e simulado.

SEGURANÇA
- Modelagem de ameaças leve (STRIDE) em docs/security.md.
- Testes de abuso: prompt injection, IDOR e payloads grandes.
- Checkov e Trivy sem achados altos; revisão de privilégios mínimos.
- Confirmar que o usuário Neo4j somente leitura não consegue escrever.
- Logs sem dados pessoais; plano de rotação de segredos.

ESCALABILIDADE
- Metas de p95 e de taxa de erro definidas antes dos testes de carga.
- Gargalos identificados pelos traces.
- Verificar a escala automática do Container Apps sob carga.
- Limites de custo e de tokens; documentar a capacidade medida.

TESTES AUTOMATIZADOS
- Harness versionado e reprodutível (seed, versão de prompt e de modelo).
- Alunos sintéticos executados automaticamente.
- Testes de regressão de métricas.
- Teste de contrato dos spans e atributos essenciais de telemetria.

CORPUS (bucket)
- Avaliação lê enem-golden e verifica que seus ids não aparecem no índice
  nem no grafo. Cada relatório registra corpus_version e versão do manifest.

RESTRIÇÕES DE CUSTO (projeto acadêmico, free tier)
- Defina teto de tokens para cada rodada A/B/C/D; use cache e rode a
  avaliação completa uma única vez no final.
- k6 local por padrão; na nuvem, somente testes curtos acompanhando franquia.
- Configure amostragem e daily cap no Application Insights e compare o custo
  medido no Cost Management com a estimativa.

CRITÉRIO DE PRONTO
Números comparáveis entre A, B, C e D, e explicação de onde o grafo
ajudou e onde não ajudou.

FORMA DE TRABALHO
Plano antes de codar, mudanças pequenas, testes rodados e mostrados.
Não invente versões ou nomes de SDK; consulte a documentação.
```

## Prompt 8 · Entrega

```
CONTEXTO
Raiz do repositório e docs/. Etapa: Fase 8. Projeto funcional e avaliado.

TAREFA
1. README com problema, arquitetura (diagrama em Mermaid), decisões e
   resultados.
2. ADRs em docs/adr: Go e Spring, Neo4j, busca híbrida e orquestrador.
3. Estimativa de custo mensal por ambiente e runbook de deploy e rollback.
4. Makefile ou scripts para subir o ambiente e rodar a avaliação.
5. Seção de limitações honesta e roteiro da demo.

SEGURANÇA
- Auditar o histórico do repositório com gitleaks.
- Criar SECURITY.md e documentar a rotação de segredos.
- Conferir licenças das dependências e que nenhum material protegido
  é redistribuído.

ESCALABILIDADE
- Documentar os limites medidos na Fase 7, os gargalos conhecidos e os
  próximos passos (cache, filas, particionamento).

TESTES AUTOMATIZADOS
- Validar o README do zero (clonar, subir, testar) em ambiente limpo.
- Verificar links dos documentos e os comandos do Makefile no CI.

CORPUS (bucket)
- Documente fontes oficiais, containers, formato/validação do manifest e
  ingestão do zero. Não publique PDFs no repositório.

RESTRIÇÕES DE CUSTO (projeto acadêmico, free tier)
- Estime custo usando consumo medido, inclusive manter tudo ligado por mês.
- Runbook de teardown: terraform destroy, verificar no portal AI Search,
  OpenAI, Storage e Log Analytics, e pausar/apagar Aura.
- Documente como rodar 100% local sem conta Azure.

CRITÉRIO DE PRONTO
Alguém sem contexto consegue clonar o repositório, subir o ambiente e
reproduzir a avaliação seguindo só o README.

FORMA DE TRABALHO
Plano antes de editar, mudanças pequenas, comandos testados.
Não invente informações; marque o que for estimativa.
```
