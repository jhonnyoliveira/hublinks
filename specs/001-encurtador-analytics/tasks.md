---

description: "Lista de tarefas da Fase 1 — Encurtador de links com analytics"
---

# Tasks: Fase 1 — Encurtador de links com analytics (MVP)

**Input**: Design documents from `specs/001-encurtador-analytics/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: obrigatórios pela constituição (princípio VI). Testes Go ficam ao lado do código
(`*_test.go`). Testes de integração usam PostgreSQL real via `TEST_DATABASE_URL` e o helper
`internal/store/testdb`. Escreva cada teste antes da implementação correspondente e confirme que
ele falha.

**Organization**: tarefas agrupadas por história de usuário (US1–US7 da spec). Cada fase termina
com um checkpoint verificável.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: pode rodar em paralelo (arquivos diferentes, sem dependência de tarefa incompleta)
- **[Story]**: história atendida (US1…US7)

## Convenções obrigatórias em todas as tarefas

- Toda função de repositório de domínio recebe `orgID uuid.UUID` e filtra por `org_id`
  (constituição IV).
- IDs são UUID v7 (`uuid.NewV7`) e datas `time.Time` em UTC.
- Nunca registrar IP em log nem persisti-lo; usar a chave HMAC de `internal/httpx/clientip.go`.
- Respostas públicas nunca chamam `http.SetCookie`.
- Mensagens de interface em português do Brasil.
- Ao final de cada fase: `gofmt -l .`, `go vet ./...` e `go test ./...`. Em pacotes
  concorrentes, também `go test -race`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: inicialização do módulo e das ferramentas

- [ ] T001 Criar o módulo Go `github.com/hublinks/hublinks` com `go 1.27` em `go.mod`, e o
  `cmd/hublinks/main.go` com despacho de subcomandos `serve` (padrão), `migrate` e
  `maintenance` (esses dois respondem "não implementado" até as fases correspondentes).
- [ ] T002 Adicionar as dependências em `go.mod`/`go.sum`: `github.com/jackc/pgx/v5`,
  `github.com/pressly/goose/v3`, `github.com/google/uuid`, `golang.org/x/crypto`,
  `golang.org/x/time` e `github.com/prometheus/client_golang`.
- [ ] T003 [P] Criar o `Makefile` com os alvos `run`, `test`
  (`go test ./...`), `test-race`, `test-ci` (`go test -v ./...` e falha se houver
  `--- SKIP`), `lint` (`gofmt -l . && go vet ./...`) e `css` (binário
  standalone do Tailwind v4 com versão fixada em `TAILWIND_VERSION`, baixado em `bin/`).
  Definir no topo
  `TEST_DATABASE_URL ?= postgresql://hublinks:hublinks@postgres:5432/hublinks` e
  `export TEST_DATABASE_URL`, para que os alvos de teste funcionem no devcontainer sem passo
  manual e o CI possa sobrescrever pelo ambiente. A variável também está definida no serviço
  `dev` do `docker-compose.yml` da raiz.
- [ ] T004 [P] Atualizar o `.gitignore` com `bin/`, `deploy/.env`, `web/static/css/app.*.css` e
  `web/static/manifest.json`, preservando as entradas existentes.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: configuração, banco, migrações, saúde, autenticação, layout base e camada HTTP.
Corresponde ao épico 1 e às bases do épico 2.

**⚠️ CRITICAL**: nenhuma história começa antes deste checkpoint.

### Configuração e banco

- [ ] T005 Escrever testes de `internal/config/config_test.go`:
  - falha nomeando a variável quando faltam `DATABASE_URL`, `PEPPER`, `BASE_URL` ou
    `PRIVACY_CONTACT`;
  - rejeita `PEPPER` com menos de 32 bytes;
  - aplica todos os padrões de `contracts/config.md`;
  - faz o parse de durações (`BOT_WINDOW`, `LOGIN_WINDOW`), de `REPORT_TZ` via
    `time.LoadLocation` e de `TRUSTED_PROXIES` como lista de CIDRs.
- [ ] T006 Implementar `internal/config/config.go` (struct `Config` e `Load(getenv)`) com todas as
  variáveis e padrões de `specs/001-encurtador-analytics/contracts/config.md`. Nenhum segredo vai
  para o `String()`.
- [ ] T007 Implementar `internal/store/db.go`: `Open(ctx, url)` com `pgxpool` (`TimeZone=UTC`
  na conexão) e `Ping` com timeout de 1 s.
- [ ] T008 Implementar `internal/store/migrate.go`: migrações goose embutidas (`//go:embed` de
  `migrations/*.sql`, com o diretório `migrations/` na raiz exposto pelo pacote
  `migrations/embed.go`), mais `Up(ctx, pool)` e `Status`.
- [ ] T009 Criar a migração `migrations/00001_core.sql` com:
  - `organizations(id uuid PK, name text NOT NULL, slug text UNIQUE NOT NULL, created_at)`;
  - `users(id uuid PK, email text UNIQUE NOT NULL CHECK (email = lower(email)), password_hash
    text NOT NULL, created_at, deleted_at)`;
  - `memberships(user_id, org_id, role CHECK (role IN ('admin','maintainer')), PK(user_id,org_id))`;
  - `sessions(token_hash bytea PK, user_id FK, org_id FK, csrf_token text NOT NULL,
    created_at, last_seen_at, expires_at)`;
  - `app_settings(key text PK, value text NOT NULL)`.
- [ ] T010 Implementar o helper `internal/store/testdb/testdb.go`. `New(t)` lê
  `TEST_DATABASE_URL`, faz `t.Skip` com mensagem explícita se ela estiver ausente, cria o banco
  `hl_test_<aleatório>`, aplica as migrações, devolve o pool e remove o banco em `t.Cleanup`.
- [ ] T011 Escrever teste de integração em `internal/store/migrate_test.go`: aplica as migrações
  do zero, verifica que reaplicar não muda nada e que as tabelas de T009 existem.
- [ ] T012 Implementar `hublinks migrate [up|status]` em `cmd/hublinks/migrate.go`.

### Bootstrap, saúde e HTTP base

