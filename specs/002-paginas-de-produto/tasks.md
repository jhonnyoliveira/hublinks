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
- [ ] T002 [P] Em `internal/config/config.go` ler `MEDIA_DIR` (padrão `./var/media`), `MEDIA_MAX_BYTES` (padrão `5242880`, inteiro > 0) e `PAGE_CACHE_MB` (padrão `64`, inteiro > 0); valor inválido aborta citando a variável. Testes em `internal/config/config_test.go` (padrões, inválidos, vazio). Na partida (`server.Build`), criar `MEDIA_DIR` com `os.MkdirAll` e gravar/remover um arquivo temporário; se falhar, abortar com erro que cite `MEDIA_DIR`. Teste com diretório somente leitura.
- [ ] T003 [P] Ignorar `var/` em `.gitignore`; acrescentar `MEDIA_DIR`, `MEDIA_MAX_BYTES` e `PAGE_CACHE_MB` a `deploy/.env.example` (valores fictícios) e ajustar o teste cobertor de `cmd/hublinks/deploy_test.go` para exigir também as variáveis de `specs/002-paginas-de-produto/contracts/config.md`.
- [ ] T004 Em `deploy/Dockerfile` criar `/data/media` no estágio de build e copiá-la para a imagem final com `--chown=65532:65532`; em `deploy/docker-compose.yml` declarar o volume `hublinks-media`, montá-lo em `/data/media` e definir `MEDIA_DIR=/data/media` no serviço `app`. Atualizar `cmd/hublinks/deploy_test.go` para proteger o volume, a variável e o dono da pasta.

---

## Phase 2: Foundational (pré-requisitos bloqueantes)

**Purpose**: migração, domínio, resolução de código tipada, eventos de produto, correção das estatísticas e pacote de mídia. Nenhuma história começa antes desta fase

