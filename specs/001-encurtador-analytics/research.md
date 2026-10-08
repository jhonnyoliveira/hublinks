# Pesquisa (Fase 0): Encurtador de links com analytics

Contexto fixo (ADRs e constituição, não reavaliados aqui): Go, PostgreSQL, um binário, 302,
eventos assíncronos, `html/template` + HTMX, Tailwind standalone, sem cookies públicos.
Ambiente de desenvolvimento verificado: Go 1.27.2 no devcontainer; PostgreSQL 16
(`timescale/timescaledb:2.16.1-pg16`) no `docker-compose.yml` da raiz.

## R1. Roteamento HTTP

- **Decision**: `net/http.ServeMux` da biblioteca padrão (padrões com método e curingas).
- **Rationale**: rotas literais (`/healthz`, `/privacidade`, `/admin/...`, `/api/v1/...`,
  `/static/...`) têm precedência sobre `/{code}` e `/{segment}/{code}`, sem dependência extra.
- **Alternatives**: chi, echo (dependência sem ganho relevante).

## R2. Acesso ao banco e migrações

- **Decision**: `jackc/pgx/v5` (`pgxpool`) com SQL escrito à mão; migrações SQL versionadas com
  `pressly/goose/v3`, embutidas no binário (`embed`), aplicadas na partida e pelo subcomando
  `hublinks migrate`.
- **Rationale**: SQL explícito facilita garantir o filtro por `org_id` e usar `ON CONFLICT`;
  migrações embutidas mantêm um só artefato.
- **Alternatives**: sqlc (geração adicional; pode ser adotado depois), golang-migrate, ORM (GORM)
  rejeitado por esconder consultas.

## R3. Identificadores

- **Decision**: UUID v7 gerado na aplicação (`google/uuid`) para todas as entidades e eventos;
  `timestamptz` em UTC.
- **Rationale**: ordenação temporal ajuda índices de `click_events`; atende "IDs em UUID".
- **Alternatives**: `gen_random_uuid()` (v4, sem ordenação), bigserial (contraria o escopo).

## R4. Código curto

- **Decision**: 7 caracteres de `[0-9a-z]` via `crypto/rand`; inserção em `short_codes` com
  nova tentativa em conflito (até 5). Códigos iguais a palavras reservadas de 7 letras
  (`healthz`) são descartados na geração. Códigos nunca são apagados, logo nunca reutilizados.
- **Rationale**: 36^7 ≈ 7,8×10^10 combinações; colisões raras.
- **Alternatives**: contador codificado em base36 (previsível, permite enumeração).

## R5. Unicidade (`visitor_seen`) — inconsistência do escopo

- **Problema**: o escopo usa o UUID nulo em `channel_id` tanto para "sem canal" (URL curta)
  quanto para a linha de unicidade por alvo. As duas chaves colidem: um visitante que chegou
  pelo WhatsApp e depois pela URL curta teria `is_unique_url = false` na URL curta.
- **Decision** (confirmada pelo responsável em 2026-10-08): acrescentar a coluna `scope` (`url` | `target`) à chave primária:
  `PK (target_type, target_id, scope, channel_id, visitor_id)`. A linha `scope=target` usa o
  UUID nulo; a linha `scope=url` usa o canal ou o UUID nulo para "sem canal". Acrescentar
  também `org_id` (não está no modelo do escopo; exigido pela constituição para tabelas de
  domínio). Registrado em `docs/pendencias.md` (item 2).
- **Concorrência**: o worker insere as duas linhas com `INSERT ... ON CONFLICT DO NOTHING
  RETURNING`; a linha retornada define a flag. Duas inserções simultâneas da mesma chave
  resultam em uma só linha, portanto em um só único.
- **Robôs**: eventos `is_bot` não tocam `visitor_seen` e gravam flags `false`, para não
  "consumir" a unicidade de um visitante real.
- **Alternatives**: UUID sentinela distinto (`ffffffff-...`) para o alvo (funciona, mas é menos
  legível); duas tabelas separadas (mais código).

## R6. Fila e worker de eventos

- **Decision**: canal Go com buffer (`EVENT_QUEUE_SIZE`, padrão 10 000); envio não bloqueante
  (`select` com `default`) que incrementa `hublinks_events_dropped_total` quando cheio. Worker
  grava em lote a cada 1 s ou 500 eventos, em transação; falha de lote é registrada em log e
  métrica e o lote é descartado após 3 tentativas. `recover` reinicia o worker após pânico.
  No desligamento, drena a fila com prazo.