- [ ] T013 Escrever testes de integração em `internal/store/bootstrap_test.go`:
  - cria a org padrão (`ORG_NAME`) só se nenhuma existir;
  - cria o admin de `ADMIN_EMAIL`/`ADMIN_PASSWORD` só se `users` estiver vazia, com membership
    `admin`;
  - não altera nada quando já existem usuários;
  - rejeita senha com menos de 12 caracteres.
- [ ] T014 Implementar `internal/store/bootstrap.go`, usando o hash de T018.
- [ ] T015 [P] Implementar `internal/httpx/clientip.go`:
  - `ClientIP(r, trusted []netip.Prefix)` percorre `X-Forwarded-For` da direita para a esquerda
    só quando o par da conexão está em `TRUSTED_PROXIES`;
  - `Normalize(ip)` mantém o IPv4 completo e reduz IPv6 a `/64`;
  - `Key(pepper, ip)` retorna `HMAC(PEPPER, "rl|"+ip_norm)`.
- [ ] T016 [P] Escrever `internal/httpx/clientip_test.go`: proxy confiável e não confiável, XFF
  com várias entradas, IPv6 `/64`, IPv4-mapped e cabeçalho forjado ignorado.
- [ ] T017 [P] Implementar `internal/httpx/log.go`: middleware `slog` JSON que registra método,
  rota, status e duração, mas nunca `RemoteAddr`, XFF ou query string. Teste em
  `internal/httpx/log_test.go`: nenhum IP aparece na saída.
- [ ] T018 [P] Implementar `internal/auth/password.go` (argon2id m=64 MiB, t=3, p=2, formato PHC;
  `Hash` e `Verify` com comparação em tempo constante) e o teste `internal/auth/password_test.go`.
- [ ] T019 Implementar `internal/httpx/health.go` (`GET /healthz` → `200 {"status":"ok"}` ou
  `503 {"status":"unavailable","checks":{"database":"error"}}`, sem limite de taxa) e o teste
  `internal/httpx/health_test.go`.
- [ ] T020 Implementar `internal/server/server.go`:
  - monta o `http.ServeMux`, os middlewares de log e recuperação de pânico e
    `http.CrossOriginProtection` nas rotas `/admin` e `/api/v1`;
  - `serve` executa config → migrações → bootstrap → HTTP com shutdown gracioso (`SIGTERM`).
  Ligar em `cmd/hublinks/serve.go`.

### Sessão, CSRF e login

- [ ] T021 Escrever testes de integração em `internal/auth/session_test.go`:
  - login cria sessão com token novo; o cookie `hl_session` tem `HttpOnly; Secure;
    SameSite=Lax; Path=/`;
  - sessão expira após 7 dias sem uso;
  - logout invalida a sessão;
  - POST sem `csrf_token`/`X-CSRF-Token` válido → `403`;
  - rota protegida sem sessão → `303 /admin/login?next=…` (HTML) ou `401` (API).
- [ ] T022 Implementar `internal/auth/session.go`:
  - repositório `sessions` (token de 32 bytes aleatórios, armazenando `token_hash = SHA-256`);
  - middlewares `RequireSession` (injeta `userID` e `orgID` no contexto) e `RequireCSRF`.
- [ ] T023 Implementar `internal/admin/login.go`:
  - `GET/POST /admin/login` com erro genérico "E-mail ou senha inválidos";
  - `POST /admin/logout`;
  - redirecionamento seguro de `next` (somente caminhos `/admin…`).

### Camadas compartilhadas e interface base

- [ ] T024 [P] Implementar `internal/api/respond.go`: helpers JSON, envelope de erro
  `{"error":{"code","message","fields"}}` com os códigos `validation_failed`/422,
  `not_found`/404, `conflict`/409, `unauthorized`/401, `forbidden`/403 e `rate_limited`/429, e
  paginação (`page`, `per_page` padrão 20, máximo 100).
- [ ] T025 [P] Implementar `internal/domain/errors.go`, com os erros de domínio
  (`ErrNotFound`, `ErrConflict`, `ValidationError{Fields map[string]string}`) mapeados pela API e
  pelo painel.
- [ ] T026 [P] Criar os tokens de design e o CSS de entrada em `web/assets/app.css`
  (Tailwind v4 `@theme`):
  - cores semânticas para os modos claro e escuro (`.dark`), com contraste AA;
  - espaçamentos, raios, sombras e tipografia Inter;
  - `@font-face` local com `font-display: swap` apontando para `web/static/fonts/`.
- [ ] T027 [P] Vendorizar em `web/static/vendor/` as versões fixadas de `htmx.min.js` (2.x),
  `alpine.min.js` (3.x) e `uplot.iife.min.js`/`uPlot.min.css`, e a fonte Inter woff2 em
  `web/static/fonts/`, registrando versões e licenças em `web/static/vendor/VERSIONS.md`.
- [ ] T028 Implementar `internal/assets/assets.go` com `embed` de `web/static` e `web/templates`,
  leitura de `web/static/manifest.json` (nome com hash) e a função de template `asset`. Arquivos
  com hash saem com `Cache-Control: public, max-age=31536000, immutable`.
- [ ] T029 Criar o script `scripts/build-css.sh`: compila `web/assets/app.css` com o Tailwind
  standalone minificado, grava `web/static/css/app.<sha256-8>.css` e `web/static/manifest.json`.
  Chamado por `make css`.
- [ ] T030 Criar o layout `web/templates/layouts/admin.html`:
  - navegação lateral fixa em `lg` e menu recolhível no celular (Alpine);
  - alternância de tema (`localStorage`, respeitando `prefers-color-scheme`, sem flash);
  - `hx-headers` com `X-CSRF-Token`;
  - região de toasts.
  Criar também `web/templates/layouts/public.html`, mínimo e sem scripts de terceiros, que inclui
  sempre o componente `privacy_notice` (criado no T031; finalize o layout público após o T031), de modo que prévia, 404 e `/privacidade` herdem
  o aviso (constituição II, FR-022).