- [ ] T005 Escrever `migrations/00004_products.sql` conforme `data-model.md`: tabelas `categories`, `products`, `media`, `product_media`, `product_spec_groups`, `product_spec_items`, `product_related`; `affiliate_links.product_id uuid NULL` com FK e índice `(org_id, product_id)`; `marketplaces.logo_media_id uuid NULL` com FK; `short_codes.target_type IN ('affiliate_link','product')` e remoção da FK de `target_id`; CHECKs de `click_events` e `click_daily` aceitando `event_type IN ('click','view')` e `target_type IN ('affiliate_link','product')`; `visitor_seen.event_type text NOT NULL DEFAULT 'click' CHECK(event_type IN ('click','view'))` na chave primária `(target_type, target_id, event_type, scope, channel_id, visitor_id)`; índices únicos parciais `(org_id, slug) WHERE purged_at IS NULL` em `products` e `categories`; `product_related` com CHECK `product_id <> related_product_id`. `Down` reversível.
- [ ] T006 Testar a migração em `internal/store/migrate_test.go`: sobe, desce e sobe de novo em banco com dados da fase 1 (links, eventos, `visitor_seen`) sem perdê-los; recusa slug de produto sem hífen ou fora de `^[a-z0-9]+(-[a-z0-9]+)+$`, relacionado igual ao próprio produto e slug repetido na mesma org (inclusive de item na lixeira).
- [ ] T007 [P] Criar `internal/domain/product.go` com `Product`, `Category`, `Media`, `SpecGroup`, `SpecItem`, `ProductStatus` (`draft`|`published`) e `Target` (alvo de código: `Kind` `link`|`product`, com `Link` ou `Product`); em `internal/domain/errors.go` acrescentar `ProductSlugTakenError`, `LinkAlreadyLinkedError{Product}`, `CategoryInUseError{Products}` e `MediaInvalidError`.
- [ ] T008 [P] Escrever primeiro os testes em `internal/domain/product_test.go` e depois implementar `ValidProductSlug` (`^[a-z0-9]+(-[a-z0-9]+)+$`, até 80), `ValidCategorySlug` (`^[a-z0-9]+(-[a-z0-9]+)*$`, até 60), `SuggestSlug(title, id)` (sem acentos via `golang.org/x/text/unicode/norm`; sem hífen → acrescenta `-` e os 4 primeiros caracteres do ID), `ValidateProduct` (`title` obrigatório até 200; `short_description` até 500; `long_description` até 20.000; `seo_title` até 70; `seo_description` até 160), `ValidateCategory` (`name` obrigatório até 80), `ValidateSpecs` (até 50 grupos, até 100 itens por grupo, `name` até 120, `value` até 1.000, grupo sem itens recusado), `ValidateRelated` (até 20, sem o próprio produto) e `ValidateGallery` (até 10 imagens, `alt` até 200).
- [ ] T009 Em `internal/store/codes.go` criar `ResolveTarget(ctx, code) (domain.Target, error)`: junta `short_codes` ao link (regras atuais) ou ao produto (`status='published'`, `deleted_at IS NULL`, `purged_at IS NULL`; senão `ErrNotFound`); manter `Resolve` de links como está. Generalizar `insertShortCode` para receber `target_type`. Testes de integração em `internal/store/codes_test.go` (link, produto publicado, rascunho, lixeira, excluído, código inexistente).
- [ ] T010 Em `internal/redirect/cache.go` e `internal/redirect/handler.go` trocar `AffiliateLink` por `domain.Target` na resolução e no cache (TTL de 5 min e negativo de 30 s inalterados), mantendo o fluxo de links idêntico (`302`, prévia de crawler, `Recorder`); o ramo `product` responde `404` por enquanto. Os testes existentes de `internal/redirect/*_test.go` devem continuar passando sem alteração de comportamento; acrescentar teste do cache para alvos de produto.
- [ ] T011 Em `internal/events/queue.go` acrescentar `Kind` (`click`|`view`, padrão `click`) e `TargetType` (`affiliate_link`|`product`, padrão `affiliate_link`) a `Event`; em `internal/events/writer.go` gravar `event_type` e `target_type` do evento e incluir `event_type` no `INSERT … ON CONFLICT DO NOTHING` de `visitor_seen` (unicidade de visitas independente da de cliques; robôs continuam sem tocar em `visitor_seen`). Testes em `internal/events/writer_test.go`: visita e clique do mesmo visitante/produto são ambos únicos; repetir visita não é único; concorrência com `-race` gera exatamente um único.
- [ ] T012 Corrigir `internal/stats/stats.go`: toda consulta da fase 1 (`dailyCTE`, resumos, canais, links) passa a filtrar `event_type='click' AND target_type='affiliate_link'` em `click_events` e `click_daily`. Escrever antes o teste de regressão em `internal/stats/stats_test.go`: com eventos de produto (`click` e `view`) no mesmo banco, os totais de links, canais e dia ficam idênticos aos da fase 1 (research R8).
- [ ] T013 [P] Criar o pacote `internal/media` (`media.go`): `Validate` detecta o tipo pelo conteúdo (`http.DetectContentType`; só JPEG, PNG e WebP), rejeita arquivo acima de `MEDIA_MAX_BYTES` e imagem acima de 24 megapixels lida por `image.DecodeConfig` antes de decodificar; `Process` gera variantes WebP nas larguras {320, 640, 960, 1280} limitadas à largura da origem (sempre ao menos uma), com `draw.CatmullRom` e qualidade 80, preservando transparência, sem manter o original; `Store` grava `{id[0:2]}/{id}/{largura}.webp` por arquivo temporário + `rename`; `Open` e `Remove` aceitam só UUID e largura da lista fechada. Testes em `internal/media/media_test.go` com imagens geradas no teste: JPEG, PNG com alfa, WebP, tipo inválido, grande demais, bomba de descompressão, remoção de arquivo já ausente.
- [ ] T014 Criar `internal/store/media.go` (inserir, buscar, listar órfãs com mais de 24 h sem vínculo em `product_media` nem `marketplaces.logo_media_id`, apagar) com testes de integração em `internal/store/media_test.go`; toda função recebe `orgID`.
- [ ] T015 Criar `internal/service/invalidate.go` com a interface `PageInvalidator` (`Bump(orgID)`) e fazer o `service.Catalog` existente chamá-la (e invalidar o `redirect.Cache`) ao editar, ativar/inativar, excluir ou restaurar link e ao editar ou excluir marketplace; implementação nula para testes. Testes em `internal/service/catalog_test.go`.

