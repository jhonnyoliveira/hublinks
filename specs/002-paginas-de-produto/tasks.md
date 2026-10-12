---

description: "Lista de tarefas da Fase 2 — Páginas de produto (CMS)"
---

# Tasks: Fase 2 — Páginas de produto (CMS)

**Input**: Design documents from `specs/002-paginas-de-produto/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: obrigatórios pela constituição (princípio VI). Testes Go ficam ao lado do código
(`*_test.go`). Testes de integração usam PostgreSQL real via `TEST_DATABASE_URL` e o helper
`internal/store/testdb`. Escreva cada teste antes da implementação correspondente e confirme que
ele falha.

**Organization**: tarefas agrupadas por história de usuário (US1–US6 da spec). Cada fase termina
com um checkpoint verificável. A fase 1 do produto já foi aceita; nada aqui altera suas decisões
(links `direct` continuam sem métrica).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: pode rodar em paralelo (arquivos diferentes, sem dependência de tarefa incompleta)
- **[Story]**: história atendida (US1…US6)

## Como executar este arquivo (leia antes de começar)

1. Leia, nesta ordem: `AGENTS.md`, `.specify/memory/constitution.md`, `specs/002-paginas-de-produto/spec.md`,
   `plan.md`, `research.md`, `data-model.md`, `contracts/*.md` e `quickstart.md`. Cada tarefa cita o
   trecho de que depende.
2. Execute as tarefas na ordem dos IDs, respeitando as fases; só inicie uma história depois do
   checkpoint da fase anterior. Marque `[x]` na tarefa quando ela estiver concluída **e** seus testes
   passarem.
3. Teste primeiro: onde a tarefa diz "escrever os testes", escreva-os, rode e veja falhar antes de
   implementar. Não apague nem enfraqueça um teste para fazê-lo passar.
4. Ao terminar cada fase rode `gofmt -l .`, `go vet ./...` e `go test ./...` (com `TEST_DATABASE_URL`
   definida; sem ela os testes de integração são pulados em silêncio). Use `make test-ci` para falhar
   se algum for pulado. Em cache, fila e beacon rode também `go test -race`.
5. Se a tarefa for ambígua, ou o código existente contradizer o que ela descreve, **não decida**:
   registre a dúvida em `docs/pendencias.md` e pergunte ao responsável. Nunca altere ADRs, a spec ou
   decisões já registradas por conta própria.
6. Sem Docker, navegador ou rede neste ambiente? Não marque a tarefa como verificada: registre o que
   ficou pendente em `docs/verificacao.md` e avise o responsável. A variável de teste é
   `TEST_DATABASE_URL=postgresql://hublinks:hublinks@postgres:5432/postgres` (ver
   `docs/desenvolvimento.md`).
7. Não faça nada fora das tarefas (preços, ranking, veredito, avaliações, comparador, sitemap, site
   público e integrações com marketplaces estão fora de escopo) e não declare verificado o que não foi
   executado.

## Glossário (um termo, um sentido)

- **endereço amigável** = `slug` do produto, em `/p/{slug}`; **código curto** = os 7 caracteres de
  `short_codes`, em `/{código}` e `/{canal}/{código}`.
- **visita** = evento `view` (abertura da página, registrada pelo beacon); **clique** = evento `click`
  (acesso a um código curto, de link ou de produto).
- **interface programática** da spec = API JSON em `/api/v1`.
- **link `direct`** = link de política efetiva `direct`: "não rastreável", sem métrica.

## Convenções obrigatórias em todas as tarefas

- Toda função de repositório de domínio recebe `orgID uuid.UUID` e filtra por `org_id`
  (constituição IV).
- IDs são UUID v7 (`uuid.NewV7`), exceto `media.id` (v4 aleatório, research R2); datas `time.Time`
  em UTC.
- Nunca registrar IP em log nem persisti-lo; usar a chave HMAC de `internal/httpx/clientip.go`.
- Respostas públicas (página, beacon, mídia) nunca chamam `http.SetCookie` nem carregam recurso de
  terceiros.
- Nenhuma escrita em banco no caminho de abertura da página: cliques e visitas só vão para a fila.
- Texto vindo do administrador é sempre escapado por `html/template`; nunca HTML cru.
- Mensagens de interface em português do Brasil.
- Ao final de cada fase: `gofmt -l .`, `go vet ./...` e `go test ./...`. Em pacotes concorrentes
  (cache de páginas, fila, beacon), também `go test -race`.

---

## Phase 1: Setup (infraestrutura compartilhada)

**Purpose**: dependências, configuração e deploy do volume de mídia

- [ ] T001 Acrescentar `github.com/gen2brain/webp` e `golang.org/x/image` ao `go.mod` (`go get` + `go mod tidy`) e confirmar `CGO_ENABLED=0 go build ./...` e `go vet ./...` (research R1; Complexity Tracking do plano).
- [ ] T002 [P] Em `internal/config/config.go` acrescentar a `Config` os campos `MediaDir` (`MEDIA_DIR`, padrão `./var/media`), `MediaMaxBytes` (`MEDIA_MAX_BYTES`, padrão `5242880`, inteiro > 0) e `PageCacheMB` (`PAGE_CACHE_MB`, padrão `64`, inteiro > 0); valor inválido aborta citando a variável. Testes em `internal/config/config_test.go` (padrões, inválidos, vazio). Criar `server.PrepareMedia(dir string) error` em `internal/server/media_dir.go` (`os.MkdirAll` e gravar/remover um arquivo temporário) e chamá-la em `cmd/hublinks/main.go` antes de `server.Build`; se falhar, o processo sai com erro que cite `MEDIA_DIR`. Teste com diretório somente leitura em `internal/server/media_dir_test.go`.
- [ ] T003 [P] Garantir `var/` em `.gitignore` (já incluído com o `scripts/browser-check`); acrescentar `MEDIA_DIR`, `MEDIA_MAX_BYTES` e `PAGE_CACHE_MB` a `deploy/.env.example` (valores fictícios) e ajustar o teste cobertor de `cmd/hublinks/deploy_test.go` para exigir também as variáveis de `specs/002-paginas-de-produto/contracts/config.md`.
- [ ] T004 Em `deploy/Dockerfile` criar `/data/media` no estágio de build e copiá-la para a imagem final com `--chown=65532:65532`; em `deploy/docker-compose.yml` declarar o volume `hublinks-media`, montá-lo em `/data/media` e definir `MEDIA_DIR=/data/media` no serviço `app`. Atualizar `cmd/hublinks/deploy_test.go` para proteger o volume, a variável e o dono da pasta.

---

## Phase 2: Foundational (pré-requisitos bloqueantes)

**Purpose**: migração, domínio, resolução de código tipada, eventos de produto, correção das estatísticas e pacote de mídia. Nenhuma história começa antes desta fase

- [ ] T005 Escrever `migrations/00004_products.sql` conforme `data-model.md`: tabelas `categories`, `products`, `media`, `product_media`, `product_spec_groups`, `product_spec_items`, `product_related`; `affiliate_links.product_id uuid NULL` com FK e índice `(org_id, product_id)`; `marketplaces.logo_media_id uuid NULL` com FK; `short_codes.target_type IN ('affiliate_link','product')` e remoção da FK de `target_id`; CHECKs de `click_events` e `click_daily` aceitando `event_type IN ('click','view')` e `target_type IN ('affiliate_link','product')`; `visitor_seen.event_type text NOT NULL DEFAULT 'click' CHECK(event_type IN ('click','view'))` na chave primária `(target_type, target_id, event_type, scope, channel_id, visitor_id)`; índices únicos parciais `(org_id, slug) WHERE purged_at IS NULL` em `products` e `categories`; `product_related` com CHECK `product_id <> related_product_id`. `Down` reversível.
- [ ] T006 Testar a migração em `internal/store/migrate_test.go`: sobe, desce e sobe de novo em banco com dados da fase 1 (links, eventos, `visitor_seen`) sem perdê-los; recusa slug de produto sem hífen ou fora de `^[a-z0-9]+(-[a-z0-9]+)+$`, relacionado igual ao próprio produto e slug repetido na mesma org (inclusive de item na lixeira).
- [ ] T007 [P] Criar `internal/domain/product.go` com `Product`, `Category`, `Media`, `SpecGroup`, `SpecItem`, `ProductStatus` (`draft`|`published`) e `Target` (alvo de código: `Kind` `link`|`product`, com `Link` ou `Product`); em `internal/domain/errors.go` acrescentar `ProductSlugTakenError`, `LinkAlreadyLinkedError{Product}`, `CategoryInUseError{Products}` e `MediaInvalidError`.
- [ ] T008 [P] Escrever primeiro os testes em `internal/domain/product_test.go` e depois implementar em `internal/domain/validate.go` (arquivo já existente da fase 1; não editar `product.go` aqui) as funções `ValidProductSlug` (`^[a-z0-9]+(-[a-z0-9]+)+$`, até 80), `ValidCategorySlug` (`^[a-z0-9]+(-[a-z0-9]+)*$`, até 60), `SuggestSlug(title, id)` (sem acentos via `golang.org/x/text/unicode/norm`; sem hífen → acrescenta `-` e os 4 primeiros caracteres do ID), `ValidateProduct` (`title` obrigatório até 200; `short_description` até 500; `long_description` até 20.000; `seo_title` até 70; `seo_description` até 160), `ValidateCategory` (`name` obrigatório até 80), `ValidateSpecs` (até 50 grupos, até 100 itens por grupo, `name` até 120, `value` até 1.000, grupo sem itens recusado), `ValidateRelated` (até 20, sem o próprio produto) e `ValidateGallery` (até 10 imagens, `alt` até 200).
- [ ] T009 Em `internal/store/codes.go` criar `ResolveTarget(ctx, code) (domain.Target, error)`: junta `short_codes` ao link (regras atuais) ou ao produto (`status='published'`, `deleted_at IS NULL`, `purged_at IS NULL`; senão `ErrNotFound`); manter `Resolve` de links como está. Generalizar `insertShortCode` para receber `target_type`. Testes de integração em `internal/store/codes_test.go` (link, produto publicado, rascunho, lixeira, excluído, código inexistente).
- [ ] T010 Em `internal/redirect/cache.go` e `internal/redirect/handler.go` trocar `AffiliateLink` por `domain.Target` na resolução e no cache (TTL de 5 min e negativo de 30 s inalterados), mantendo o fluxo de links idêntico (`302`, prévia de crawler, `Recorder`); o ramo `product` responde `404` por enquanto. Os testes existentes de `internal/redirect/*_test.go` devem continuar passando sem alteração de comportamento; acrescentar teste do cache para alvos de produto. Na interface `redirect.Catalog` substituir `Resolve(ctx, code) (domain.AffiliateLink, error)` por `ResolveTarget(ctx, code) (domain.Target, error)`, adaptar `store.Catalog` e os fakes de `internal/redirect/*_test.go`; o método `Resolve` de links continua existindo em `store.Catalog` para quem já o usa.
- [ ] T011 Em `internal/events/queue.go` acrescentar `Kind` (`click`|`view`, padrão `click`) e `TargetType` (`affiliate_link`|`product`, padrão `affiliate_link`) a `Event`; em `internal/events/writer.go` gravar `event_type` e `target_type` do evento e incluir `event_type` no `INSERT … ON CONFLICT DO NOTHING` de `visitor_seen` (unicidade de visitas independente da de cliques; robôs continuam sem tocar em `visitor_seen`). Testes em `internal/events/writer_test.go`: visita e clique do mesmo visitante/produto são ambos únicos; repetir visita não é único; concorrência com `-race` gera exatamente um único.
- [ ] T012 Corrigir `internal/stats/stats.go`: toda consulta da fase 1 (`dailyCTE`, resumos, canais, links) passa a filtrar `event_type='click' AND target_type='affiliate_link'` em `click_events` e `click_daily`. Escrever antes o teste de regressão em `internal/stats/stats_test.go`: com eventos de produto (`click` e `view`) no mesmo banco, os totais de links, canais e dia ficam idênticos aos da fase 1 (research R8).
- [ ] T013 [P] Criar o pacote `internal/media` (`media.go`): `Validate` detecta o tipo pelo conteúdo (`http.DetectContentType`; só JPEG, PNG e WebP), rejeita arquivo acima de `MEDIA_MAX_BYTES` e imagem acima de 24 megapixels lida por `image.DecodeConfig` antes de decodificar; `Process` gera variantes WebP nas larguras {320, 640, 960, 1280} limitadas à largura da origem (sempre ao menos uma), com `draw.CatmullRom` e qualidade 80, preservando transparência, sem manter o original; `Store` grava `{id[0:2]}/{id}/{largura}.webp` por arquivo temporário + `rename`; `Open` e `Remove` aceitam só UUID e largura da lista fechada. Testes em `internal/media/media_test.go` com imagens geradas no teste: JPEG, PNG com alfa, WebP, tipo inválido, grande demais, bomba de descompressão, remoção de arquivo já ausente. `Process` é limitado por semáforo a 2 execuções simultâneas (uma imagem de 24 megapixels ocupa cerca de 96 MB decodificada); teste com 5 chamadas simultâneas confirmando que no máximo 2 rodam ao mesmo tempo.
- [ ] T014 Criar `internal/store/media.go` (inserir, buscar, listar órfãs com mais de 24 h sem vínculo em `product_media` nem `marketplaces.logo_media_id`, apagar) com testes de integração em `internal/store/media_test.go`; toda função recebe `orgID`.
- [ ] T015 Criar `internal/service/invalidate.go` com a interface `PageInvalidator` (`Bump(orgID)`) e fazer o `service.Catalog` existente chamá-la (e invalidar o `redirect.Cache`) ao editar, ativar/inativar, excluir ou restaurar link e ao editar ou excluir marketplace; implementação nula para testes. Testes em `internal/service/catalog_test.go`.

**Checkpoint**: `go test ./...` e `go vet ./...` passam; a migração sobe e desce; os totais da fase 1 não mudam com eventos de produto no banco; `CGO_ENABLED=0 go build ./...` compila.

---

## Phase 3: User Story 1 - Publicar uma página de produto pelo painel (Priority: P1)

**Purpose**: cadastrar, vincular links, enviar imagens, publicar e excluir produtos pelo painel e pela API

**Goal**: o administrador cadastra um produto, vincula links de afiliado, envia imagens e o publica, obtendo endereço amigável e código curto.

**Independent Test**: criar um produto com título, imagem e um link no painel, publicá-lo e conferir endereço e código; despublicar, excluir e restaurar. A abertura do endereço público fecha no checkpoint da US2.

- [ ] T016 [US1] Escrever os testes de integração de `internal/store/products_test.go`: criar com código curto próprio, slug único por org (inclusive item na lixeira e reutilização após `purged_at`), vincular e desvincular links (link de outro produto → erro com o produto atual), publicar, despublicar, lixeira, restauração, listagem com busca/filtro/paginação e isolamento por `org_id`.
- [ ] T017 [US1] Implementar `internal/store/products.go`: criar (gera `short_codes` em transação), buscar por id/slug, listar, atualizar, `SetStatus`, `SoftDelete`, `Restore`, `SetLinks` (substitui o conjunto vinculado em `affiliate_links.product_id`, recusando link já vinculado a outro produto) e `SetGallery` (`product_media`, posição 0 = imagem principal, contígua). `SetStatus` preenche `published_at` apenas na primeira publicação e nunca o apaga ao despublicar.
- [ ] T018 [US1] Escrever os testes de `internal/service/products_test.go` e implementar `internal/service/products.go` (usa `domain.ValidateProduct`, `ValidateGallery` e `SuggestSlug`). Regras testadas: sugestão e validação de slug; publicar exige título e slug válidos; restaurar dentro do prazo funciona e, após o prazo, devolve `ErrNotFound`. Invalidação: **toda escrita** que afete a página (criar, editar, publicar, despublicar, excluir, restaurar, vincular ou desvincular link, trocar imagem, alterar ficha ou relacionados) invalida a entrada do código no `redirect.Cache` e chama `PageInvalidator.Bump`; um teste percorre cada uma dessas operações e confirma o `Bump`.
- [ ] T019 [P] [US1] Escrever os testes de `internal/admin/media_test.go` e implementar `internal/admin/media.go` (`POST /admin/media`, `multipart`, CSRF): valida com `internal/media`, grava as variantes, insere a mídia sem vínculo e devolve o trecho HTMX com a miniatura e o campo oculto `media_id`; tipo inválido → `422`, acima do limite → `413`, mensagens em português. Limitar o corpo com `http.MaxBytesReader` (`MediaMaxBytes` mais 1 MiB de folga para os campos do formulário) e chamar `ParseMultipartForm` com limite de memória de 1 MiB (o resto vai para arquivo temporário); teste com corpo maior que o limite → `413` sem consumir memória proporcional.
- [ ] T020 [P] [US1] Escrever os testes e implementar `GET /media/{id}/{largura}.webp` em `internal/web/media.go`: `id` por `uuid.Parse` e largura da lista fechada; `200 image/webp` com `Cache-Control: public, max-age=31536000, immutable` e `X-Content-Type-Options: nosniff`; qualquer outra entrada ou arquivo ausente → `404`; nunca emite `Set-Cookie`.
- [ ] T021 [US1] Escrever os testes em `internal/admin/products_test.go` e implementar `internal/admin/products.go` conforme `contracts/admin-ui.md`: listagem com busca, filtro de status e paginação por HTMX; formulário de criação (título, slug sugerido, descrição curta e longa, galeria com texto alternativo, seleção de links vinculados entre os links ativos, fora da lixeira, sem produto ou já deste produto); detalhe com endereço amigável e, em "URLs para divulgar", o código curto e uma URL por canal ativo, cada uma com botão copiar (componente já usado na tela do link); publicar/despublicar (confirmação ao despublicar); exclusão com confirmação e restauração; aviso ao trocar o slug de produto já divulgado; estado vazio "Cadastre seu primeiro produto"; ao vincular um link que pertence a outro produto, exibir o erro `LinkAlreadyLinkedError` com o nome desse produto. Nomes dos campos do formulário (testes e templates devem usá-los): `title`, `slug`, `short_description`, `long_description`, `seo_title`, `seo_description`, `category_id`, `link_id` (repetido, um por link vinculado) e, para a galeria em ordem, `media_id` e `media_alt` (repetidos, mesma ordem; o primeiro é a imagem principal).
- [ ] T022 [US1] Criar os templates `web/templates/admin/products/list.html`, `form.html`, `show.html` e `confirm_delete.html` com os componentes existentes (campos, tabela que vira cartões abaixo de 40 rem, selo de status, `unsaved.js`), `data-label` nas células, rótulos e `alt`; mostrar os links vinculados com `policy_badge` e a marca "não rastreável" nos `direct`. Item "Produtos" no menu lateral de `web/templates/layouts/admin.html`. Incluir os novos diretórios de templates (`templates/admin/products`, `templates/admin/categories`) na lista de pastas de `TestEveryIconUsedInTemplatesExists` em `web/components_test.go`. O campo de envio de imagem usa `hx-post="/admin/media"` com `hx-encoding="multipart/form-data"` (sem isso o HTMX não envia o arquivo) e `hx-target` no trecho da miniatura.
- [ ] T023 [P] [US1] Escrever os testes e implementar a API em `internal/api/products.go` e `internal/api/media.go` conforme `contracts/api-v1.md` (`/products` CRUD, `publish`, `unpublish`, `restore`, `media` multipart): `409` com o produto atual ao vincular link de outro produto e com o slug repetido, `422` nas validações, `404` para outra org; o objeto de produto traz `url`, `short_url`, `code` e links com `trackable`.
- [ ] T024 [US1] Registrar em `internal/server/server.go` as rotas abaixo (`view` = sessão; `write` = sessão + CSRF, como as demais): `GET /admin/products`, `GET /admin/products/new`, `POST /admin/products`, `GET /admin/products/{id}`, `GET /admin/products/{id}/edit`, `POST /admin/products/{id}`, `POST /admin/products/{id}/publish`, `POST /admin/products/{id}/unpublish`, `GET /admin/products/{id}/delete`, `POST /admin/products/{id}/delete`, `POST /admin/products/{id}/restore`, `POST /admin/media`; API (sessão, CSRF nas escritas): `POST` e `GET /api/v1/products`, `GET`, `PATCH` e `DELETE /api/v1/products/{id}`, `POST /api/v1/products/{id}/publish`, `POST /api/v1/products/{id}/unpublish`, `POST /api/v1/products/{id}/restore`, `POST /api/v1/media`; pública: `GET /media/{id}/{file}` (o mux não aceita curinga parcial: `file` = `{largura}.webp`, validado no handler). Atualizar `internal/server/routes_test.go` e `internal/server/nocookie_test.go` para cobrir as rotas novas. Antes de registrar, construir em `Build`: o `media.Store` (com `c.MediaDir` e `c.MediaMaxBytes`), o `service.Products` (store, `redirect.Cache`, `media.Store` e um `PageInvalidator` nulo, que a tarefa de registro de `/p/{slug}` troca pelo `PageCache`) e os handlers `admin.Products`, `admin.Media`, `api.Products` e `api.Media` com essas dependências.
- [ ] T025 [US1] Mostrar produtos na lixeira do painel (`internal/admin/trash.go` e `web/templates/admin/trash_list.html`) e estender a expiração em `internal/maintenance/trash.go`: ao fim do prazo marcar `purged_at`, desvincular os links (`product_id = NULL`), apagar `product_media`, mídias exclusivas (linhas e arquivos), grupos, itens e relações, apagar `visitor_seen` do produto e invalidar o `redirect.Cache`. Testes em `internal/maintenance/trash_test.go`.
- [ ] T026 [US1] Estender `internal/maintenance` com a limpeza de mídia órfã (sem vínculo há mais de 24 h; apaga linha e arquivos, tolerando arquivo ausente) chamada pelo agendador diário; testes em `internal/maintenance/media_test.go`. Registrar a chamada em `internal/maintenance/scheduler.go`, junto das demais rotinas diárias, com contagem de falhas na métrica existente de manutenção.

**Checkpoint**: no painel é possível criar, editar, vincular links, enviar imagens, publicar, despublicar, excluir e restaurar um produto (também pela API); o código curto é gerado e `/p/{slug}` ainda não é público (US2). `go test ./...` e `go vet ./...` passam. Quickstart: passo 3, itens 2 e 3 (ignore categoria e logotipo, que chegam na US6).

---

## Phase 4: User Story 2 - Visitante vê o produto e vai à loja (Priority: P1)

**Purpose**: página pública renderizada no servidor, com cache, pré-visualização e botões de "Onde comprar"

**Goal**: quem chega por rede social vê a página no celular e chega à loja pelo botão certo.

**Independent Test**: com um produto publicado e dois links de lojas diferentes, abrir a página a 375 px e chegar a cada loja pelos botões.

- [ ] T027 [US2] Escrever os testes de `internal/web/pagecache_test.go` (inclusive `-race`) e implementar `internal/web/pagecache.go`: cache em memória de HTML por `(produto, segmento do canal)` com orçamento de `PAGE_CACHE_MB` e remoção do menos usado, `ETag` por hash do conteúdo e versão por organização; implementa `service.PageInvalidator` (`Bump` incrementa a versão e entradas antigas são descartadas na leitura); `ErrNotFound` nunca é cacheado como página. Construtor `web.NewPageCache(budgetMB int)` chamado em `Build` com `c.PageCacheMB`.
- [ ] T028 [US2] Implementar `internal/web/product.go` e, em `internal/store/products.go`, a leitura da página: `DefaultOrgID(ctx)` (única organização da instalação: `SELECT id FROM organizations ORDER BY created_at LIMIT 1`, guardada em memória no processo; o slug é único por organização e a fase 2 tem uma só; registrar a assunção em `docs/pendencias.md`) e `PageBySlug(ctx, orgID, slug)`/`PageByID` (produto publicado fora da lixeira, imagem principal e galeria, links ativos fora da lixeira com seus marketplaces fora da lixeira, logotipo do marketplace). O modelo da página agrupa os links por marketplace, com logotipo ou nome; o botão `shorten` aponta para `BASE_URL[/{segmento}]/{código do link}` (com o canal da visita) e o `direct` para `destination_url` com `rel="noopener noreferrer nofollow sponsored"`; a descrição longa vira parágrafos escapados (função de template, sem HTML cru). Testes em `internal/web/product_test.go` e `internal/store/products_test.go`.
- [ ] T029 [US2] Criar `web/templates/public/product.html` (visão geral com imagem principal `srcset`/`sizes`/`width`/`height`/`fetchpriority`, descrição curta, botão "Ver onde comprar", seção "Onde comprar" sem preço, descrição longa, barra fixa, aviso de privacidade) e os parciais em `web/templates/components/` (galeria, cartão de marketplace, barra fixa); seções vazias são omitidas e, sem links disponíveis, a página informa que não há lojas e oculta o botão principal. Sem imagem principal, usar `web/static/img/produto-sem-imagem.svg` (arquivo próprio, sem fonte externa; o diretório `web/static` já é embutido no binário), com `alt` igual ao título e dimensões declaradas, e omitir `og:image` e a imagem do JSON-LD; teste da página sem imagem em `internal/web/product_test.go`. Acrescentar `public/product` à tabela de `TestPublicPagesAlwaysCarryThePrivacyNotice` em `web/layouts_test.go`, para a página sempre trazer o aviso de privacidade. O `srcset` lista somente as larguras existentes em `media.widths`, `sizes` e `width`/`height` vêm da maior variante. Seguir `docs/guia-de-estilo.md` e a seção 11.1 de `docs/referencias/escopo.md`; a inspiração visual está em `docs/referencias/layout-referencia.png`, sem copiar marca, textos ou dados.
- [ ] T030 [P] [US2] Acrescentar os estilos da página em `web/static/css/base.css` (mobile first, tokens existentes, modos claro e escuro, contraste AA, foco visível, barra fixa e barra de ação no rodapé do celular, `scroll-snap`) e rodar `make css`; criar `web/static/js/product.js` apenas com a revelação da barra fixa por `IntersectionObserver` (sem dependências, < 2 KB). Sem JavaScript os botões e o conteúdo continuam utilizáveis. Este arquivo recebe depois código de outras tarefas (T038 e T041): ao editá-lo, acrescentar sem remover o que já existe.
- [ ] T031 [US2] Registrar `GET /p/{slug}` em `internal/server/server.go` com a mesma instância do limitador de `GET /`; `200` com `ETag` e `Cache-Control: no-cache`, `304` para `If-None-Match` igual, `404` para rascunho, lixeira, excluído ou inexistente (HTML simples, sem cookie). Resolver o ramo `product` do handler de `/{código}` e `/{canal}/{código}` em `internal/redirect/handler.go` para entregar a mesma página, sem redirecionar e sem evento por enquanto; `HEAD` sem corpo. Contrato: `specs/002-paginas-de-produto/contracts/public-routes.md`. A organização vem de `DefaultOrgID` (tarefa anterior). Ligar o `PageCache` como `service.PageInvalidator` do `service.Catalog` e do `service.Products` em `Build`. Teste em `internal/server/product_routes_test.go`: editar o título, trocar um link, inativar um link e renomear o marketplace mudam o `ETag` da página na abertura seguinte.
- [ ] T032 [US2] Implementar a pré-visualização em `internal/admin/products.go` (sessão exigida, aviso "Rascunho", `noindex`, sem cache e sem eventos), registrar `GET /admin/products/{id}/preview` em `internal/server/server.go` com `view` e acrescentar o botão em `web/templates/admin/products/show.html`. Teste em `internal/admin/products_test.go`: rascunho aparece para a sessão e continua `404` sem sessão em `/p/{slug}`.
- [ ] T033 [US2] Escrever os testes de `internal/web/product_handler_test.go` e `internal/server/product_routes_test.go`: `200`/`404`/`304`; nenhuma resposta com `Set-Cookie`; HTML sem domínio externo; um botão por link, agrupados por marketplace; `direct` leva à URL original e `shorten` ao código do link com o canal da visita; link inativo ou na lixeira some; produto sem links mostra a mensagem; edição do título aparece na abertura seguinte (SC-007); `429` com `Retry-After` excedendo o limite. Verificar também, no HTML renderizado: `lang="pt-BR"`, texto alternativo em todas as imagens, um único `<h1>` e rótulos acessíveis nos controles.

**Checkpoint**: um produto publicado abre em `/p/{slug}` e em `/{código}` sem sessão e sem cookie; os botões levam às lojas pela política de cada link; a edição aparece na abertura seguinte. Quickstart: passo 3, itens 2 a 5 (ignore categoria e logotipo até a US6), passo 4 e passo 8 (`304`, edição visível na abertura seguinte, lixeira). `-race` em `internal/web` passa. Rodar `make browser-check` em `/p/{slug}` e `/{código}` (375 e 1280 px, claro e escuro) sem falhas.

---

## Phase 5: User Story 3 - Ficha técnica e produtos relacionados (Priority: P2)

**Purpose**: grupos de especificações por produto em acordeão e carrossel de relacionados

**Goal**: cada produto tem sua própria ficha técnica e seus relacionados.

**Independent Test**: criar três grupos com itens, reordená-los, escolher dois relacionados e conferir a página.

- [ ] T034 [US3] Escrever os testes de `internal/store/products_test.go` (acréscimo) e implementar em `internal/store/products.go` `ReplaceSpecs` (substitui `product_spec_groups` e `product_spec_items` do produto numa transação, posições contíguas) e `ReplaceRelated` (`product_related`, mesma org, sem o próprio produto, até 20, posições contíguas), mais a leitura de ambos; relacionados só publicados e fora da lixeira na leitura pública. Incluir a ficha e os relacionados no modelo de página lido por `PageBySlug` e `PageByID` (criados na tarefa de `internal/web/product.go`).
- [ ] T035 [US3] Acrescentar à `internal/service/products.go` as operações de ficha e relacionados (usando `domain.ValidateSpecs` e `ValidateRelated`) com invalidação da página e testes em `internal/service/products_test.go`.
- [ ] T036 [US3] Criar o editor de ficha no formulário do produto (`web/templates/admin/products/form.html` e `web/templates/components/spec_editor.html`): adicionar, remover e mover grupos e itens com botões acessíveis por teclado (Alpine, sem arrastar-e-soltar), campos indexados, aviso de grupo vazio; busca HTMX de relacionados em `GET /admin/products/related-search?q=` (publicados, exceto o próprio) reordenáveis por botões. Testes em `internal/admin/products_test.go`. Registrar `GET /admin/products/related-search` em `internal/server/server.go` com `view` (rota literal; o mux a prefere a `{id}`). Nomes dos campos: `specs[0][name]`, `specs[0][items][0][name]`, `specs[0][items][0][value]` (índices reordenados no envio) e `related_id` (repetido, na ordem escolhida).
- [ ] T037 [P] [US3] Estender a API (`internal/api/products.go`): `specs` e `related` em `PATCH /products/{id}` e no detalhe, com os erros `422` das validações; testes em `internal/api/products_test.go`.
- [ ] T038 [US3] Renderizar a ficha em `<details>` (um por grupo, `id` âncora, só o primeiro aberto, abre sozinho quando o `#hash` aponta para ele) e o carrossel de relacionados com `scroll-snap` e `loading="lazy"` em `web/templates/public/product.html`; acrescentar o controle "Mostrar todos" a `web/static/js/product.js`; omitir grupos sem itens e relacionados despublicados, excluídos ou sem imagem. Testes em `internal/web/product_test.go`. Este arquivo (`product.js`) já recebeu código de T030: acrescentar sem remover. O `id` de cada grupo é o slug do nome (minúsculas, sem acentos, hífens; nome repetido ganha `-2`, `-3`…), por exemplo "Tela" → `#tela`.

**Checkpoint**: três grupos criados, reordenados e exibidos na ordem salva, com acordeão e "Mostrar todos"; relacionado despublicado some do carrossel; relacionar o produto a ele mesmo é recusado. Passo 5 do quickstart.

---

## Phase 6: User Story 4 - Medir visitas e cliques do produto (Priority: P2)

**Purpose**: beacon de visita, clique do código curto, estatísticas separadas e retenção

**Goal**: o administrador vê, por produto, visitas e cliques em separado, por canal e por dia.

**Independent Test**: abrir a página três vezes (dois visitantes) e usar o link curto uma vez; o painel mostra 3 visitas (2 únicas) e 1 clique.

- [ ] T039 [US4] Escrever os testes de `internal/api/beacon_test.go` (inclusive `-race`) e implementar `internal/api/beacon.go` (`POST /api/v1/beacon/view`): corpo `text/plain` de até 512 bytes com `{"k","c"}`; `204` sempre, sem corpo e sem cookie; resolve o código pelo `redirect.Cache` e o canal pelo cache de canais; ignora crawler, código inexistente, de link ou de produto não publicado e canal inexistente; enfileira `Event{Kind: view, TargetType: product}` com o mesmo `VisitorID`, marcando `is_bot` pelo `BotDetector`; instância própria do limitador (`429` ao exceder). Registrar a rota em `internal/server/server.go` antes do coringa `GET /api/v1/`. Contrato: `contracts/public-routes.md`. Testar também que crawler conhecido, código de link e produto não publicado respondem `204` sem enfileirar evento. O `api.Beacon` recebe o `redirect.Cache`, o catálogo, o cache de canais, a fila de eventos, o `BotDetector`, o `Pepper` e `TrustedProxies`, construídos em `Build`. Testar também `GET /api/v1/beacon/view` → resposta do coringa `/api/v1/` (`401` sem sessão) e `PUT` → `405`.
- [ ] T040 [US4] Em `internal/redirect/record.go` acrescentar o registro do `click` de produto (`TargetType: product`) quando o código curto do produto é aberto (também com canal), ignorando crawlers e `HEAD`, e chamar esse registro no ramo `product` de `internal/redirect/handler.go`; nunca bloqueia a resposta. Testes em `internal/redirect/record_test.go` e `handler_integration_test.go`: um clique por abertura, nenhum para crawler, descarte contabilizado com a fila cheia. A interface `redirect.Recorder` hoje só tem `Record(r *http.Request, link domain.AffiliateLink, code, channelID string)`: acrescentar a ela e a `EventRecorder` o método `RecordProduct(r *http.Request, p domain.Product, code, channelID string)` e atualizar os fakes dos testes de `internal/redirect`.
- [ ] T041 [P] [US4] Em `web/static/js/product.js` enviar o beacon uma vez por carregamento (`navigator.sendBeacon` com `Blob` `text/plain`, falha silenciosa), lendo `data-k` e `data-c` do elemento raiz de `web/templates/public/product.html`; a página continua funcionando sem JavaScript. Este arquivo já tem código de T030 e T038: acrescentar sem remover.
- [ ] T042 [US4] Escrever os testes de integração de `internal/stats/products_test.go` e implementar `internal/stats/products.go`: `ProductsList` (visitas e cliques, totais e únicos, do período), `ProductDetail` (séries diárias, quebra por canal, links com `trackable` e **sem** contadores nos `direct`) e `Summary` com `page_visits` e `page_visits_unique`; dias fechados vêm de `click_daily`, o dia corrente de `click_events`; unicidade do produto = soma de `unique_target`, por canal = `unique_url`.
- [ ] T043 [US4] Mostrar visitas e cliques separados no detalhe e na listagem do produto (`internal/admin/products.go`, `web/templates/admin/products/show.html` e `list.html`, com o seletor de período e o gráfico existentes) e o cartão "Visitas às páginas" em `internal/admin/dashboard.go` e `web/templates/admin/dashboard.html`; o cartão "Cliques" continua sendo só de links. Conforme `contracts/admin-ui.md`. Em `Build`, passar o `statsService` ao `admin.Products` (novo campo `Stats`).
- [ ] T044 [P] [US4] Expor as estatísticas na API: `GET /api/v1/stats/products`, `GET /api/v1/stats/products/{id}` e os novos campos de `GET /api/v1/stats/summary` em `internal/api/stats.go`, com testes em `internal/api/stats_test.go` (período inválido → `422`, outra org → `404`). Conforme `contracts/api-v1.md`. Incluir também `visits`, `visits_unique`, `clicks` e `clicks_unique` do período em cada item de `GET /api/v1/products` (`internal/api/products.go`), como diz o contrato. Registrar em `internal/server/server.go`, junto das rotas `/api/v1/stats/*` existentes (sessão, resposta JSON): `GET /api/v1/stats/products` e `GET /api/v1/stats/products/{id}`; teste em `internal/server/routes_test.go` de que respondem com sessão e `401` sem ela.
- [ ] T045 [US4] Testes de ponta a ponta em `internal/server/product_events_test.go` com PostgreSQL real: abrir a página duas vezes com o mesmo visitante → 2 visitas e 1 único; trocar o user-agent → novo único; clique do código curto separado da visita; crawler e `is_bot` fora das estatísticas; link `direct` sem contadores; consolidação diária e limpeza (`internal/maintenance`) preservam e não alteram visitas e cliques de produto, e reexecutar a consolidação não muda os valores.

**Checkpoint**: 3 visitas (2 únicas) e 1 clique aparecem separados no painel e na API; crawler e robô não contam; `direct` continua "não rastreável". Passo 6 do quickstart. `-race` em `internal/api`, `internal/redirect` e `internal/events` passa.

---

## Phase 7: User Story 5 - Compartilhar a página e ser encontrado (Priority: P2)

**Purpose**: metadados de compartilhamento, SEO, dados estruturados e texto de privacidade

**Goal**: o endereço colado numa rede social mostra uma boa prévia e a página é indexável.

**Independent Test**: inspecionar o HTML e colar o endereço amigável e o código curto num validador de prévia.

- [ ] T046 [US5] Escrever os testes de `internal/web/product_seo_test.go` e implementar o `<head>` da página (`web/templates/public/product.html` e `internal/web/product.go`): `<title>`, `meta description`, `link rel="canonical"` = `BASE_URL/p/{slug}` também quando aberta pelo código curto, Open Graph (`og:type=product`, `og:title`, `og:description`, `og:url` canônico, `og:image` de 1280 px) e `twitter:card=summary_large_image`; um único `<h1>` e hierarquia `h2`/`h3` correta. `og:image` usa a **maior variante gerada** da imagem principal (`max(media.widths)`), nunca uma largura fixa: imagens menores que 1280 px não têm essa variante. Teste com uma imagem de 600 px de largura.
- [ ] T047 [US5] Emitir o JSON-LD `schema.org/Product` (`name`, `description`, `image`, `category`) **sem** `offers`, preço, avaliação ou ranking, escapado por `html/template` (`internal/web/product.go`); teste que falha se `offers` ou `price` aparecerem. Montar o JSON com `json.Marshal` de uma struct (que escapa `<`, `>` e `&`) e emiti-lo como `template.JS` dentro de `<script type="application/ld+json">`, como `chartData` em `internal/admin/period.go`; nunca concatenar texto no script. Teste adicional: um produto com título `</script><script>alert(1)</script>` não gera nenhum `<script>` extra no HTML.
- [ ] T048 [US5] Campos de SEO no formulário e na API (`seo_title` até 70, `seo_description` até 160); quando em branco, as metatags usam título e descrição curta (FR-032). Testes em `internal/web/product_seo_test.go` e `internal/admin/products_test.go`.
- [ ] T049 [US5] Garantir que crawlers conhecidos recebem a página completa, `200` com os metadados de compartilhamento, em `/p/{slug}`, `/{código}` e `/{canal}/{código}` (`internal/redirect/handler_integration_test.go`). Que crawler não gera visita nem clique já é testado em T039 e T040; esta tarefa não depende deles.
- [ ] T050 [P] [US5] Atualizar `/privacidade` (`web/templates/public/privacy.html` e `internal/web/privacy.go`) com a contagem de aberturas de página de produto pelo mesmo identificador anônimo, a mesma retenção dos cliques e a limitação para navegadores sem JavaScript; ajustar `internal/web/privacy_test.go`.

**Checkpoint**: a prévia mostra título, imagem e descrição nos dois endereços e o canônico é sempre o amigável; o JSON-LD não contém oferta nem preço. Passo 7 do quickstart.

---

## Phase 8: User Story 6 - Categorias e logotipo de marketplace (Priority: P3)

**Purpose**: agrupar produtos em categorias e identificar lojas com logotipo

**Goal**: produtos organizados em categorias e lojas reconhecíveis pelo logotipo.

**Independent Test**: criar uma categoria, associá-la a um produto, definir o logotipo de um marketplace e conferir painel e página.

- [ ] T051 [US6] Escrever os testes de `internal/store/categories_test.go` e implementar `internal/store/categories.go` (CRUD por `orgID`, slug único com reserva na lixeira, exclusão lógica; ir para a lixeira só sem produtos fora da lixeira, devolvendo `CategoryInUseError` com os produtos; restaurar da lixeira) e a regra em `products.go`: restaurar produto cuja categoria está na lixeira é bloqueado.
- [ ] T052 [US6] Implementar `internal/service/categories.go` com testes (sugestão de slug, `domain.ValidateCategory`, invalidação das páginas), a API `internal/api/categories.go` conforme `contracts/api-v1.md` (`409` com os produtos que impedem) e a expiração da lixeira de categorias em `internal/maintenance/trash.go`. Registrar em `internal/server/server.go` as rotas da API, no mesmo padrão de `/api/v1/channels` (sessão com resposta JSON; `RequireCSRF` nas escritas): `POST /api/v1/categories`, `GET /api/v1/categories`, `GET /api/v1/categories/{id}`, `PATCH /api/v1/categories/{id}`, `DELETE /api/v1/categories/{id}` e `POST /api/v1/categories/{id}/restore`; testar que cada uma responde (não `404`) com sessão e `401` sem ela em `internal/server/routes_test.go`.
- [ ] T053 [US6] Painel de categorias: `internal/admin/categories.go` e `web/templates/admin/categories/{list,form,confirm_delete}.html` no padrão dos canais, mensagem que lista os produtos que impedem a exclusão, categorias na lixeira de `internal/admin/trash.go`, item "Categorias" no menu; registrar as rotas em `internal/server/server.go` e cobrir em `internal/admin/categories_test.go`. Rotas a registrar, no padrão de `/admin/channels` (`view` para `GET`, `write` para `POST`): `GET /admin/categories`, `GET /admin/categories/new`, `POST /admin/categories`, `GET /admin/categories/{id}/edit`, `POST /admin/categories/{id}`, `GET /admin/categories/{id}/delete`, `POST /admin/categories/{id}/delete` e `POST /admin/categories/{id}/restore`.
- [ ] T054 [US6] Campo de categoria no formulário do produto e na API, filtro por categoria na listagem de produtos (painel e API) e categoria nos dados estruturados da página; testes em `internal/admin/products_test.go` e `internal/api/products_test.go`.
- [ ] T055 [US6] Logotipo de marketplace: `marketplaces.logo_media_id` em `internal/store/marketplaces.go` e `internal/service/catalog.go` (vincula/remove, invalida páginas), envio e prévia em `web/templates/admin/marketplaces/form.html`, `logo_media_id` em `PATCH /marketplaces/{id}` (`internal/api/catalog.go`) e exibição na seção "Onde comprar" (nome quando sem logotipo). Testes em `internal/store/marketplaces_test.go`, `internal/admin/marketplaces_test.go` e `internal/web/product_test.go`.

**Checkpoint**: categoria criada, associada e filtrável; exclusão com produtos bloqueada com a lista; logotipo aparece na página e o nome quando ausente. Quickstart: passo 3, item 1 e a categoria do item 2.

---

## Phase 9: Polish e endurecimento

**Purpose**: carga, métricas, privacidade, documentação e validação final

- [ ] T056 [P] Em `internal/metrics/metrics.go` registrar: `hublinks_page_cache_hits_total` e `hublinks_page_cache_misses_total` (contadores), `hublinks_page_cache_bytes` (gauge), `hublinks_view_events_total` (contador de visitas enfileiradas pelo beacon; as métricas `hublinks_events_enqueued_total` e `hublinks_events_dropped_total` existentes já contam cliques e visitas juntos) e `hublinks_media_uploads_total{result}` com `result` em `ok`, `invalid`, `too_large` e `error`. Incrementar em `internal/web/pagecache.go`, `internal/api/beacon.go` e `internal/admin/media.go`. Testes em `internal/metrics/metrics_test.go`.
- [ ] T057 [P] Estender a auditoria de privacidade em `internal/audit` para os handlers novos (página, beacon, mídia): nenhum `Set-Cookie`, nenhum IP em log ou coluna, nenhum recurso de terceiros nos templates públicos.
- [ ] T058 Criar `scripts/seed-products.sh` (usa `psql` e as variáveis `PG*` como `scripts/manual-run.sh`): semeia 1.000 produtos publicados, cada um com título, descrição curta, dois a três grupos de ficha e um link `shorten` ativo, **sem imagem** (a página usa a imagem padrão); 10 dos produtos recebem imagem enviada por `POST /api/v1/media`. Adaptar `scripts/loadtest.sh` para abrir `/p/{slug}` e `/{código}` de produtos aleatórios; executar de verdade 200 aberturas/s por 60 s e registrar em `docs/operacao.md` o p95 no servidor (meta < 100 ms), os erros (meta zero) e o descarte (meta < 0,1 %) (SC-004, SC-011). Acrescentar `scripts/` à seção de estrutura de `specs/002-paginas-de-produto/plan.md` se ainda não estiver.
- [ ] T059 [P] Atualizar `docs/operacao.md`: variáveis novas, backup e restauração do volume `hublinks-media`, a migração `00004` (reconstrução da chave de `visitor_seen`), limpeza de mídia órfã e o que fazer com o cache de páginas; atualizar `docs/pendencias.md` com as assunções da fase 2 e `docs/guia-de-estilo.md` com os componentes novos (galeria, cartão de marketplace, acordeão, carrossel, barra fixa, editor de ficha), exibindo-os em `/admin/ui`. Em `docs/pendencias.md` registrar como assunções: organização única em `/p/{slug}`; slug antigo sem redirecionamento; imagem não publicada acessível por quem souber o UUID; tetos técnicos (50 grupos, 100 itens por grupo, 10 imagens por produto, 20 relacionados); categorias em lista simples; descrições em texto simples; relacionados manuais; `sitemap.xml` e `robots.txt` na fase 3; visita mede a página e não é atribuída a links `direct`. Os componentes novos entram em `internal/admin/ui.go` e `web/templates/admin/ui.html`.
- [ ] T060 Executar o `quickstart.md` completo (passos 1 a 10): teste automatizado; navegador: rodar `make browser-check ARGS="--base http://localhost:8099 --login admin@example.com:senha-de-teste-123 --path /p/{slug} --path /{código} --path /admin/products …"` (ver `docs/desenvolvimento.md`; 375 e 1280 px, claro e escuro) e corrigir toda falha (rolagem horizontal, `axe` sério ou crítico, recursos de terceiros, erro de console), revisar os avisos e as capturas de `var/browser-check/`, e registrar o que depende do olhar de uma pessoa (visual, teclado completo, fluxos); Lighthouse no DevTools em modo celular (emulação padrão com 4G lento simulado) com LCP < 2,5 s, CLS < 0,1 e INP < 200 ms (SC-002); SC-005 abrindo a página 100 vezes em navegador real (manual ou script Playwright fora do repositório) e conferindo `select count(*) from click_events where event_type='view'`; `docker compose up --build` com o volume e a persistência após recriar o contêiner; `MEDIA_DIR` sem permissão impedindo a partida. Registrar o resultado e o que depender de uma pessoa ou do Docker em `docs/verificacao.md`; não declarar aprovado o que não foi executado.
- [ ] T061 Rodar `gofmt -l .`, `go vet ./...`, `go test ./...`, `make test-ci` (nenhum teste pulado) e `go test -race ./internal/events/... ./internal/redirect/... ./internal/web/... ./internal/api/... ./internal/maintenance/...` com `TEST_DATABASE_URL`; atualizar a seção "Momento atual e próximo trabalho" do `AGENTS.md`.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (T001–T004)**: sem dependências.
- **Foundational (T005–T015)**: depende do Setup e **bloqueia todas as histórias** (migração, alvo
  tipado, eventos de produto, correção das estatísticas e pacote de mídia).
- **US1 (T016–T026)**: depende da Foundational. A abertura do endereço público fecha na US2.
- **US2 (T027–T033)**: depende da US1 (produtos e links vinculados) e da Foundational.
- **US3 (T034–T038)**: depende da US2 (a ficha e os relacionados aparecem na página).
- **US4 (T039–T045)**: depende da US2 (a página que envia o beacon) e dos eventos da Foundational.
- **US5 (T046–T050)**: depende da US2; pode andar em paralelo com a US3 e a US4.
- **US6 (T051–T055)**: depende da US1 (formulário) e da US2 (logotipo na página); pode andar em
  paralelo com a US3 a US5.
- **Polish (T056–T061)**: depende das histórias desejadas.

### Dentro de cada história

- Testes antes da implementação (devem falhar primeiro); repositório → serviço → painel/API →
  rotas → página.
- `internal/server/server.go`, `web/templates/public/product.html`, `web/static/js/product.js`,
  `web/static/css/base.css` e os arquivos de produto (`internal/store/products.go`,
  `internal/service/products.go`, `internal/admin/products.go`, `internal/api/products.go`,
  `web/templates/admin/products/form.html`) são editados por várias tarefas: não rode em paralelo tarefas que os
  alterem, e ao editá-los acrescente sem remover o que já existe.

## Parallel Opportunities

- Setup: as tarefas [P] (T002 e T003) em paralelo.
- Foundational: domínio (T007, T008) e `internal/media` (T013) em paralelo
  com a migração.
- US1: envio de mídia (T019), rota `/media` (T020) e API (T023) em paralelo.
- US3 a US6 em paralelo após a US2, por equipes ou sessões diferentes, cuidando dos conflitos nos
  arquivos compartilhados listados acima.

## Implementation Strategy

### MVP primeiro (US1 + US2)

1. Setup e Foundational.
2. US1 e US2: produto publicado com página pública e botões para as lojas.
3. **Parar e validar** (passos 3 e 4 do quickstart). Com isso o hub já divulga produtos.

### Entrega incremental

1. US3 (ficha e relacionados) → validar o passo 5.
2. US4 (visitas e cliques) → validar o passo 6; só então o painel mede a fase 2.
3. US5 (compartilhamento e SEO) → validar o passo 7.
4. US6 (categorias e logotipos) → validar a organização do catálogo.
5. Polish: carga com 1.000 produtos, documentação e a verificação final.

## Notes

- Itens fora de escopo permanente (preços, ranking, veredito, avaliações, comparador, integrações
  com marketplaces) **não** entram; `sitemap.xml`, `robots.txt` e site público são da fase 3.
- Links `direct` seguem "não rastreável" e sem métrica; a visita mede a página, não o link.
- Registre dúvidas e assunções em `docs/` e consulte o responsável antes de decidir questões em
  aberto.
