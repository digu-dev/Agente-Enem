# Prompts para GitHub Copilot · Agente ENEM com GraphRAG

Um prompt por etapa do plano de execução. Todos já pedem segurança, escalabilidade e testes automatizados.

## Como usar

- Use o Copilot em modo agente (ou abra uma issue e atribua ao Copilot coding agent). Cole o prompt da etapa e preencha os campos {{entre chaves}}.
- Trabalhe uma etapa por vez e peça o plano antes do código. Revise o plano e depois o diff como se fosse de outra pessoa.
- Agentes erram versões, nomes de SDK e configurações de nuvem. Por isso cada prompt manda consultar a documentação atual e rodar os testes.
- Depois de validar um prompt, guarde-o em `.github/prompts/` e mova as regras globais (seção abaixo) para `.github/copilot-instructions.md`. Confirme na documentação atual do Copilot os nomes e o suporte desses recursos.

## Modelo base

```
CONTEXTO
Projeto: {{nome}}. Stack: {{stack}}. Etapa: {{fase}}.
Arquivos e pastas envolvidos: {{caminhos}}.
O que já existe: {{resumo}}.

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
   Log Analytics e App Insights, Key Vault (RBAC), ACR, Container Apps
   Environment, Managed Identities, alerta de orçamento e tags de custo.
3. Hello-world com GET /health em Spring e em Go, e uma página React, cada
   um com Dockerfile multi-stage.
4. Workflows: terraform fmt/validate/plan no PR (plan comentado no PR),
   apply na main, build e deploy dos 3 serviços em dev.

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
2. extract: Azure Document Intelligence (ou modelo multimodal); imagens no
   Blob, referenciadas pelo id da questão.
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
   ranker; contrato publicado no OpenAPI.
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

CRITÉRIO DE PRONTO
Um comando gera a tabela comparando A e B, e o CI roda os testes.

FORMA DE TRABALHO
Plano antes de codar, mudanças pequenas, testes rodados e mostrados.
Não invente versões ou nomes de SDK; consulte a documentação.
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
4. Ambiente prod com GitHub Environment e aprovação manual.
5. Imagens imutáveis por SHA no ACR; deploy por revisão do Container Apps
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

CRITÉRIO DE PRONTO
Alguém sem contexto consegue clonar o repositório, subir o ambiente e
reproduzir a avaliação seguindo só o README.

FORMA DE TRABALHO
Plano antes de editar, mudanças pequenas, comandos testados.
Não invente informações; marque o que for estimativa.
```