**Checkpoint**: `go test ./...` e `go vet ./...` passam; a migração sobe e desce; os totais da fase 1 não mudam com eventos de produto no banco; `CGO_ENABLED=0 go build ./...` compila.

---

## Phase 3: User Story 1 - Publicar uma página de produto pelo painel (Priority: P1)

**Purpose**: cadastrar, vincular links, enviar imagens, publicar e excluir produtos pelo painel e pela API

**Goal**: o administrador cadastra um produto, vincula links de afiliado, envia imagens e o publica, obtendo endereço amigável e código curto.

**Independent Test**: criar um produto com título, imagem e um link no painel, publicá-lo e conferir endereço e código; despublicar, excluir e restaurar. A abertura do endereço público fecha no checkpoint da US2.

- [ ] T016 [US1] Escrever os testes de integração de `internal/store/products_test.go`: criar com código curto próprio, slug único por org (inclusive item na lixeira e reutilização após `purged_at`), vincular e desvincular links (link de outro produto → erro com o produto atual), publicar, despublicar, lixeira, restauração, listagem com busca/filtro/paginação e isolamento por `org_id`.
- [ ] T017 [US1] Implementar `internal/store/products.go`: criar (gera `short_codes` em transação), buscar por id/slug, listar, atualizar, `SetStatus`, `SoftDelete`, `Restore`, `SetLinks` (substitui o conjunto vinculado em `affiliate_links.product_id`, recusando link já vinculado a outro produto) e `SetGallery` (`product_media`, posição 0 = imagem principal, contígua).
- [ ] T018 [US1] Escrever os testes de `internal/service/products_test.go` (sugestão e validação de slug, publicação exige título e slug válidos, **toda escrita** que afete a página (criar, editar, publicar, despublicar, excluir, restaurar, vincular ou desvincular link, trocar imagem, alterar ficha ou relacionados) invalida a entrada do código no `redirect.Cache` e chama `PageInvalidator.Bump`, com teste que percorre cada operação e confirma o `Bump`, restauração dentro do prazo, após o prazo `ErrNotFound`) e implementar `internal/service/products.go` com essas regras, usando `domain.ValidateProduct`, `ValidateGallery` e `SuggestSlug`.
- [ ] T019 [P] [US1] Escrever os testes de `internal/admin/media_test.go` e implementar `internal/admin/media.go` (`POST /admin/media`, `multipart`, CSRF): valida com `internal/media`, grava as variantes, insere a mídia sem vínculo e devolve o trecho HTMX com a miniatura e o campo oculto `media_id`; tipo inválido → `422`, acima do limite → `413`, mensagens em português.
- [ ] T020 [P] [US1] Escrever os testes e implementar `GET /media/{id}/{largura}.webp` em `internal/web/media.go`: `id` por `uuid.Parse` e largura da lista fechada; `200 image/webp` com `Cache-Control: public, max-age=31536000, immutable` e `X-Content-Type-Options: nosniff`; qualquer outra entrada ou arquivo ausente → `404`; nunca emite `Set-Cookie`.
- [ ] T021 [US1] Escrever os testes em `internal/admin/products_test.go` e implementar `internal/admin/products.go`: listagem com busca, filtro de status e paginação por HTMX; formulário de criação (título, slug sugerido, descrição curta e longa, galeria com texto alternativo, links vinculados); detalhe com endereço amigável e código curto para copiar; publicar/despublicar (confirmação ao despublicar); exclusão com confirmação e restauração; aviso ao trocar o slug de produto já divulgado; estado vazio "Cadastre seu primeiro produto".
- [ ] T022 [US1] Criar os templates `web/templates/admin/products/list.html`, `form.html`, `show.html` e `confirm_delete.html` com os componentes existentes (campos, tabela que vira cartões abaixo de 40 rem, selo de status, `unsaved.js`), `data-label` nas células, rótulos e `alt`; mostrar os links vinculados com `policy_badge` e a marca "não rastreável" nos `direct`. Item "Produtos" no menu lateral de `web/templates/layouts/admin.html`.
- [ ] T023 [P] [US1] Escrever os testes e implementar a API em `internal/api/products.go` e `internal/api/media.go` conforme `contracts/api-v1.md` (`/products` CRUD, `publish`, `unpublish`, `restore`, `media` multipart): `409` com o produto atual ao vincular link de outro produto e com o slug repetido, `422` nas validações, `404` para outra org; o objeto de produto traz `url`, `short_url`, `code` e links com `trackable`.
- [ ] T024 [US1] Registrar as rotas em `internal/server/server.go` (`/admin/products…`, `/admin/media`, `/media/…`, `/api/v1/products…`, `/api/v1/media`), com `view`/`write` e CSRF como nas demais; atualizar `internal/server/routes_test.go` e `internal/server/nocookie_test.go` para cobrir as rotas novas.
- [ ] T025 [US1] Mostrar produtos na lixeira do painel (`internal/admin/trash.go` e `web/templates/admin/trash_list.html`) e estender a expiração em `internal/maintenance/trash.go`: ao fim do prazo marcar `purged_at`, desvincular os links (`product_id = NULL`), apagar `product_media`, mídias exclusivas (linhas e arquivos), grupos, itens e relações, apagar `visitor_seen` do produto e invalidar o `redirect.Cache`. Testes em `internal/maintenance/trash_test.go`.
- [ ] T026 [US1] Estender `internal/maintenance` com a limpeza de mídia órfã (sem vínculo há mais de 24 h; apaga linha e arquivos, tolerando arquivo ausente) chamada pelo agendador diário; testes em `internal/maintenance/media_test.go`.