- [ ] T031 [P] Criar os componentes parciais em `web/templates/components/`: `button`, `field`
  (rótulo, erro inline, `aria-describedby`), `table`, `card`, `badge`, `modal`
  (`role="dialog"`, foco preso), `toast`, `tabs`, `empty_state`, `skeleton`, `period_picker`,
  `icons/*.svg` (Lucide inline) e `privacy_notice` (aviso informativo, sem pedido de
  consentimento, com link para `/privacidade`, botão "Entendi" e descarte lembrado em
  `localStorage` por script inline mínimo, sem cookie).
- [ ] T032 Criar a tela de login `web/templates/admin/login.html` usando os componentes.

**Checkpoint**: `make css && go run ./cmd/hublinks` sobe; `/healthz` responde 200; o login do
admin inicial funciona; os testes passam.

---

## Phase 3: User Story 1 - Cadastrar link e divulgar por canal (Priority: P1) 🎯 MVP

**Goal**: CRUD de marketplaces, canais e links com lixeira; geração de código; URLs por canal;
redirecionamento 302 e 404.

**Independent Test**: no painel, cadastrar marketplace, canal e link, copiar a URL por canal e
acessá-la. Deve redirecionar `302` ao destino; código ou canal inexistente → `404`.

### Tests for User Story 1

- [ ] T033 [P] [US1] Testes unitários em `internal/domain/validate_test.go`:
  - URL de destino aceita só `http`/`https` com host, "até 2048 caracteres";
  - `image_url` opcional, também `http`/`https`;
  - título "1–200 caracteres";
  - nome de marketplace "1–80 caracteres";
  - nome de canal "1–60 caracteres";
  - segmento `^[a-z0-9][a-z0-9-]{0,19}$`, minúsculo, fora de `api`, `admin`, `p`, `static`,
    `healthz`, `beacon` e `privacidade`;
  - política ∈ {`shorten`,`direct`}.
- [ ] T034 [P] [US1] Testes unitários em `internal/domain/code_test.go`: o código gerado casa
  `^[0-9a-z]{7}$`, nunca é `healthz`, e há distribuição sobre os 36 símbolos.
- [ ] T035 [P] [US1] Testes de integração em `internal/store/catalog_test.go`:
  - CRUD dos três repositórios sempre filtrado por `org_id` (ID de outra org → `ErrNotFound`);
  - unicidade de `(org_id, segment) WHERE purged_at IS NULL`;
  - nome de marketplace único por org entre não excluídos definitivamente;
  - nova tentativa em colisão de `short_codes.code`;
  - política efetiva = override ou a do marketplace.
- [ ] T036 [P] [US1] Testes de integração em `internal/service/trash_test.go`:
  - excluir envia à lixeira (`deleted_at`) e restaurar limpa o campo;
  - marketplace com links fora da lixeira → `ErrConflict` com a lista de links (FR-008c);
  - canal na lixeira mantém o segmento reservado (FR-008d);
  - restaurar link exige marketplace fora da lixeira;
  - item com `purged_at` não é restaurável;
  - excluir canal informa a contagem de links ativos afetados (FR-008c).
- [ ] T037 [P] [US1] Testes HTTP da API em `internal/api/catalog_test.go`, cobrindo cada rota de
  `contracts/api-v1.md` (marketplaces, channels e affiliate-links: POST/GET/PATCH/DELETE/restore):
  - status, envelope de erro e `?trash=true`;
  - campos `urls`, `effective_policy` e `trackable`;
  - link `direct` retorna apenas `{"label":"original","url":destination_url}`;
  - `DELETE /channels/{id}` → `204` mesmo com links ativos, e `GET /channels/{id}` traz
    `active_links_count`;
  - `DELETE /marketplaces/{id}` com links → `409` e a lista de links.
- [ ] T038 [P] [US1] Testes HTTP em `internal/redirect/handler_test.go`:
  - `/{code}` e `/{segment}/{code}` → `302` com `Location`, `Cache-Control: no-store` e sem
    `Set-Cookie`;
  - código com formato inválido, inexistente, inativo, na lixeira ou excluído → `404`;
  - segmento inexistente ou na lixeira → `404`;
  - link `direct` → `302`;
  - edição do destino invalida o cache.

### Implementation for User Story 1

- [ ] T039 [US1] Criar a migração `migrations/00002_catalog.sql`:
  - `marketplaces(id, org_id FK, name text NOT NULL, shorten_policy text CHECK IN
    ('shorten','direct'), created_at, updated_at, deleted_at, purged_at)`, com índice único
    `(org_id, lower(name)) WHERE purged_at IS NULL`;
  - `channels(id, org_id FK, name, segment, created_at, updated_at, deleted_at, purged_at)`,
    com `UNIQUE (org_id, segment) WHERE purged_at IS NULL`;
  - `affiliate_links(id, org_id FK, marketplace_id FK, product_id uuid NULL, title,
    image_url NULL, destination_url, shorten_policy_override NULL CHECK IN
    ('shorten','direct'), active boolean DEFAULT true, created_at, updated_at, deleted_at,
    purged_at)`;
  - `short_codes(code char(7) PK CHECK (code ~ '^[0-9a-z]{7}$'), org_id FK, target_type text
    CHECK (target_type = 'affiliate_link'), target_id uuid, created_at)`, com índice único
    `(target_type, target_id)`.
- [ ] T040 [P] [US1] Implementar `internal/domain/catalog.go`: entidades `Marketplace`,
  `Channel`, `AffiliateLink` e `Policy`, `EffectivePolicy()` e `Trackable()`.
- [ ] T041 [P] [US1] Implementar `internal/domain/validate.go` com as regras de T033 e as
  mensagens em pt-BR.
- [ ] T042 [P] [US1] Implementar `internal/domain/code.go`: `NewCode()` com `crypto/rand`,
  alfabeto `[0-9a-z]`, 7 caracteres, descartando palavras reservadas.
- [ ] T043 [US1] Implementar os repositórios em `internal/store/marketplaces.go`,
  `internal/store/channels.go`, `internal/store/links.go` e `internal/store/codes.go`:
  - listagem com `q`, filtros, ordenação e paginação;
  - `trash` (lixeira), `SoftDelete`, `Restore` e `CountActiveLinksByMarketplace`;
  - criação do link e do código na mesma transação, com até 5 tentativas em colisão.