- **Rationale**: atende ADR-004 e FR-012/FR-016; perda pequena aceita e medida.
- **Alternatives**: tabela de fila no banco (escreve no caminho do clique), Redis (segundo serviço).

## R7. Resolução e cache do redirecionamento

- **Decision**: cache em memória (mapa protegido por `sync.RWMutex`) de `code → destino,
  política efetiva, ativo, org, alvo` e `segment → canal`, com TTL de 5 min e invalidação
  explícita a cada escrita no painel/API (instância única). Ausências também são cacheadas por
  30 s para conter varreduras.
- **Rationale**: meta p95 < 50 ms com o banco fora do caminho na maioria dos acessos.
- **Alternatives**: sem cache (latência depende do banco), Redis (segundo serviço).

## R8. Identificação do visitante e IP do cliente

- **Decision**: `visitor_id = HMAC-SHA256(PEPPER, ip_norm + "|" + user_agent)` (32 bytes,
  `bytea`). IPv4 completo; IPv6 mascarado em /64 (`net/netip`). `X-Forwarded-For` lido da
  direita para a esquerda somente se o par da conexão pertencer a `TRUSTED_PROXIES` (CIDRs),
  parando no primeiro endereço não confiável. O IP puro nunca sai da requisição: limitadores e
  detector de robôs usam `HMAC(PEPPER, "rl|" + ip_norm)` como chave; o logger de acesso não
  registra endereço.
- **Alternatives**: confiar sempre no XFF (falsificável).

## R9. Detecção de robôs

- **Decision**: (1) lista de substrings de user-agent de crawlers conhecidos (WhatsApp,
  TelegramBot, facebookexternalhit, Facebot, Twitterbot, Slackbot, LinkedInBot, Discordbot,
  Googlebot, bingbot, Applebot, Pinterest, redditbot, SkypeUriPreview, vkShare) → preview,
  sem evento; (2) janela deslizante em memória por chave de IP: ao atingir
  `BOT_DISTINCT_CODES` (5) códigos distintos em `BOT_WINDOW` (10 s), o acesso e os seguintes da
  janela recebem `is_bot = true` (clarificação 3). Estruturas limpas periodicamente.
- **Alternatives**: biblioteca externa de detecção (dependência e falsos positivos).

## R10. Limites de taxa e de login

- **Decision**: `golang.org/x/time/rate` por chave de IP (300/min, rajada 60) nas rotas
  públicas → `429` sem evento; login: contador em memória por chave de IP e por e-mail
  normalizado, 5 falhas em 15 min bloqueiam por 15 min (clarificação 4). Mensagem de erro
  genérica.
- **Rationale**: instância única; reinício zera contadores (aceito, ver plano).
- **Alternatives**: tabela no banco (mais escrita), proxy reverso (fora do controle da app).

## R11. Sessão administrativa e CSRF

- **Decision**: tabela `sessions` com hash SHA-256 do token aleatório (32 bytes); cookie
  `hl_session` `HttpOnly; Secure; SameSite=Lax; Path=/`, validade 7 dias deslizante, renovação
  do token no login. CSRF por token sincronizado (campo oculto e cabeçalho `X-CSRF-Token`
  enviado pelo HTMX) somado a `http.CrossOriginProtection`. Senhas com argon2id
  (`golang.org/x/crypto/argon2`, m=64 MiB, t=3, p=2). Cookie `Secure` funciona em
  `http://localhost` nos navegadores atuais.
- **Alternatives**: gorilla/sessions e gorilla/csrf (dependências a mais); JWT (revogação difícil).

## R12. Agregação, limpeza, lixeira e fuso