**Checkpoint**: no painel é possível criar, editar, vincular links, enviar imagens, publicar, despublicar, excluir e restaurar um produto (também pela API); o código curto é gerado e `/p/{slug}` ainda não é público (US2). `go test ./...` e `go vet ./...` passam.

---

## Phase 4: User Story 2 - Visitante vê o produto e vai à loja (Priority: P1)

**Purpose**: página pública renderizada no servidor, com cache, pré-visualização e botões de "Onde comprar"

**Goal**: quem chega por rede social vê a página no celular e chega à loja pelo botão certo.

**Independent Test**: com um produto publicado e dois links de lojas diferentes, abrir a página a 375 px e chegar a cada loja pelos botões.

- [ ] T027 [US2] Escrever os testes de `internal/web/pagecache_test.go` (inclusive `-race`) e implementar `internal/web/pagecache.go`: cache em memória de HTML por `(produto, segmento do canal)` com orçamento de `PAGE_CACHE_MB` e remoção do menos usado, `ETag` por hash do conteúdo e versão por organização; implementa `service.PageInvalidator` (`Bump` incrementa a versão e entradas antigas são descartadas na leitura); `ErrNotFound` nunca é cacheado como página.
- [ ] T028 [US2] Implementar `internal/web/product.go`: carrega produto, imagem principal e galeria, links ativos agrupados por marketplace (fora os inativos, na lixeira ou de marketplace na lixeira), logotipo ou nome do marketplace, e monta o modelo da página; botão `shorten` aponta para `BASE_URL[/{segmento}]/{código do link}` (com o canal da visita) e `direct` para `destination_url` com `rel="noopener noreferrer nofollow sponsored"`; converte a descrição longa em parágrafos escapados (função de template, sem HTML cru). Testes em `internal/web/product_test.go`.
- [ ] T029 [US2] Criar `web/templates/public/product.html` (visão geral com imagem principal `srcset`/`sizes`/`width`/`height`/`fetchpriority`, descrição curta, botão "Ver onde comprar", seção "Onde comprar" sem preço, descrição longa, barra fixa, aviso de privacidade) e os parciais em `web/templates/components/` (galeria, cartão de marketplace, barra fixa); seções vazias são omitidas e, sem links disponíveis, a página informa que não há lojas e oculta o botão principal. Sem imagem principal, usar `web/static/img/produto-sem-imagem.svg` (arquivo próprio, sem fonte externa), com `alt` igual ao título e dimensões declaradas, e omitir `og:image` e a imagem do JSON-LD. Teste da página sem imagem em `internal/web/product_test.go`.
- [ ] T030 [P] [US2] Acrescentar os estilos da página em `web/static/css/base.css` (mobile first, tokens existentes, modos claro e escuro, contraste AA, foco visível, barra fixa e barra de ação no rodapé do celular, `scroll-snap`) e rodar `make css`; criar `web/static/js/product.js` apenas com a revelação da barra fixa por `IntersectionObserver` (sem dependências, < 2 KB). Sem JavaScript os botões e o conteúdo continuam utilizáveis.
- [ ] T031 [US2] Registrar `GET /p/{slug}` em `internal/server/server.go` com a mesma instância do limitador de `GET /`; `200` com `ETag` e `Cache-Control: no-cache`, `304` para `If-None-Match` igual, `404` para rascunho, lixeira, excluído ou inexistente (HTML simples, sem cookie). Resolver o ramo `product` do handler de `/{código}` e `/{canal}/{código}` em `internal/redirect/handler.go` para entregar a mesma página, sem redirecionar e sem evento por enquanto; `HEAD` sem corpo. Ligar o `PageCache` como `service.PageInvalidator` do `service.Catalog` e do `service.Products` em `Build`. Teste em `internal/server/product_routes_test.go`: editar o título, trocar um link, inativar um link e renomear o marketplace mudam o `ETag` da página na abertura seguinte.
- [ ] T032 [US2] Implementar a pré-visualização `GET /admin/products/{id}/preview` em `internal/admin/products.go` (sessão exigida, aviso "Rascunho", `noindex`, sem cache e sem eventos) e o botão correspondente em `web/templates/admin/products/show.html`.
- [ ] T033 [US2] Escrever os testes de `internal/web/product_handler_test.go` e `internal/server/product_routes_test.go`: `200`/`404`/`304`; nenhuma resposta com `Set-Cookie`; HTML sem domínio externo; um botão por link, agrupados por marketplace; `direct` leva à URL original e `shorten` ao código do link com o canal da visita; link inativo ou na lixeira some; produto sem links mostra a mensagem; edição do título aparece na abertura seguinte (SC-007); `429` com `Retry-After` excedendo o limite.

