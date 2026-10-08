# Implementation Plan: Fase 1 — Encurtador de links com analytics (MVP)

**Branch**: `001-encurtador-analytics` | **Date**: 2026-10-08 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/001-encurtador-analytics/spec.md`

## Summary

Encurtador de links de afiliado com analytics por canal: o administrador cadastra marketplaces,
canais e links pelo painel; cada link recebe um código de 7 caracteres e URLs por canal.

O redirecionamento responde `302` a partir de um cache em memória e enfileira o evento sem
bloquear. Um worker grava os eventos em lote e calcula a unicidade permanente via `visitor_seen`
com `ON CONFLICT`. Crawlers recebem uma prévia Open Graph sem evento, e automação é marcada como
robô. Jobs diários agregam em `click_daily` (de forma idempotente, no fuso de relatório), limpam
eventos antigos de dias já agregados e expiram a lixeira.

Tudo roda em um binário Go com PostgreSQL. O painel usa `html/template` + HTMX, e a API JSON fica
em `/api/v1`, sobre a mesma camada de serviço. O deploy usa imagem multi-stage e compose.

## Technical Context

**Language/Version**: Go 1.27 (versão do devcontainer).

**Primary Dependencies**:
- Back-end: `jackc/pgx/v5`, `pressly/goose/v3`, `google/uuid`, `golang.org/x/crypto`
  (argon2id), `golang.org/x/time/rate`, `prometheus/client_golang`; o resto é biblioteca
  padrão (`net/http`, `html/template`, `log/slog`, `embed`, `net/netip`).
- Front-end (sem npm): binário standalone do Tailwind CSS v4, HTMX 2, Alpine.js 3, uPlot,
  ícones Lucide inline e fonte Inter local.

**Storage**: PostgreSQL 16 (`postgres:16-alpine` no deploy; TimescaleDB pg16 no devcontainer,
só com recursos padrão).

**Testing**: `go test` com `httptest`; integração com PostgreSQL real via `TEST_DATABASE_URL`
(um banco temporário por pacote); `-race` nos pacotes concorrentes; carga com `vegeta` no épico 8.

**Target Platform**: Linux, contêiner `distroless/static:nonroot`, instância única.

**Project Type**: serviço web (SSR + API JSON) em um único módulo Go.

**Performance Goals**:
- Redirecionamento com p95 < 50 ms no servidor.
- Prévia da página pública com LCP < 2,5 s.

**Constraints**:
- Nenhuma escrita síncrona no caminho do clique.
- Sem `Set-Cookie` público e sem IP puro persistido ou em log.
- Sem recursos de terceiros nas páginas públicas.
- Sem Node/npm no build.

**Scale/Scope**:
- Um administrador e uma organização.
- Dezenas a poucos milhares de links.
- Até ~10⁵ eventos/dia, com retenção de 13 meses.
- Cerca de 12 telas de painel.

Não há itens NEEDS CLARIFICATION; todas as escolhas estão em [research.md](research.md).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Princípio | Verificação no plano | Status |
|---|---|---|
| I. Núcleo primeiro e redirecionamento resiliente | 302 sempre; resolução por cache (R7); enfileiramento não bloqueante (R6); validação `http`/`https` com host; código de 7 `[0-9a-z]` único e não reutilizado (R4) | PASS |
| II. Privacidade por construção | HMAC com `PEPPER` (R8); IPv6 /64; `TRUSTED_PROXIES`; chaves de limitadores derivadas por HMAC; logger sem IP; nenhum cookie público (testado); sessão `HttpOnly/Secure/Lax` + CSRF (R11); aviso informativo e `/privacidade` | PASS |
| III. Eventos assíncronos e contagem correta | Fila + lote + métrica de descarte (R6); `scope` separa `is_unique_url` de `is_unique_target` sob concorrência (R5); crawlers sem evento e robôs fora das métricas (R9); `direct` como "não rastreável"; agregação idempotente e limpeza só de dias agregados (R12) | PASS |
| IV. Isolamento por organização e integridade | `org_id` em todas as tabelas de domínio, inclusive `visitor_seen` (acrescentado); repositórios recebem `orgID` obrigatório; UUID v7 e UTC; migrações goose; soft delete com lixeira; segmentos únicos e reservados | PASS |
| V. Simplicidade operacional e stack | Um binário, um banco, sem Redis; `html/template` + HTMX, sem SPA; Tailwind standalone; assets locais; configuração 12-factor ([contracts/config.md](contracts/config.md)); pacotes criados só quando usados | PASS |
| VI. Teste e validação por épico | Testes por épico; integração com PostgreSQL real; `gofmt`, `vet`, `test` e `-race` em `events`, `redirect` e `cache`; [quickstart.md](quickstart.md) reproduzível | PASS |
| VII. Escopo controlado | Épicos na ordem da seção 13; nada de produtos, CMS, multiusuário ou integrações; divergências registradas em `docs/pendencias.md` sem alterar ADRs | PASS |

**Re-check pós-design**: PASS. As três decisões de modelagem (`visitor_seen.scope`, exclusão de
canal e `products` adiada) não violam a constituição e foram confirmadas pelo responsável
(`docs/pendencias.md`).

## Project Structure

### Documentation (this feature)

```text
specs/001-encurtador-analytics/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── public-routes.md
│   ├── api-v1.md
│   ├── admin-ui.md
│   └── config.md
├── checklists/requirements.md
└── tasks.md             # /speckit-tasks
```

### Source Code (repository root)

```text
cmd/hublinks/            main: subcomandos serve | migrate | maintenance
internal/
├── config/              leitura e validação do ambiente
├── domain/              entidades, políticas, validações (URL, segmento, código)
├── store/               pgx, repositórios por org, migrações embutidas, testes de integração
│   └── testdb/          helper de banco temporário
├── auth/                argon2id, sessões, CSRF, limitador de login
├── httpx/               IP do cliente (TRUSTED_PROXIES), limitador de taxa, logger sem IP, métricas
├── redirect/            handlers /{code} e /{segment}/{code}, cache, detecção de crawler e automação, prévia
├── events/              visitor_id, fila, worker em lote, unicidade
├── stats/               consultas (click_daily + dia corrente)
├── maintenance/         agregação, limpeza, lixeira e scheduler
├── service/             camada de casos de uso compartilhada por api e admin
├── server/              montagem do mux, middlewares e ciclo de vida (serve)
├── assets/              embed de web/static e web/templates, manifest de hash
├── metrics/             registro Prometheus
├── api/                 handlers JSON /api/v1
├── admin/               handlers HTMX /admin e templates do painel
└── web/                 páginas públicas (prévia, /privacidade, 404)
migrations/              SQL goose versionado (embutido via pacote store)
web/
├── assets/              CSS de entrada do Tailwind e tokens
├── templates/           layouts, parciais e ícones SVG
└── static/              CSS compilado com hash, vendor (htmx, alpine, uplot), fontes
deploy/
├── Dockerfile           multi-stage: tailwind → go build → distroless
├── docker-compose.yml   app + postgres
└── .env.example         valores fictícios
docs/                    escopo, pendências, guia de estilo (épico 7), operação (épico 8)
Makefile                 css, test, test-integration, lint, run
```

**Structure Decision**: um módulo Go, seguindo a seção 12 do escopo. Diferenças em relação a ela:
- o ponto de entrada fica em `cmd/hublinks`, porque o binário tem subcomandos além do servidor;
- `auth`, `httpx` e `service` existem porque são compartilhados entre `api`, `admin` e
  `redirect`, conforme a constituição V;
- os templates ficam em `web/templates`, porque o painel e as páginas públicas compartilham
  layout e componentes.

Cada pacote nasce no épico que o usa:

| Épico | Entrega | Pacotes |
|---|---|---|
| 1. Fundação | módulo, config, conexão, migrações, `/healthz`, Dockerfile, compose | `cmd`, `config`, `store`, `deploy` |
| 2. Modelo e CRUD | orgs, users, marketplaces, channels, links, short_codes, lixeira; API CRUD; auth com sessão e CSRF; admin inicial | `domain`, `service`, `api`, `auth` |
| 3. Redirecionamento | rotas, cache, 302 e 404, limitador público | `redirect`, `httpx` |
| 4. Eventos | visitor_id, fila, worker, `visitor_seen`, flags | `events` (`-race`) |
| 5. Bots e prévia | crawlers, prévia OG com aviso, `is_bot`, `/privacidade` | `redirect`, `web` |
| 6. Estatísticas e retenção | API stats, `click_daily`, `aggregated_days`, jobs, fuso, expiração da lixeira | `stats`, `maintenance` |
| 7. Painel | pipeline de CSS, design system, telas, `/admin/ui`, guia de estilo | `admin`, `web/*` |
| 8. Endurecimento | métricas completas, carga, documentação de operação | `httpx`, `docs` |

## Complexity Tracking

Sem violações da constituição a justificar.
