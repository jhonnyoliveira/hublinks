# Contrato: API administrativa `/api/v1` (Fase 2)

Acrescenta-se ao contrato da fase 1 (`specs/001-encurtador-analytics/contracts/api-v1.md`), com as
mesmas regras de autenticação por cookie de sessão, CSRF, escopo por organização, formato de erro,
listagens (`?q=&page=&per_page=&trash=`) e período (`?period=` ou `?from=&to=`).
O beacon público está em [public-routes.md](public-routes.md). Mídia usa `multipart/form-data`.

## Produtos

| Método e rota | Corpo / efeito | Sucesso |
|---|---|---|
| `POST /products` | `{title, slug?, short_description?, long_description?, category_id?, seo_title?, seo_description?}`. `slug` ausente → sugerido. Slug repetido (inclusive na lixeira) → `409`; inválido → `422`. Nasce em `draft` | `201` com o produto |
| `GET /products` | filtros: `q`, `status` (`draft`\|`published`), `category_id`, `trash`. Cada item traz visitas e cliques do `period` | `200` |
| `GET /products/{id}` | detalhe completo: textos, imagens, grupos e itens, relacionados, links vinculados | `200` |
| `PATCH /products/{id}` | qualquer campo acima; `media` (lista ordenada `[{media_id, alt?}]`, no máximo 10); `specs` (`[{name, items:[{name,value}]}]`, substitui tudo); `related` (`[product_id]`, ordenados, substitui tudo); `links` (`[affiliate_link_id]`, substitui o conjunto vinculado; link de outro produto → `409` com `{"product":{id,title}}`) | `200` |
| `POST /products/{id}/publish` | publica; sem título/slug válidos → `422` | `200` |
| `POST /products/{id}/unpublish` | volta a `draft` | `200` |
| `DELETE /products/{id}` | envia à lixeira (código curto e URLs passam a `404`) | `204` |
| `POST /products/{id}/restore` | restaura da lixeira; após o prazo → `404`; categoria na lixeira → `409` com a categoria | `200` |

Produto: `{id, slug, title, short_description, long_description, category, seo_title, seo_description,
status, code, url, short_url, images:[{media_id, alt, urls:{"320":"/media/…"}}], specs, related, links:[{id,
title, marketplace, policy, trackable}], published_at, deleted_at, created_at, updated_at}`.
`url` = `BASE_URL/p/{slug}`; `short_url` = `BASE_URL/{code}`. Links `direct` aparecem com
`trackable:false` e sem contadores.

## Categorias

| Método e rota | Corpo / efeito | Sucesso |
|---|---|---|
| `POST /categories` | `{name, slug?}`; slug repetido (inclusive na lixeira) → `409` | `201` |
| `GET /categories` | filtros: `q`, `trash`; cada item traz `products_count` | `200` |
| `GET /categories/{id}` | | `200` |
| `PATCH /categories/{id}` | `{name?, slug?}` | `200` |
| `DELETE /categories/{id}` | com produtos fora da lixeira → `409` com `{"products":[{id,title}]}` | `204` |
| `POST /categories/{id}/restore` | restaura; após o prazo → `404` | `200` |

## Mídia

| Método e rota | Corpo / efeito | Sucesso |
|---|---|---|
| `POST /media` | `multipart`: `file`, `kind` (`product`\|`marketplace_logo`). JPEG, PNG ou WebP até `MEDIA_MAX_BYTES` e 24 MP; tipo inválido → `422`; grande demais → `413` | `201` `{id, width, height, urls:{"320":"/media/{id}/320.webp",…}}` |

A mídia nasce sem vínculo e é vinculada ao salvar produto (`media`) ou marketplace
(`PATCH /marketplaces/{id}` aceita `logo_media_id`, ou `null` para remover). Mídia sem vínculo por
mais de 24 h é apagada pela manutenção.

## Estatísticas de produto

| Método e rota | Resposta |
|---|---|
| `GET /stats/products` | por produto: `visits`/`visits_unique` e `clicks`/`clicks_unique` do período, mais totais; filtros `q`, `category_id`; paginação |
| `GET /stats/products/{id}` | totais separados de visitas e cliques; série diária de cada um; quebra por canal (`channel` nulo = sem canal); lista de links do produto com `trackable` e, para `direct`, **sem** contadores |
| `GET /stats/summary` | passa a incluir `page_visits` e `page_visits_unique` do período; `clicks` e `unique` continuam sendo só os de links de afiliado |

Unicidade: `visits_unique`/`clicks_unique` por produto = soma de `unique_target` (alcance); por
canal = `unique_url`, como nos links da fase 1.