**Checkpoint**: um produto publicado abre em `/p/{slug}` e em `/{código}` sem sessão e sem cookie; os botões levam às lojas pela política de cada link; a edição aparece na abertura seguinte. Passo 3 (item 4) e passo 4 do quickstart. `-race` em `internal/web` passa.

---

## Phase 5: User Story 3 - Ficha técnica e produtos relacionados (Priority: P2)

**Purpose**: grupos de especificações por produto em acordeão e carrossel de relacionados

**Goal**: cada produto tem sua própria ficha técnica e seus relacionados.

**Independent Test**: criar três grupos com itens, reordená-los, escolher dois relacionados e conferir a página.

- [ ] T034 [US3] Escrever os testes de `internal/store/products_test.go` (acréscimo) e implementar em `internal/store/products.go` `ReplaceSpecs` (substitui `product_spec_groups` e `product_spec_items` do produto numa transação, posições contíguas) e `ReplaceRelated` (`product_related`, mesma org, sem o próprio produto, até 20, posições contíguas), mais a leitura de ambos; relacionados só publicados e fora da lixeira na leitura pública.
- [ ] T035 [US3] Acrescentar à `internal/service/products.go` as operações de ficha e relacionados (usando `domain.ValidateSpecs` e `ValidateRelated`) com invalidação da página e testes em `internal/service/products_test.go`.
- [ ] T036 [US3] Criar o editor de ficha no formulário do produto (`web/templates/admin/products/form.html` e `web/templates/components/spec_editor.html`): adicionar, remover e mover grupos e itens com botões acessíveis por teclado (Alpine, sem arrastar-e-soltar), campos indexados, aviso de grupo vazio; busca HTMX de relacionados em `GET /admin/products/related-search?q=` (publicados, exceto o próprio) reordenáveis por botões. Testes em `internal/admin/products_test.go`.
- [ ] T037 [P] [US3] Estender a API (`internal/api/products.go`): `specs` e `related` em `PATCH /products/{id}` e no detalhe, com os erros `422` das validações; testes em `internal/api/products_test.go`.
- [ ] T038 [US3] Renderizar a ficha em `<details>` (um por grupo, `id` âncora, só o primeiro aberto, abre sozinho quando o `#hash` aponta para ele) e o carrossel de relacionados com `scroll-snap` e `loading="lazy"` em `web/templates/public/product.html`; acrescentar o controle "Mostrar todos" a `web/static/js/product.js`; omitir grupos sem itens e relacionados despublicados, excluídos ou sem imagem. Testes em `internal/web/product_test.go`.