- **Decision**: scheduler interno (ticker) que roda diariamente às 00:30 em `REPORT_TZ`,
  protegido por `pg_try_advisory_lock`; subcomando `hublinks maintenance run [--day]` para
  execução manual e testes.
  - Agregação: para cada dia passado ainda não agregado (e reprocessamento sob demanda), em uma
    transação: `DELETE` de `click_daily` do dia/org, `INSERT ... SELECT` sem robôs agrupando por
    `(occurred_at AT TIME ZONE REPORT_TZ)::date`, e registro em `aggregated_days`. Idempotente.
  - Limpeza: remove `click_events` com dia < hoje − retenção (395 reais / 30 robôs) **e** dia
    presente em `aggregated_days`.
  - Lixeira: itens com `deleted_at` há mais de `TRASH_RETENTION_DAYS` (30) recebem `purged_at`;
    para links, apaga `visitor_seen` do alvo. Linhas não são apagadas (histórico e nomes).
  - Fuso: `REPORT_TZ` (padrão `America/Sao_Paulo`) gravado em `app_settings` na primeira
    agregação; na partida, divergência com o valor gravado aborta com erro claro.
- **Alternatives**: cron externo (mais um componente); TimescaleDB continuous aggregates (o
  deploy usa PostgreSQL comum).

## R13. Consultas de estatística

- **Decision**: períodos fechados leem `click_daily`; o dia corrente (em `REPORT_TZ`) lê
  `click_events` com `is_bot = false`; a camada `stats` une os dois. Índices:
  `click_events (org_id, occurred_at)`, `(target_id, occurred_at)`; PK de `click_daily`.
- **Alternatives**: sempre de `click_events` (quebra após a limpeza).

## R14. Front-end e assets

- **Decision**: Tailwind CSS v4 pelo binário standalone (versão fixada por `ARG` no Dockerfile
  e no `Makefile`); HTMX 2 e Alpine.js 3 versionados em `web/static/vendor`; uPlot só nas telas
  de métricas; fonte Inter (woff2, `font-display: swap`) local; ícones Lucide como SVG inline
  em templates parciais. Build gera `app.<hash>.css` e um `manifest.json`; o binário embute
  `web/static` e os templates (`embed`) e expõe a função de template `asset`. Arquivos com hash
  servidos com `Cache-Control: public, max-age=31536000, immutable`.
- **Alternatives**: Chart.js (maior que uPlot); CDN (proibido nas páginas públicas).

## R15. Observabilidade

- **Decision**: `log/slog` em JSON; métricas Prometheus (`prometheus/client_golang`) em porta
  separada (`METRICS_ADDR`, padrão `:9091`, não publicada no compose). Evita colisão de
  `/metrics` (7 letras, formato válido de código) com códigos curtos. Métricas: requisições por
  rota/status, latência do redirecionamento (histograma), eventos enfileirados, gravados e
  descartados, tamanho da fila, duração dos jobs. `/healthz` verifica o banco (`SELECT 1`, 1 s).

## R16. Testes

- **Decision**: `go test` com pacote padrão `testing` e `httptest`. Integração usa PostgreSQL
  real via `TEST_DATABASE_URL` (no devcontainer, o serviço `postgres`); o helper cria um banco
  temporário por pacote, aplica as migrações e o remove ao final. Sem `TEST_DATABASE_URL`, os
  testes de integração são pulados com mensagem explícita e o CI deve defini-la. Carga do
  redirecionamento (épico 8) com `vegeta` como ferramenta externa, documentada.
- **Alternatives**: testcontainers-go (exige Docker dentro do devcontainer).

## R17. Deploy

- **Decision**: `deploy/Dockerfile` multi-stage (tailwind → build Go com `CGO_ENABLED=0` →
  `gcr.io/distroless/static-debian12:nonroot`) e `deploy/docker-compose.yml` (app +
  `postgres:16-alpine`, healthcheck, volume). `deploy/.env.example` com valores fictícios.
  O `docker-compose.yml` e o `Dockerfile.dev` da raiz (ambiente de desenvolvimento) são
  preservados. O Redis do ambiente de desenvolvimento não é usado pela aplicação.

## R18. Página de prévia

- **Decision**: o link não tem campo de descrição na fase 1; `og:description` usa texto fixo
  derivado do marketplace ("Disponível em {marketplace}"). `og:image` só quando houver imagem.
  A página inclui aviso de privacidade com script inline mínimo (descarte em `localStorage`)
  e meta `noindex`.

## R19. Produtos na fase 1

- **Decision**: não criar a tabela `products` na fase 1 (nenhum requisito a usa); `target_type`
  aceita apenas `affiliate_link` por `CHECK`, ampliado por migração na fase 2. Confirmado pelo responsável em
  2026-10-08 (`docs/pendencias.md`, item 4).