- [ ] T044 [US1] Implementar `internal/service/catalog.go`, com os casos de uso compartilhados
  por API e painel: validação, regras de lixeira (FR-008a–d), montagem de `urls` a partir de
  `BASE_URL` e dos canais ativos, e invalidação do cache de resolução a cada escrita.
- [ ] T045 [US1] Implementar os handlers da API em `internal/api/marketplaces.go`,
  `internal/api/channels.go` e `internal/api/links.go`, conforme `contracts/api-v1.md`, sob
  `RequireSession` + `RequireCSRF`.
- [ ] T046 [US1] Implementar `internal/redirect/cache.go`:
  - mapas `code → Resolved{OrgID, TargetType, TargetID, Destination, Policy, Active}` e
    `(orgID, segment) → channelID`, sob `sync.RWMutex`;
  - TTL de 5 min, cache negativo de 30 s, `Invalidate(code)` e `InvalidateAll()`;
  - teste `internal/redirect/cache_test.go`, executado com `-race`.
- [ ] T047 [US1] Implementar `internal/redirect/handler.go` para `GET /{code}` e
  `GET /{segment}/{code}`:
  - valida o formato antes do cache;
  - `404` HTML via `web/templates/public/404.html`;
  - `302` com `Cache-Control: no-store` e `Referrer-Policy: no-referrer-when-downgrade`;
  - deixa um ponto de extensão `Recorder` (no-op até a US2).
- [ ] T048 [US1] Implementar as telas de marketplaces em `internal/admin/marketplaces.go` e
  `web/templates/admin/marketplaces/*.html`:
  - listagem HTMX com busca e paginação e CRUD em modal;
  - confirmação de exclusão; o `409` lista os links que bloqueiam.
- [ ] T049 [US1] Implementar as telas de canais em `internal/admin/channels.go` e
  `web/templates/admin/channels/*.html`, com listagem, CRUD em modal e confirmação de exclusão
  que exibe `active_links_count` ("N links terão a URL deste canal invalidada").
- [ ] T050 [US1] Implementar as telas de links em `internal/admin/links.go` e
  `web/templates/admin/links/{index,form,show}.html`:
  - listagem com busca, filtro por marketplace e paginação;
  - formulário com validação inline;
  - detalhe com a URL curta e uma por canal, cada uma com botão "Copiar" (Alpine +
    `navigator.clipboard`) e toast de sucesso;
  - política `direct` mostra só a URL original e o selo "não rastreável";
  - controle "Ativo" (interruptor acessível, `role="switch"`) no formulário e alternância rápida
    via HTMX na listagem, com selo "Inativo"; um link inativo responde `404` (FR-005, FR-011).
- [ ] T051 [US1] Implementar a lixeira em `internal/admin/trash.go` e
  `web/templates/admin/trash.html` (itens com dias restantes até `TRASH_RETENTION_DAYS` e ação
  de restaurar), mais as rotas `delete`/`restore` dos três CRUDs.
- [ ] T052 [US1] Registrar as rotas de catálogo, painel e redirecionamento em
  `internal/server/server.go`, garantindo a precedência das rotas literais (`/healthz`,
  `/privacidade`, `/admin/`, `/api/v1/`, `/static/`) sobre `/{code}` e `/{segment}/{code}`, com
  teste em `internal/server/routes_test.go`.

**Checkpoint**: História 1 completa. Os cenários 1–5 da spec passam e o passo 3 do
`quickstart.md` funciona.

---

## Phase 4: User Story 2 - Ver cliques totais e únicos (Priority: P1)

**Goal**: registro assíncrono de eventos, unicidade permanente (URL e alvo), estatísticas por
link, canal e dia, e dashboard com seletor de período.

**Independent Test**: acessar o link duas vezes com o mesmo IP/UA (2 cliques, 1 único), mudar o
UA (novo único) e acessar por dois canais (alcance 1, um único por canal). Conferir no painel e
em `/api/v1/stats/*`.

### Tests for User Story 2

- [ ] T053 [P] [US2] Testes unitários em `internal/events/visitor_test.go`:
  `VisitorID(pepper, ip, ua)` é determinístico, retorna 32 bytes, muda com o UA e coincide para
  IPv6 do mesmo `/64`.
- [ ] T054 [P] [US2] Testes em `internal/events/queue_test.go`, executados com `-race`:
  - `Enqueue` nunca bloqueia;
  - com a fila cheia, descarta e incrementa `hublinks_events_dropped_total`;
  - worker parado ou em pânico não afeta o `Enqueue`;
  - o flush acontece a cada `EVENT_BATCH_SIZE` ou `EVENT_FLUSH_INTERVAL`;
  - o shutdown drena a fila.
- [ ] T055 [P] [US2] Testes de integração em `internal/events/writer_test.go`:
  - sem canal e com canal: `is_unique_url` e `is_unique_target` corretos;
  - visitante via `wapp` e depois pela URL curta → `is_unique_url=true` e
    `is_unique_target=false` (R5);
  - 20 goroutines gravando o mesmo visitante e alvo → exatamente 1 único (`-race`);
  - evento `is_bot` não insere em `visitor_seen` e grava as flags `false`.
- [ ] T056 [P] [US2] Testes HTTP em `internal/redirect/record_test.go`:
  - acesso real a link `shorten` enfileira um evento com `occurred_at`, `channel_id`, `referer`
    (truncado em 1024) e `user_agent` (truncado em 512);
  - link `direct` não enfileira nada;
  - com o worker parado, continua `302`.
- [ ] T057 [P] [US2] Testes de integração em `internal/stats/stats_test.go`:
  - semeia `click_events` e `click_daily`;
  - períodos `7d`/`30d`/`90d`/personalizado; `from > to` ou mais de 366 dias → erro;
  - o dia corrente em `REPORT_TZ` vem de `click_events`, e os dias fechados de `click_daily`;
  - robôs excluídos;
  - por canal, `unique = SUM(unique_url)`; por link, `unique = SUM(unique_target)`;
  - link `direct` → `trackable:false`, contadores `null` e `history` preservado;
  - acesso às 23h30 de Brasília conta no próprio dia;
  - todas as consultas restritas à org.
