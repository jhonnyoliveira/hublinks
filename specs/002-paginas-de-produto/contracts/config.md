# Contrato: configuração por ambiente (Fase 2)

Acrescenta-se ao contrato da fase 1 (`specs/001-encurtador-analytics/contracts/config.md`). Valor
inválido ou diretório sem permissão de escrita faz a aplicação abortar na partida, citando a variável.

| Variável | Obrigatória | Padrão | Descrição |
|---|---|---|---|
| `MEDIA_DIR` | não | `./var/media` (compose: `/data/media`) | Pasta das imagens enviadas; criada se não existir; deve ser gravável. **Entra no backup** |
| `MEDIA_MAX_BYTES` | não | `5242880` | Tamanho máximo de um envio de imagem (5 MiB) |
| `PAGE_CACHE_MB` | não | `64` | Orçamento de memória do cache de páginas renderizadas |

`deploy/docker-compose.yml` monta o volume nomeado `hublinks-media` em `/data/media` e define
`MEDIA_DIR=/data/media` no serviço `app`. `deploy/.env.example` lista as três variáveis.
