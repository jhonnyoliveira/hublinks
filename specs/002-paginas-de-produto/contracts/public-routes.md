# Contrato: rotas públicas (Fase 2)

Acrescenta-se ao contrato da fase 1 (`specs/001-encurtador-analytics/contracts/public-routes.md`),
que continua valendo para `/{code}` de links, `/privacidade`, `/healthz` e `/static/`.

Regras comuns a todas as respostas desta página:
- Nunca contêm `Set-Cookie` (FR-013 da fase 1; SC-008).
- Nenhum recurso de terceiros: HTML, CSS, JavaScript, imagens e fontes vêm do próprio serviço.
- Limite de `PUBLIC_RATE_LIMIT` requisições por minuto por IP em páginas e beacon; excedente → `429`
  com `Retry-After`, sem gerar evento. `/media/` não tem limite (R13).

## `GET /p/{slug}`

`slug` = `^[a-z0-9]+(-[a-z0-9]+)+$`.

| Situação | Resposta | Evento |
|---|---|---|
| Produto publicado e fora da lixeira | `200` HTML da página | nenhum (a visita vem do beacon) |
| Rascunho, lixeira, excluído definitivamente ou inexistente | `404` HTML simples | não |
| `If-None-Match` igual ao `ETag` atual | `304` sem corpo | nenhum |

Cabeçalhos: `ETag`, `Cache-Control: no-cache`, `Content-Type: text/html; charset=utf-8`.

## `GET /{code}` e `GET /{segment}/{code}` quando o código é de um produto

Mesma resolução da fase 1; o alvo é um produto.

| Situação | Resposta | Evento |
|---|---|---|
| Produto publicado, acesso real | `200` mesma página, **sem redirecionar** | `click` (`target_type=product`), enfileirado |
| Acesso real classificado como automação | `200` | `click` com `is_bot=true` |
| Crawler conhecido | `200` mesma página | nenhum |
| `HEAD` | `200` sem corpo | nenhum |
| Produto não publicado, na lixeira ou excluído; canal inexistente | `404` | não |
| Fila cheia ou worker parado | `200` | descartado e contabilizado |

Cabeçalhos: iguais aos de `/p/{slug}` (`no-cache` + `ETag`), para que toda abertura chegue ao
servidor e seja contada.

## Conteúdo da página (todas as formas de acesso)

- `<title>` e `meta description` (SEO do produto, com fallback para título e descrição curta).
- `link rel="canonical"` = `BASE_URL/p/{slug}`, inclusive pelo código curto.
- Open Graph: `og:type=product`, `og:title`, `og:description`, `og:url` (o canônico),
  `og:image` (variante de 1280 px), `twitter:card=summary_large_image`.
- JSON-LD `schema.org/Product` com `name`, `description`, `image`, `category`; **sem** `offers`,
  preço, avaliação ou ranking.
- Um único `<h1>`; seções com `<h2>` e grupos da ficha com `<h3>` dentro de `<details>`.
- Botões de "Onde comprar": link `shorten` → `BASE_URL[/{segment}]/{code-do-link}` (com o canal da
  visita, quando houver); link `direct` → `destination_url` com `rel="noopener noreferrer nofollow
  sponsored"`. Sem preço.
- `<script src="/static/js/product.js" defer>` com `data-k` (código do produto) e `data-c` (segmento
  do canal ou vazio) no elemento raiz; aviso de privacidade da fase 1 com link para `/privacidade`.

## `POST /api/v1/beacon/view`

Público (sem sessão). Corpo `text/plain`: `{"k":"<código>","c":"<segmento ou vazio>"}`, até 512 bytes.

| Situação | Resposta | Evento |
|---|---|---|
| Código de produto publicado, visitante real | `204` | `view`, enfileirado |
| Código de produto, automação detectada | `204` | `view` com `is_bot=true` |
| Crawler conhecido, corpo inválido, código inexistente ou de link, canal inexistente | `204` | nenhum |
| Excedeu o limite de taxa | `429` | não |
| `GET` em `/api/v1/beacon/view` | resposta do coringa `/api/v1/` da área administrativa (`401` sem sessão), sem evento | não |
| Outro método (`PUT`, `DELETE`…) | `405` | não |

Sempre sem corpo e sem `Set-Cookie`. Falha do envio é silenciosa no navegador.

## `GET /media/{id}/{largura}.webp`

`id` = UUID; `largura` ∈ {320, 640, 960, 1280}.

| Situação | Resposta |
|---|---|
| Arquivo existe | `200 image/webp`, `Cache-Control: public, max-age=31536000, immutable`, `X-Content-Type-Options: nosniff` |
| `id` ou `largura` inválidos, ou arquivo ausente | `404` |

Sem sessão, sem cookie, sem limite de taxa.

## `GET /privacidade`

Mesma rota da fase 1; o texto passa a descrever a contagem de aberturas de página de produto, a
mesma retenção dos cliques e a limitação para navegadores sem JavaScript.