**Checkpoint**: três grupos criados, reordenados e exibidos na ordem salva, com acordeão e "Mostrar todos"; relacionado despublicado some do carrossel; relacionar o produto a ele mesmo é recusado. Passo 5 do quickstart.

---

## Phase 6: User Story 4 - Medir visitas e cliques do produto (Priority: P2)

**Purpose**: beacon de visita, clique do código curto, estatísticas separadas e retenção

**Goal**: o administrador vê, por produto, visitas e cliques em separado, por canal e por dia.

**Independent Test**: abrir a página três vezes (dois visitantes) e usar o link curto uma vez; o painel mostra 3 visitas (2 únicas) e 1 clique.

- [ ] T039 [US4] Escrever os testes de `internal/api/beacon_test.go` (inclusive `-race`) e implementar `internal/api/beacon.go` (`POST /api/v1/beacon/view`): corpo `text/plain` de até 512 bytes com `{"k","c"}`; `204` sempre, sem corpo e sem cookie; resolve o código pelo `redirect.Cache` e o canal pelo cache de canais; ignora crawler, código inexistente, de link ou de produto não publicado e canal inexistente; enfileira `Event{Kind: view, TargetType: product}` com o mesmo `VisitorID`, marcando `is_bot` pelo `BotDetector`; instância própria do limitador (`429` ao exceder). Registrar a rota em `internal/server/server.go` antes do coringa `GET /api/v1/`.
- [ ] T040 [US4] Em `internal/redirect/record.go` acrescentar o registro do `click` de produto (`TargetType: product`) quando o código curto do produto é aberto (também com canal), ignorando crawlers e `HEAD`, e chamar esse registro no ramo `product` de `internal/redirect/handler.go`; nunca bloqueia a resposta. Testes em `internal/redirect/record_test.go` e `handler_integration_test.go`: um clique por abertura, nenhum para crawler, descarte contabilizado com a fila cheia.
- [ ] T041 [P] [US4] Em `web/static/js/product.js` enviar o beacon uma vez por carregamento (`navigator.sendBeacon` com `Blob` `text/plain`, falha silenciosa), lendo `data-k` e `data-c` do elemento raiz de `web/templates/public/product.html`; a página continua funcionando sem JavaScript.
- [ ] T042 [US4] Escrever os testes de integração de `internal/stats/products_test.go` e implementar `internal/stats/products.go`: `ProductsList` (visitas e cliques, totais e únicos, do período), `ProductDetail` (séries diárias, quebra por canal, links com `trackable` e **sem** contadores nos `direct`) e `Summary` com `page_visits` e `page_visits_unique`; dias fechados vêm de `click_daily`, o dia corrente de `click_events`; unicidade do produto = soma de `unique_target`, por canal = `unique_url`.
- [ ] T043 [US4] Mostrar visitas e cliques separados no detalhe e na listagem do produto (`internal/admin/products.go`, `web/templates/admin/products/show.html` e `list.html`, com o seletor de período e o gráfico existentes) e o cartão "Visitas às páginas" em `internal/admin/dashboard.go` e `web/templates/admin/dashboard.html`; o cartão "Cliques" continua sendo só de links.
- [ ] T044 [P] [US4] Expor as estatísticas na API: `GET /api/v1/stats/products`, `GET /api/v1/stats/products/{id}` e os novos campos de `GET /api/v1/stats/summary` em `internal/api/stats.go`, com testes em `internal/api/stats_test.go` (período inválido → `422`, outra org → `404`).
- [ ] T045 [US4] Testes de ponta a ponta em `internal/server/product_events_test.go` com PostgreSQL real: abrir a página duas vezes com o mesmo visitante → 2 visitas e 1 único; trocar o user-agent → novo único; clique do código curto separado da visita; crawler e `is_bot` fora das estatísticas; link `direct` sem contadores; consolidação diária e limpeza (`internal/maintenance`) preservam e não alteram visitas e cliques de produto, e reexecutar a consolidação não muda os valores.

