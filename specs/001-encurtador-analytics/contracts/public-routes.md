# Contrato: rotas públicas

Regras comuns a todas as respostas desta página:
- Nunca contêm `Set-Cookie` (FR-013).
- Limite de 300 requisições/min por IP; o excedente recebe `429` com `Retry-After`, sem
  redirecionar e sem gerar evento (FR-034).
- Nenhum recurso de terceiros (FR-033).

## `GET /{code}` e `GET /{segment}/{code}`

`code` = `^[0-9a-z]{7}$`; `segment` = segmento de canal da organização dona do código.

| Situação | Resposta | Evento |
|---|---|---|
| Código inexistente, formato inválido, link inativo, na lixeira ou excluído | `404` HTML simples | não |
| `segment` inexistente, na lixeira ou excluído | `404` | não |
| User-agent de crawler conhecido | `200` HTML de prévia (ver abaixo) | não |
| Política efetiva `direct` | `302 Location: destination_url` | não |
| Acesso real, política `shorten` | `302 Location: destination_url` | `click`, enfileirado |
| Acesso real classificado como automação (5 códigos distintos em 10 s) | `302` | `click` com `is_bot=true` |
| Fila cheia ou worker parado | `302` | descartado e contabilizado |

O redirecionamento usa sempre `302` (nunca `301`) e envia `Cache-Control: no-store` e
`Referrer-Policy: no-referrer-when-downgrade`.

### HTML de prévia

- `<meta name="robots" content="noindex">`.
- Metatags: `og:title` (título), `og:description` ("Disponível em {marketplace}"), `og:image`
  (quando houver), `og:url` (URL acessada, absoluta a partir de `BASE_URL`), `og:type=website`,
  `twitter:card` (`summary_large_image` com imagem, `summary` sem).
- Corpo: título, imagem, botão para o destino e aviso de privacidade com link para
  `/privacidade` (descarte em `localStorage`).

## `GET /privacidade`

`200` HTML com a identificação anônima (HMAC de IP + user-agent, sem cookies), a finalidade, os
prazos de retenção (lidos da configuração) e o contato para pedir exclusão. Inclui o aviso.

## `GET /healthz`

- `200 {"status":"ok"}` quando o banco responde em até 1 s.
- `503 {"status":"unavailable","checks":{"database":"error"}}` caso contrário.
- Sem limite de taxa.

## `GET /static/{path}`

Arquivos embutidos. Nomes com hash recebem `Cache-Control: public, max-age=31536000, immutable`.