- [ ] T058 [P] [US2] Testes HTTP em `internal/api/stats_test.go` para `GET /stats/links`,
  `/stats/links/{id}`, `/stats/channels` e `/stats/summary`, no formato de
  `contracts/api-v1.md`.

### Implementation for User Story 2

- [ ] T059 [US2] Criar a migração `migrations/00003_events.sql`:
  - `click_events` com as colunas de `data-model.md` (`visitor_id bytea` com
    `CHECK (octet_length(visitor_id)=32)`, `event_type CHECK IN ('click')`, `referer` e
    `user_agent` text), índices `(org_id, occurred_at)` e `(target_id, occurred_at)`, sem FKs;
  - `visitor_seen(target_type, target_id, scope CHECK IN ('url','target'), channel_id uuid NOT
    NULL, visitor_id bytea, org_id uuid NOT NULL, first_seen_at)` com
    `PK(target_type, target_id, scope, channel_id, visitor_id)`;
  - `click_daily` com `PK(org_id, day, event_type, target_type, target_id, channel_id)` e
    contadores `integer NOT NULL DEFAULT 0`;
  - `aggregated_days(org_id, day, aggregated_at, PK(org_id, day))`.
- [ ] T060 [P] [US2] Implementar `internal/events/visitor.go`:
  `VisitorID = HMAC-SHA256(PEPPER, ip_norm + "|" + ua)`, usando `httpx.Normalize`.
- [ ] T061 [P] [US2] Implementar `internal/metrics/metrics.go` com o registro Prometheus e os
  contadores e histogramas de R15: `hublinks_events_enqueued_total`,
  `hublinks_events_dropped_total`, `hublinks_events_written_total`,
  `hublinks_events_write_errors_total` e `hublinks_event_queue_length`.
- [ ] T062 [US2] Implementar `internal/events/queue.go`:
  - canal com buffer `EVENT_QUEUE_SIZE` e `Enqueue` não bloqueante (`select … default`);
  - worker com lote de `EVENT_BATCH_SIZE`/`EVENT_FLUSH_INTERVAL`, 3 tentativas por lote,
    `recover` com reinício;
  - drenagem no shutdown com prazo.
- [ ] T063 [US2] Implementar `internal/events/writer.go`: para cada evento real, em transação,
  `INSERT INTO visitor_seen … ON CONFLICT DO NOTHING RETURNING`, uma vez com `scope='url'`
  (canal ou o UUID nulo) e outra com `scope='target'` (UUID nulo), define as flags e grava o lote
  com `CopyFrom` em `click_events`. Robôs gravam direto, sem `visitor_seen`.
- [ ] T064 [US2] Implementar `internal/redirect/record.go` (o `Recorder` real):
  - monta o evento (`occurred_at` = agora em UTC, `visitor_id`, canal, referer e UA truncados);
  - enfileira só para a política efetiva `shorten`;
  - liga o worker em `internal/server/server.go`.
- [ ] T065 [US2] Implementar `internal/stats/period.go` (parse de `period`/`from`/`to` em
  `REPORT_TZ`, com limite de 366 dias) e `internal/stats/stats.go` (consultas da união
  `click_daily` + dia corrente de `click_events`), com `orgID` obrigatório.
- [ ] T066 [US2] Implementar `internal/api/stats.go`, com as quatro rotas de estatística de
  `contracts/api-v1.md`.
- [ ] T067 [US2] Implementar o dashboard em `internal/admin/dashboard.go` e
  `web/templates/admin/dashboard.html`:
  - cartões de cliques, únicos e links ativos;
  - série diária em uPlot (script carregado só nesta tela e no detalhe);
  - top 5 links, comparação entre canais e seletor de período com `hx-push-url`;
  - aviso das limitações da contagem de únicos (FR-026).
- [ ] T068 [US2] Acrescentar estatísticas às telas de links:
  - cliques e únicos na listagem (`web/templates/admin/links/index.html`);
  - tabela por canal e série diária no detalhe (`web/templates/admin/links/show.html`);
  - seletor de período;
  - link `direct` com o selo "não rastreável" e nunca zero, mostrando o histórico anterior
    quando houver.
- [ ] T069 [US2] Expor as métricas em um servidor separado em `METRICS_ADDR` (vazio desativa) no
  `internal/server/server.go`.

**Checkpoint**: Histórias 1 e 2 completas; o passo 4 do `quickstart.md` funciona.

---

## Phase 5: User Story 3 - Prévia de compartilhamento sem distorcer métricas (Priority: P2)

**Goal**: crawlers recebem a prévia Open Graph sem evento; automação é marcada como robô; há
aviso de privacidade e `/privacidade`.

**Independent Test**: `curl -A "WhatsApp/2.23"` retorna `200` com as metatags e os contadores
não mudam; 5 códigos distintos em 10 s pelo mesmo IP → o 5º é `is_bot`.

### Tests for User Story 3

- [ ] T070 [P] [US3] Testes em `internal/redirect/bots_test.go`:
  - detecção dos UAs de R9, sem diferenciar maiúsculas;
  - UA comum não é crawler;
  - janela deslizante: com 4 códigos distintos em 10 s, o 5º e os seguintes da janela são
    `is_bot` e os 4 anteriores não;
  - o mesmo código repetido não conta como distinto;
  - após a janela, a contagem é zerada;
  - limites lidos de `BOT_DISTINCT_CODES`/`BOT_WINDOW`;
  - executar com `-race`.
- [ ] T071 [P] [US3] Testes HTTP em `internal/redirect/preview_test.go`:
  - crawler → `200`, sem `Location` e sem evento enfileirado;
  - `og:title`, `og:description` ("Disponível em {marketplace}"), `og:url` absoluta,
    `og:type=website` e `twitter:card` (`summary_large_image` com imagem, `summary` sem);
  - `og:image` só quando houver imagem;
  - `<meta name="robots" content="noindex">`;
  - aviso com link para `/privacidade`;
  - sem `Set-Cookie` e sem URLs de terceiros.
