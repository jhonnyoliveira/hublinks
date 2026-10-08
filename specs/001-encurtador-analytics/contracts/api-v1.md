# Contrato: API administrativa `/api/v1` (JSON)

- **Autenticação**: o mesmo cookie de sessão do painel. Sem sessão → `401`.
- **CSRF**: métodos que alteram dados exigem o cabeçalho `X-CSRF-Token` com o token da sessão e
  origem igual ao site; caso contrário → `403`.
- **Escopo**: todas as operações valem só para a organização da sessão. Um ID de outra org
  responde `404`.
- **Formato**: `Content-Type: application/json`; datas em RFC 3339 UTC; dias em `YYYY-MM-DD` no
  fuso de relatório.
- **Erro**: `{"error":{"code":"validation_failed","message":"...","fields":{"segment":"já em uso"}}}`.
  Códigos de erro: `validation_failed` (422), `not_found` (404), `conflict` (409),
  `unauthorized` (401), `forbidden` (403), `rate_limited` (429).
- **Listagens**: `?q=&page=1&per_page=20` (máximo 100); o padrão exclui itens na lixeira, e
  `?trash=true` mostra só a lixeira. Resposta: `{"items":[...],"page":1,"per_page":20,"total":N}`.
- **Período** (rotas de estatística): `?period=7d|30d|90d` ou `?from=YYYY-MM-DD&to=YYYY-MM-DD`
  (inclusivo). `from > to`, ou mais de 366 dias → `422`. Padrão: `30d`.

## Marketplaces

| Método e rota | Corpo / efeito | Sucesso |
|---|---|---|
| `POST /marketplaces` | `{name, shorten_policy}` | `201` com o marketplace |
| `GET /marketplaces` | filtros: `q`, `trash` | `200` |
| `GET /marketplaces/{id}` | | `200` |
| `PATCH /marketplaces/{id}` | `{name?, shorten_policy?}` | `200` |
| `DELETE /marketplaces/{id}` | envia à lixeira; com links ativos → `409` com `{"links":[{id,title}]}` | `204` |
| `POST /marketplaces/{id}/restore` | restaura da lixeira; após 30 dias → `404` | `200` |

Marketplace: `{id, name, shorten_policy, deleted_at, created_at, updated_at}`.

## Canais

| Método e rota | Corpo / efeito | Sucesso |
|---|---|---|
| `POST /channels` | `{name, segment}`. Segmento duplicado (inclusive de canal na lixeira) → `409`; reservado ou inválido → `422` | `201` |
| `GET /channels` | filtros: `q`, `trash` | `200` |
| `GET /channels/{id}` | inclui `active_links_count` (links fora da lixeira e ativos cujas URLs deste canal serão invalidadas) | `200` |
| `PATCH /channels/{id}` | `{name?, segment?}`, com as mesmas validações do `POST` | `200` |
| `DELETE /channels/{id}` | envia à lixeira, sem trava por links (FR-008c) | `204` |
| `POST /channels/{id}/restore` | restaura da lixeira; após o prazo → `404` | `200` |

Canal: `{id, name, segment, active_links_count, deleted_at, created_at, updated_at}`.

## Links de afiliado

Mesmas operações em `/affiliate-links`, com corpo:
`{title, destination_url, marketplace_id, image_url?, shorten_policy_override?, active?}`.

- Filtros de listagem: `q`, `marketplace_id`, `active`, `trash`.
- `POST` gera o código curto automaticamente.
- Restaurar exige o marketplace fora da lixeira; caso contrário → `409`.

Link:

```json
{
  "id": "…", "title": "…", "destination_url": "https://…", "image_url": null,
  "marketplace": {"id": "…", "name": "…"},
  "shorten_policy_override": null, "effective_policy": "shorten",
  "trackable": true, "active": true, "code": "3p4mj5a",
  "urls": [
    {"channel": null, "label": "curta", "url": "https://hub.example/3p4mj5a"},
    {"channel": {"id": "…", "segment": "wapp", "name": "WhatsApp"}, "url": "https://hub.example/wapp/3p4mj5a"}
  ],
  "deleted_at": null, "created_at": "…", "updated_at": "…"
}
```

Com `effective_policy = direct`: `trackable: false` e `urls` contém só
`{"label":"original","url": destination_url}`.

## Estatísticas

Os contadores excluem robôs. Para links não rastreáveis, os contadores de cliques vêm como
`null` e a resposta traz `"trackable": false`, nunca `0`. Dados históricos de quando o link
era `shorten` vêm em `history`.

- `GET /stats/links?period=…&marketplace_id=&q=&page=`
  → `{"items":[{"link":{id,title,code,marketplace,trackable},"clicks":98,"unique":71}], …}`.
  O campo `unique` é o alcance por alvo (`SUM(unique_target)`).
- `GET /stats/links/{id}?period=…`
  → `{"link":{…},"totals":{"clicks":185,"unique":140},`
    `"by_channel":[{"channel":null|{…},"url":"…","clicks":98,"unique":71}],`
    `"daily":[{"day":"2026-10-01","clicks":12,"unique":9}]}`.
  No detalhe por canal, `unique` = `SUM(unique_url)`.
- `GET /stats/channels?period=…`
  → `{"items":[{"channel":null|{…},"clicks":…,"unique":…}]}`, com `unique` = `SUM(unique_url)`.
- `GET /stats/summary?period=…` (dashboard)
  → `{"clicks":…,"unique":…,"active_links":…,"top_links":[…5],"daily":[…]}`.