**Checkpoint**: 3 visitas (2 únicas) e 1 clique aparecem separados no painel e na API; crawler e robô não contam; `direct` continua "não rastreável". Passo 6 do quickstart. `-race` em `internal/api`, `internal/redirect` e `internal/events` passa.

---

## Phase 7: User Story 5 - Compartilhar a página e ser encontrado (Priority: P2)

**Purpose**: metadados de compartilhamento, SEO, dados estruturados e texto de privacidade

**Goal**: o endereço colado numa rede social mostra uma boa prévia e a página é indexável.

**Independent Test**: inspecionar o HTML e colar o endereço amigável e o código curto num validador de prévia.

- [ ] T046 [US5] Escrever os testes de `internal/web/product_seo_test.go` e implementar o `<head>` da página (`web/templates/public/product.html` e `internal/web/product.go`): `<title>`, `meta description`, `link rel="canonical"` = `BASE_URL/p/{slug}` também quando aberta pelo código curto, Open Graph (`og:type=product`, `og:title`, `og:description`, `og:url` canônico, `og:image` de 1280 px) e `twitter:card=summary_large_image`; um único `<h1>` e hierarquia `h2`/`h3` correta.
- [ ] T047 [US5] Emitir o JSON-LD `schema.org/Product` (`name`, `description`, `image`, `category`) **sem** `offers`, preço, avaliação ou ranking, escapado por `html/template` (`internal/web/product.go`); teste que falha se `offers` ou `price` aparecerem.
- [ ] T048 [US5] Campos de SEO no formulário e na API (`seo_title` até 70, `seo_description` até 160); quando em branco, as metatags usam título e descrição curta (FR-032). Testes em `internal/web/product_seo_test.go` e `internal/admin/products_test.go`.
- [ ] T049 [US5] Garantir que crawlers conhecidos recebem a página completa com os metadados, em `/p/{slug}`, `/{código}` e `/{canal}/{código}`, sem gerar visita nem clique (`internal/redirect/handler_integration_test.go` e `internal/api/beacon_test.go`).
- [ ] T050 [P] [US5] Atualizar `/privacidade` (`web/templates/public/privacy.html` e `internal/web/privacy.go`) com a contagem de aberturas de página de produto pelo mesmo identificador anônimo, a mesma retenção dos cliques e a limitação para navegadores sem JavaScript; ajustar `internal/web/privacy_test.go`.

**Checkpoint**: a prévia mostra título, imagem e descrição nos dois endereços e o canônico é sempre o amigável; o JSON-LD não contém oferta nem preço. Passo 7 do quickstart.

---

## Phase 8: User Story 6 - Categorias e logotipo de marketplace (Priority: P3)

**Purpose**: agrupar produtos em categorias e identificar lojas com logotipo

**Goal**: produtos organizados em categorias e lojas reconhecíveis pelo logotipo.

**Independent Test**: criar uma categoria, associá-la a um produto, definir o logotipo de um marketplace e conferir painel e página.