- [ ] T072 [P] [US3] Teste HTTP em `internal/web/privacy_test.go`: `GET /privacidade` → `200`
  com os prazos de `EVENTS_RETENTION_DAYS`, `BOT_EVENTS_RETENTION_DAYS` e
  `TRASH_RETENTION_DAYS`, o `PRIVACY_CONTACT`, a menção ao identificador anônimo sem cookies e o
  aviso; sem `Set-Cookie`.

### Implementation for User Story 3

- [ ] T073 [P] [US3] Implementar `internal/redirect/bots.go`: `IsCrawler(ua)` com a lista de R9
  e o detector de automação (janela deslizante em memória por chave `httpx.Key`, com limpeza
  periódica de entradas expiradas).
- [ ] T074 [US3] Integrar no `internal/redirect/handler.go`: crawler → prévia sem evento;
  automação → `302` e evento com `is_bot=true`.
- [ ] T075 [P] [US3] Revisar o texto do aviso em `web/templates/components/privacy_notice.html`
  (criado no T031) com a finalidade e o link para `/privacidade`, e garantir que
  `web/templates/public/404.html` usa o layout `public` e, portanto, exibe o aviso.
- [ ] T076 [US3] Criar `web/templates/public/preview.html` (layout `public`, mobile first:
  imagem com dimensões declaradas, título, botão principal para o destino e aviso) e
  `internal/redirect/preview.go`.
- [ ] T077 [US3] Implementar `internal/web/privacy.go` e `web/templates/public/privacidade.html`,
  com o conteúdo de FR-023 a partir da configuração.

**Checkpoint**: História 3 completa; o passo 5 do `quickstart.md` funciona.

---

## Phase 6: User Story 4 - Operar no celular com modo claro e escuro (Priority: P2)

**Goal**: painel mobile first, acessível, nos dois temas, com estados vazios, proteção contra
perda de dados e guia de componentes.

**Independent Test**: com largura de 375 px, percorrer login → novo link → detalhe → dashboard,
nos dois temas, sem rolagem horizontal e com o tema lembrado.

### Tests for User Story 4

