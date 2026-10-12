# Modelo de dados (Fase 2)

Convenções da fase 1 mantidas: IDs `uuid` (v7 gerados na aplicação; `media.id` é v4, ver R2),
datas `timestamptz` em UTC, nomes em inglês, toda tabela de domínio com `org_id` e toda consulta
filtrando por `org_id`. `NIL` = `00000000-0000-0000-0000-000000000000`. Uma única migração nova,
`migrations/00004_products.sql`, com `Down` reversível.

Ciclo de vida de produtos e categorias (igual ao dos demais itens da fase 1):

```text
ativo ──excluir──▶ lixeira (deleted_at) ──30 dias (job)──▶ excluído definitivamente (purged_at)
  ▲                    │
  └─────restaurar──────┘
```

Produto tem ainda o estado de publicação `draft | published`, independente da lixeira. Só
`published` e fora da lixeira é visível ao público.

## categories

| Campo | Tipo | Regras |
|---|---|---|
| id | uuid PK | |
| org_id | uuid FK organizations | |
| name | text | obrigatório, até 80 caracteres |
| slug | text | `^[a-z0-9]+(-[a-z0-9]+)*$`, até 60; único por `org_id` enquanto `purged_at IS NULL` (permanece reservado na lixeira) |
| created_at, updated_at | timestamptz | |
| deleted_at, purged_at | timestamptz NULL | exclusão lógica e definitiva |

Regra: não pode ir para a lixeira se houver produto **fora da lixeira** nela (FR-008); a resposta
lista os produtos que impedem. Restaurar um produto cuja categoria está na lixeira é bloqueado até
restaurar a categoria (mesmo padrão de link × marketplace da fase 1).

## products

| Campo | Tipo | Regras |
|---|---|---|
| id | uuid PK | |
| org_id | uuid FK organizations | |
| slug | text | `^[a-z0-9]+(-[a-z0-9]+)+$` (ao menos um hífen), até 80; índice único `(org_id, slug) WHERE purged_at IS NULL` |
| title | text | obrigatório, até 200 |
| short_description | text | até 500; vazio permitido |
| long_description | text | até 20.000, texto simples |
| category_id | uuid NULL FK categories | no máximo uma categoria |
| seo_title, seo_description | text NULL | em branco usa título e descrição curta (FR-032); até 70 e 160 |
| status | text | `draft` \| `published`; padrão `draft` |
| published_at | timestamptz NULL | primeira publicação |
| created_at, updated_at | timestamptz | |
| deleted_at, purged_at | timestamptz NULL | exclusão lógica e definitiva |

Índices: `(org_id, status) WHERE deleted_at IS NULL AND purged_at IS NULL` (listagens e relacionados) e
`(org_id, category_id)`.

Regras: título e slug são validados no domínio (`internal/domain`); trocar o slug não redireciona o
antigo (assunção da spec). Despublicar não exige desvincular nada. A exclusão definitiva marca
`purged_at`, desvincula os links (`product_id = NULL`), apaga `product_media` e as mídias
exclusivas, grupos, itens e relações, e apaga `visitor_seen` do produto.

## short_codes (alterada)

- `target_type` passa de `CHECK(target_type='affiliate_link')` para
  `CHECK(target_type IN ('affiliate_link','product'))`.
- A FK `target_id → affiliate_links(id)` é removida (tabela polimórfica); a coerência é garantida
  pelo serviço, dentro da transação que cria o alvo, e por teste de integração.
- Índice único `(target_type, target_id)` e geração de código com até 5 tentativas: inalterados.

## affiliate_links (alterada)

| Campo | Mudança |
|---|---|
| product_id | **nova** `uuid NULL FK products(id)`; no máximo um produto por link; índice `(org_id, product_id)` |

Vincular um link que já tem outro `product_id` é recusado com o nome do produto atual (FR-005).
Mudar `product_id` conta como edição de página (incrementa a versão do cache, R5).

## marketplaces (alterada)

| Campo | Mudança |
|---|---|
| logo_media_id | **nova** `uuid NULL FK media(id)`; sem logotipo, a página mostra o nome |

## media

| Campo | Tipo | Regras |
|---|---|---|
| id | uuid PK | v4 aleatório; parte da URL pública |
| org_id | uuid FK organizations | |
| kind | text | `product` \| `marketplace_logo` |
| width, height | integer | dimensões da maior variante gerada |
| widths | integer[] | larguras geradas, subconjunto de {320, 640, 960, 1280} |
| bytes | bigint | soma dos arquivos, para operação |
| created_at | timestamptz | |