- [ ] T051 [US6] Escrever os testes de `internal/store/categories_test.go` e implementar `internal/store/categories.go` (CRUD por `orgID`, slug único com reserva na lixeira, exclusão lógica; ir para a lixeira só sem produtos fora da lixeira, devolvendo `CategoryInUseError` com os produtos; restaurar da lixeira) e a regra em `products.go`: restaurar produto cuja categoria está na lixeira é bloqueado.
- [ ] T052 [US6] Implementar `internal/service/categories.go` com testes (sugestão de slug, `domain.ValidateCategory`, invalidação das páginas), a API `internal/api/categories.go` conforme `contracts/api-v1.md` (`409` com os produtos que impedem) e a expiração da lixeira de categorias em `internal/maintenance/trash.go`.
- [ ] T053 [US6] Painel de categorias: `internal/admin/categories.go` e `web/templates/admin/categories/{list,form,confirm_delete}.html` no padrão dos canais, mensagem que lista os produtos que impedem a exclusão, categorias na lixeira de `internal/admin/trash.go`, item "Categorias" no menu; registrar as rotas em `internal/server/server.go` e cobrir em `internal/admin/categories_test.go`.
- [ ] T054 [US6] Campo de categoria no formulário do produto e na API, filtro por categoria na listagem de produtos (painel e API) e categoria nos dados estruturados da página; testes em `internal/admin/products_test.go` e `internal/api/products_test.go`.
- [ ] T055 [US6] Logotipo de marketplace: `marketplaces.logo_media_id` em `internal/store/marketplaces.go` e `internal/service/catalog.go` (vincula/remove, invalida páginas), envio e prévia em `web/templates/admin/marketplaces/form.html`, `logo_media_id` em `PATCH /marketplaces/{id}` (`internal/api/catalog.go`) e exibição na seção "Onde comprar" (nome quando sem logotipo). Testes em `internal/store/marketplaces_test.go`, `internal/admin/marketplaces_test.go` e `internal/web/product_test.go`.

**Checkpoint**: categoria criada, associada e filtrável; exclusão com produtos bloqueada com a lista; logotipo aparece na página e o nome quando ausente.

---

## Phase 9: Polish e endurecimento

**Purpose**: carga, métricas, privacidade, documentação e validação final

- [ ] T056 [P] Métricas em `internal/metrics` e `internal/web`: acertos e erros do cache de páginas, tamanho do cache, visitas enfileiradas e descartadas, envios e falhas de imagem; testes em `internal/metrics/metrics_test.go`.
- [ ] T057 [P] Estender a auditoria de privacidade em `internal/audit` para os handlers novos (página, beacon, mídia): nenhum `Set-Cookie`, nenhum IP em log ou coluna, nenhum recurso de terceiros nos templates públicos.
- [ ] T058 Criar `scripts/seed-products.sh` (1.000 produtos publicados com links e uma imagem de teste) e adaptar `scripts/loadtest.sh` para a página de produto; executar de verdade 200 aberturas/s por 60 s e registrar p95 no servidor (< 100 ms), erros (zero) e descarte (< 0,1 %) em `docs/operacao.md` (SC-004, SC-011).
- [ ] T059 [P] Atualizar `docs/operacao.md`: variáveis novas, backup e restauração do volume `hublinks-media`, a migração `00004` (reconstrução da chave de `visitor_seen`), limpeza de mídia órfã e o que fazer com o cache de páginas; atualizar `docs/pendencias.md` com as assunções da fase 2 e `docs/guia-de-estilo.md` com os componentes novos (galeria, cartão de marketplace, acordeão, carrossel, barra fixa, editor de ficha), exibindo-os em `/admin/ui`.
- [ ] T060 Executar o `quickstart.md` completo (passos 1 a 10): teste automatizado, navegador a 375 px nos dois temas e só com teclado, Lighthouse (LCP < 2,5 s, CLS < 0,1, INP < 200 ms), `docker compose up --build` com o volume e a persistência após recriar o contêiner, `MEDIA_DIR` sem permissão impedindo a partida. Registrar o resultado e o que depender de uma pessoa ou do Docker em `docs/verificacao.md`; não declarar aprovado o que não foi executado.
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
- `internal/server/server.go` é editado por várias tarefas: não rode em paralelo tarefas que o
  alterem.

## Parallel Opportunities

- Setup: as tarefas [P] (T002 e T003) em paralelo.
- Foundational: domínio (T007, T008) e `internal/media` (T013) em paralelo
  com a migração.
- US1: envio de mídia (T019), rota `/media` (T020) e API (T023) em paralelo.
- US3 a US6 em paralelo após a US2, por equipes ou sessões diferentes, cuidando dos conflitos em
  `server.go` e `product.html`.

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