- [ ] T078 [P] [US4] Testes de templates em `internal/admin/render_test.go`:
  - cada listagem vazia renderiza `empty_state` com o próximo passo (ex.: "Cadastre seu
    primeiro link");
  - todo `<input>` tem `<label>` associado e toda `<img>` tem `alt`;
  - `<html lang="pt-BR">`;
  - o botão de tema tem `aria-label`;
  - `/admin/ui` → `404` fora de `APP_ENV=development`.

### Implementation for User Story 4

- [ ] T079 [P] [US4] Tornar as tabelas responsivas em `web/templates/components/table.html`:
  colunas secundárias com `hidden md:table-cell` e linhas em formato de cartão abaixo de `md`.
  Aplicar nas listagens de links, marketplaces, canais e lixeira.
- [ ] T080 [P] [US4] Acrescentar estados vazios com orientação às listagens e ao dashboard em
  `web/templates/admin/**`.
- [ ] T081 [P] [US4] Implementar a proteção contra perda de dados não salvos em
  `web/static/js/unsaved.js` (Alpine: marca o formulário alterado e usa `beforeunload` e
  `htmx:beforeRequest` em navegação) e aplicar em `web/templates/admin/links/form.html`.
- [ ] T082 [P] [US4] Garantir foco visível (`focus-visible`), navegação por teclado no menu, no
  modal e nas abas, e contraste AA dos tokens nos dois temas, em `web/assets/app.css` e
  `web/templates/components/*`.
- [ ] T083 [US4] Implementar `GET /admin/ui` em `internal/admin/ui.go` e
  `web/templates/admin/ui.html`, mostrando todos os componentes nos dois temas, registrado só
  com `APP_ENV=development`.
- [ ] T084 [US4] Escrever o guia de estilo `docs/guia-de-estilo.md` (tokens, componentes, quando
  usar cada um, padrões CRUD da seção 11.1).

**Checkpoint**: História 4 completa; o passo 9 do `quickstart.md` funciona.

---

## Phase 7: User Story 5 - Acesso restrito e política de privacidade (Priority: P2)

**Goal**: bloqueio de login após falhas, limite de taxa nas rotas públicas e garantia de que
nenhuma resposta pública define cookies.

**Independent Test**: 5 senhas erradas → a 6ª tentativa, mesmo correta, é recusada por 15 min;
301 requisições em 1 min do mesmo IP → `429`; nenhuma resposta pública tem `Set-Cookie`.

### Tests for User Story 5

- [ ] T085 [P] [US5] Testes em `internal/auth/limiter_test.go` (relógio injetável, `-race`):
  - 5 falhas em 15 min por chave de IP **ou** por e-mail normalizado bloqueiam por 15 min;
  - a 6ª tentativa com a senha correta é recusada;
  - após o bloqueio, volta a aceitar;
  - sucesso antes do limite zera o contador do e-mail;
  - a mensagem não revela se o e-mail existe.
- [ ] T086 [P] [US5] Testes em `internal/httpx/ratelimit_test.go`: 300/min por chave de IP; o
  excedente recebe `429` com `Retry-After` e nenhum evento é enfileirado; `/healthz` fica isento.
  Executar com `-race`.
- [ ] T087 [P] [US5] Teste de varredura em `internal/server/nocookie_test.go`: requisições a
  `/{code}`, `/{segment}/{code}`, prévia de crawler, `404`, `429` e `/privacidade` (com e sem
  sessão de admin) nunca trazem `Set-Cookie`, e toda resposta HTML entre elas contém o link para
  `/privacidade`.

### Implementation for User Story 5

- [ ] T088 [US5] Implementar `internal/auth/limiter.go` (contadores em memória por chave
  `httpx.Key` e por e-mail, com `LOGIN_MAX_FAILURES`, `LOGIN_WINDOW` e `LOGIN_LOCKOUT`) e
  integrá-lo ao `internal/admin/login.go`, com mensagem informando o prazo do bloqueio.
- [ ] T089 [US5] Implementar `internal/httpx/ratelimit.go` (`golang.org/x/time/rate` por chave
  de IP, `PUBLIC_RATE_LIMIT`/min com rajada de 60 e limpeza de chaves ociosas) e aplicá-lo às
  rotas públicas em `internal/server/server.go`.

**Checkpoint**: História 5 completa.

---

## Phase 8: User Story 6 - Histórico preservado com retenção automática (Priority: P3)

**Goal**: agregação diária idempotente no fuso de relatório, limpeza só de dias agregados e
expiração da lixeira com remoção de `visitor_seen`.

**Independent Test**: semear eventos antigos, rodar `hublinks maintenance run` e comparar as
contagens (idênticas); reexecutar (sem mudança); um link na lixeira há 31 dias fica com
`purged_at` e sem `visitor_seen`.

### Tests for User Story 6

- [ ] T090 [P] [US6] Testes de integração em `internal/maintenance/aggregate_test.go`:
  - agrega por `(occurred_at AT TIME ZONE REPORT_TZ)::date`, ignorando robôs;
  - reexecutar o mesmo dia produz valores idênticos;
  - todos os dias passados não agregados são processados (recuperação de atraso);
  - o dia corrente não é agregado;
  - `aggregated_days` é registrado.
- [ ] T091 [P] [US6] Testes de integração em `internal/maintenance/cleanup_test.go`:
  - remove eventos reais com mais de 395 dias e de robôs com mais de 30, só em dias presentes em
    `aggregated_days`;
  - dias não agregados são preservados;
  - o corte é por dia em `REPORT_TZ`: eventos cujo dia local é `< hoje(REPORT_TZ) − N` são
    removidos; um evento às 23h30 (horário de Brasília) do dia-limite é preservado;
  - as contagens totais e únicas de `stats` são idênticas antes e depois (SC-006).
- [ ] T092 [P] [US6] Testes de integração em `internal/maintenance/trash_test.go`:
  - item com `deleted_at` há mais de `TRASH_RETENTION_DAYS` recebe `purged_at`;
  - para links, apaga `visitor_seen` do alvo;
  - `click_daily` permanece e entra nos totais;
  - o código continua respondendo `404` e nunca é reutilizado;
  - restaurar antes do prazo mantém `visitor_seen` (visitante antigo não vira único novo).
- [ ] T093 [P] [US6] Teste de integração em `internal/maintenance/timezone_test.go`:
  - a primeira agregação grava `report_timezone` em `app_settings`;
  - partir com outro `REPORT_TZ` aborta com erro claro.

### Implementation for User Story 6

- [ ] T094 [US6] Implementar `internal/maintenance/aggregate.go`. Por dia e org, em transação:
  `DELETE` de `click_daily`, `INSERT … SELECT` com `SUM(is_unique_url::int)` e
  `SUM(is_unique_target::int)` (com `channel_id` nulo convertido no UUID nulo) e upsert em
  `aggregated_days`.
- [ ] T095 [US6] Implementar `internal/maintenance/cleanup.go`: remove `click_events` com
  `(occurred_at AT TIME ZONE REPORT_TZ)::date < hoje(REPORT_TZ) − EVENTS_RETENTION_DAYS` (reais)
  ou `− BOT_EVENTS_RETENTION_DAYS` (`is_bot`), sempre com `EXISTS (aggregated_days)` do mesmo
  dia e org, em lotes de 10 000 linhas.
- [ ] T096 [US6] Implementar `internal/maintenance/trash.go`: expiração da lixeira, com
  `purged_at` e `DELETE FROM visitor_seen` para links, e invalidação do cache de resolução.
- [ ] T097 [US6] Implementar `internal/maintenance/timezone.go` (fixa e verifica `REPORT_TZ`) e
  chamá-lo na partida em `internal/server/server.go`.
- [ ] T098 [US6] Implementar `internal/maintenance/scheduler.go`: execução diária em
  `MAINTENANCE_AT` no `REPORT_TZ`, protegida por `pg_try_advisory_lock`, com métrica de duração
  e falhas; ligado em `serve`.
- [ ] T099 [US6] Implementar `hublinks maintenance run [--day YYYY-MM-DD]` em
  `cmd/hublinks/maintenance.go` (agrega, limpa e expira uma vez; com `--day`, reagrega aquele dia).

**Checkpoint**: História 6 completa; o passo 8 do `quickstart.md` funciona.

---

## Phase 9: User Story 7 - Subir o serviço com um comando (Priority: P3)

**Goal**: imagem multi-stage e compose (app + PostgreSQL) a partir de variáveis de ambiente.

**Independent Test**: `docker compose -f deploy/docker-compose.yml --env-file deploy/.env up -d
--build` e `/healthz` responde `200` em até 2 min; sem `PEPPER`, sai com erro que cita a variável.

### Implementation for User Story 7

- [ ] T100 [P] [US7] Criar o `deploy/Dockerfile` multi-stage:
  1. estágio `css`: baixa o Tailwind standalone pela versão do `ARG` e roda
     `scripts/build-css.sh`;
  2. estágio `build`: `golang:1.27`, `CGO_ENABLED=0`, `-trimpath -ldflags "-s -w"`;
  3. estágio final: `gcr.io/distroless/static-debian12:nonroot`, `EXPOSE 8080`,
     `ENTRYPOINT ["/hublinks"]`, `CMD ["serve"]`.
  Criar também o `.dockerignore` na raiz.
- [ ] T101 [P] [US7] Criar o `deploy/docker-compose.yml`:
  - serviço `app` (build `deploy/Dockerfile`, contexto `..`, `env_file: .env`, porta `8080`,
    `depends_on` de `db` saudável);
  - serviço `db` (`postgres:16-alpine` com volume e healthcheck `pg_isready`);
  - sem publicar `9091`.
  Não alterar o `docker-compose.yml` nem o `Dockerfile.dev` da raiz.
- [ ] T102 [P] [US7] Criar o `deploy/.env.example` apenas com valores fictícios para todas as
  variáveis de `contracts/config.md`, com comentário de que `PEPPER` nunca deve ser rotacionado.
- [ ] T103 [US7] Escrever o `README.md` na raiz: requisitos, desenvolvimento no devcontainer
  (`make css`, `make run`, `make test` com `TEST_DATABASE_URL`), deploy pelo compose e a tabela
  de variáveis (link para `specs/001-encurtador-analytics/contracts/config.md`).
- [ ] T104 [US7] Validar o build da imagem e a subida pelo compose, conforme o passo 2 do
  `quickstart.md`, registrando o resultado no PR.

**Checkpoint**: História 7 completa.

---

## Phase 10: Polish & Cross-Cutting Concerns (épico 8 — endurecimento)

- [ ] T105 [P] Completar as métricas HTTP em `internal/httpx/metrics.go`: requisições por rota e
  status e histograma `hublinks_redirect_duration_seconds`.
- [ ] T106 [P] Criar o teste de carga do redirecionamento em `scripts/loadtest.sh` (`vegeta`
  contra `/{code}` em cache, 500 req/s por 60 s) e registrar em `docs/operacao.md` o p95 medido
  no servidor (meta < 50 ms, SC-003). Registrar também, a partir de
  `hublinks_events_enqueued_total` e `hublinks_events_dropped_total`, a taxa de descarte durante
  a carga, e verificar que fica abaixo de 0,1% (SC-011).
- [ ] T107 [P] Escrever `docs/operacao.md`: variáveis, backup do banco, cuidado com `PEPPER` e
  `REPORT_TZ`, jobs de manutenção, métricas e alertas sugeridos (descartes, falhas de job) e
  carga.
- [ ] T108 Auditar a privacidade: buscar por `RemoteAddr`, `X-Forwarded-For` e `SetCookie` em
  `internal/**` e confirmar que só ocorrem em `httpx/clientip.go` e `auth/session.go`; executar o
  passo 10 do `quickstart.md`.
- [ ] T109 Executar `gofmt -l .`, `go vet ./...`, `go test ./...` e
  `go test -race ./internal/events/... ./internal/redirect/... ./internal/auth/...
  ./internal/httpx/... ./internal/maintenance/... ./internal/metrics/...` com
  `TEST_DATABASE_URL` definida, e o `quickstart.md` completo. Confirmar que nenhum teste de
  integração foi pulado (`make test-ci`: `go test -v ./... 2>&1 | grep -E '^\s*--- SKIP'` deve
  retornar vazio). Registrar no PR o resultado do passo 9 do `quickstart.md` (375 px, dois
  temas, teclado e contraste conferido no DevTools) e o tempo cronometrado do passo 3, com meta
  abaixo de 3 min (SC-001, SC-009).
- [ ] T110 Atualizar a seção "Momento atual e próximo trabalho" do `AGENTS.md` e as pendências
  ainda abertas em `docs/pendencias.md`.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (1)** → **Foundational (2)** → histórias.
- **US1 (3)** depende só da fase 2. É o MVP.
- **US2 (4)** depende da US1 (links, códigos e o `Recorder` do handler).
- **US3 (5)** depende da US1 (handler). Integra-se ao enfileiramento da US2 para `is_bot`; sem a
  US2, a marcação fica pronta, mas não é gravada.
- **US4 (6)** depende das telas da US1; o polimento do dashboard depende da US2.
- **US5 (7)** depende da fase 2 (login) e da US1 (rotas públicas). Pode correr em paralelo com
  as US2–US4.
- **US6 (8)** depende da US2 (tabelas de eventos e `stats`).
- **US7 (9)** depende só da fase 2 e pode ser antecipada em paralelo com qualquer história. É
  revalidada ao final.
- **Polish (10)** depende de todas.

Ordem recomendada, alinhada aos épicos do escopo:
fase 2 + US7 (épico 1) → US1 (épicos 2 e 3) → US2 (épico 4) → US3 (épico 5) → US6 (épico 6)
→ US4 (épico 7) → US5 + Polish (épico 8).

### Within Each User Story

Testes (falhando) → migração → domínio → repositórios → serviço → API/handlers → telas.

### Parallel Opportunities

- Fase 2: T015–T018 e T024–T027 em paralelo.
- US1: T033–T038 (testes) em paralelo; depois T040–T042 em paralelo.
- US2: T053–T058 em paralelo; T060 e T061 em paralelo.
- US3: T070–T072 em paralelo; T073 e T075 em paralelo.
- US4: T079–T082 em paralelo.
- US5: T085–T087 em paralelo, e a fase inteira em paralelo com as US2–US4.
- US7: T100–T102 em paralelo, logo após a fase 2.

## Parallel Example: User Story 1

```bash
# Testes primeiro, em paralelo:
Task: "T033 Testes de validação em internal/domain/validate_test.go"
Task: "T034 Testes de código em internal/domain/code_test.go"
Task: "T035 Testes de repositórios em internal/store/catalog_test.go"
Task: "T038 Testes do redirecionamento em internal/redirect/handler_test.go"

# Depois, domínio em paralelo:
Task: "T040 Entidades em internal/domain/catalog.go"
Task: "T041 Validações em internal/domain/validate.go"
Task: "T042 Gerador de código em internal/domain/code.go"
```

## Implementation Strategy

### MVP First (User Story 1)

1. Fases 1 e 2, validando `/healthz` e o login.
2. US1, validando o cadastro e o redirecionamento `302`/`404`.
3. **PARAR e validar**: links divulgáveis funcionando (ainda sem métricas).

### Incremental Delivery

Cada fase fecha um épico do escopo com testes passando. Atualize o `AGENTS.md` ao concluir cada
épico. A fase 2 do produto (CMS) só começa após todos os critérios de aceite da fase 1.

### Notes

- Decisões de modelagem confirmadas em `docs/pendencias.md` (itens 2–4): `visitor_seen.scope`,
  exclusão de canal (FR-008c) e `products` adiada para a fase 2.
- Não implementar nada fora de escopo (integração com marketplaces, preço, ranking, avaliações,
  produtos/CMS).