Arquivos em `MEDIA_DIR/{id[0:2]}/{id}/{largura}.webp`. Sem exclusão lógica: a mídia é removida
(linha e arquivos) quando não tem vínculo por mais de 24 h ou quando o dono é excluído
definitivamente. Restaurar da lixeira não precisa de mídia nova, pois o vínculo permanece.

## product_media

| Campo | Tipo | Regras |
|---|---|---|
| org_id | uuid | |
| product_id | uuid FK products | |
| media_id | uuid FK media | |
| position | integer | 0 = imagem principal; contígua a partir de 0 |
| alt | text | até 200; vazio usa o título do produto |

PK `(product_id, media_id)`; índice único `(product_id, position)`. No máximo 10 imagens por produto.

## product_spec_groups e product_spec_items

| Tabela | Campos |
|---|---|
| product_spec_groups | `id`, `org_id`, `product_id` FK, `name` (≤ 120), `position` |
| product_spec_items | `id`, `org_id`, `group_id` FK ON DELETE CASCADE, `name` (≤ 120), `value` (≤ 1.000, texto simples), `position` |

Posições contíguas por pai. Tetos técnicos: 50 grupos por produto e 100 itens por grupo (R10).
Salvar substitui os grupos e itens do produto numa transação. Grupo sem itens é recusado no
formulário (edge case da spec); na página só aparecem grupos com ao menos um item.

## product_related

| Campo | Tipo | Regras |
|---|---|---|
| org_id | uuid | |
| product_id | uuid FK products | |
| related_product_id | uuid FK products | diferente de `product_id` (CHECK) e da mesma org |
| position | integer | contígua; no máximo 20 por produto |

PK `(product_id, related_product_id)`. A relação é direcional (A relaciona B não implica B relaciona
A). Na página só aparecem relacionados `published` e fora da lixeira.

## click_events, click_daily e visitor_seen (alteradas)

| Tabela | Mudança |
|---|---|
| click_events | `event_type` aceita `click` e `view`; `target_type` aceita `affiliate_link` e `product`. Para `view`, `code` é o código do produto e `channel_id` vem do beacon |
| click_daily | as mesmas duas CHECKs relaxadas; a chave primária já contém `event_type` e `target_type` |
| visitor_seen | **nova coluna** `event_type text NOT NULL DEFAULT 'click' CHECK(event_type IN ('click','view'))`, incluída na chave primária: `(target_type, target_id, event_type, scope, channel_id, visitor_id)` |

Semântica das flags para `view`: `is_unique_url` = primeira visita do visitante àquele produto
naquele canal; `is_unique_target` = primeira visita àquele produto em qualquer canal (igual à dos
cliques). Visitas e cliques de produto têm unicidade independente. Robôs gravam sem tocar em
`visitor_seen`. Agregação, retenção (395 dias; 30 para robôs) e limpeza valem sem mudança de regra.

### Mapa de eventos da fase 2

| Situação | event_type | target_type | target_id | channel |
|---|---|---|---|---|
| Clique no botão "Onde comprar" (link `shorten`) | `click` | `affiliate_link` | link | do botão |
| Abertura pelo código curto do produto | `click` | `product` | produto | da URL |
| Beacon da página aberta | `view` | `product` | produto | do beacon |
| Botão de link `direct` | nenhum | | | |
| `/p/{slug}` aberto direto | apenas `view` | `product` | produto | nenhum |

## Consultas de estatística: regra obrigatória

Toda consulta de `internal/stats` da fase 1 passa a filtrar `event_type='click' AND
target_type='affiliate_link'`; as novas consultas de produto filtram por `target_type='product'` e
separam `event_type`. Ver R8.

## Validações de domínio (`internal/domain`)

| Regra | Onde |
|---|---|
| slug de produto e de categoria (padrões e tamanho) | `ValidProductSlug`, `ValidCategorySlug` |
| sugestão de slug a partir do título (sem acentos; hífen garantido) | `SuggestSlug(title, id)` |
| limites de tamanho dos textos e da ficha | `ValidateProduct`, `ValidateSpecs` |
| relacionado diferente do próprio produto e do mesmo org | `ValidateRelated` |
| imagem: tipo por conteúdo, bytes, megapixels | `internal/media` |
