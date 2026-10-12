# Implementation Plan: Fase 2 — Páginas de produto (CMS)

**Branch**: `002-paginas-de-produto` | **Date**: 2026-10-12 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/002-paginas-de-produto/spec.md`

## Summary

O administrador cadastra produtos (com categoria, imagens, descrições, ficha técnica por grupos,
relacionados e links de afiliado vinculados) e publica uma página pública renderizada no servidor.
A página abre no endereço amigável `/p/{slug}` e também nos endereços do código curto do produto
(`/{código}`, `/{canal}/{código}`), sempre com o amigável como canônico.

Reaproveita a base da fase 1: `short_codes` passa a identificar também produtos (o `Resolve`
devolve um alvo tipado); a fila de eventos, o identificador anônimo, a unicidade permanente, a
agregação diária, a retenção e a lixeira ganham o evento `view` e o alvo `product`. Como a página
é cacheada, a visita é registrada por um beacon (`POST /api/v1/beacon/view`, `204`, sem cookie);
o acesso pelo código curto continua registrando um `click`.

Imagens são enviadas pelo painel, recodificadas em WebP com tamanhos responsivos (sem CGO) e
servidas do volume `MEDIA_DIR`, de forma imutável e sem recursos de terceiros. O HTML renderizado
fica num cache em memória com `ETag` e invalidação por versão do catálogo; o navegador sempre
revalida (`no-cache`), então toda abertura é contada e toda edição aparece na abertura seguinte.

O painel ganha produtos, categorias, logotipo de marketplace e estatísticas separadas de visitas
e cliques; a API JSON em `/api/v1` espelha o painel sobre a mesma camada de serviço.

## Technical Context

**Language/Version**: Go 1.27 (o mesmo da fase 1).

**Primary Dependencies**:
- Já existentes: `pgx/v5`, `goose/v3`, `google/uuid`, `x/crypto`, `prometheus/client_golang`, biblioteca
  padrão (`net/http`, `html/template`, `image/*`, `embed`).
- **Novas** (Go puro, sem CGO): `github.com/gen2brain/webp` (codifica/decodifica WebP via
  WebAssembly embutido) e `golang.org/x/image` (redimensionamento). `golang.org/x/text/unicode/norm`
  (remoção de acentos do slug) já é dependência indireta.
- Front-end: HTMX e Alpine já existentes no painel; a página pública usa um único `product.js`
  próprio (< 2 KB, sem dependências).

**Storage**: PostgreSQL 16 (uma migração, `00004_products.sql`) e sistema de arquivos em
`MEDIA_DIR` para as imagens (volume nomeado no compose).

**Testing**: `go test` com `httptest`; integração com PostgreSQL real (`TEST_DATABASE_URL`);
`-race` em eventos, redirecionamento, cache de páginas e beacon; imagens com arquivos gerados nos
próprios testes; carga com `scripts/loadtest.sh` adaptado; verificação em navegador pelo passo 4 do
quickstart.

**Target Platform**: Linux, contêiner `distroless/static:nonroot` (inalterado), instância única.

**Project Type**: serviço web (SSR + API JSON) em um único módulo Go.

**Performance Goals**: página (cache quente) p95 < 100 ms no servidor; LCP < 2,5 s, CLS < 0,1,
INP < 200 ms em celular médio; 200 aberturas/s por 60 s sem erro e com descarte < 0,1 %.

**Constraints**:
- Sem `Set-Cookie` público; sem IP puro persistido ou em log; sem recursos de terceiros.
- Nenhuma escrita em banco no caminho de abertura: cliques e visitas vão para a fila.
- Build estático com `CGO_ENABLED=0`; sem Node/npm.
- Toda abertura deve chegar ao servidor (`no-cache` + `ETag`) para ser contada.

**Scale/Scope**: até 1.000 produtos publicados, uma organização; cerca de 8 telas novas de painel
(produtos ×5, categorias ×2, detalhe com estatísticas); imagens: tamanho estimado de ~100–300 KB por
produto em quatro variantes.

Não há itens NEEDS CLARIFICATION; as escolhas estão em [research.md](research.md).

## Constitution Check

*GATE: aprovado antes da pesquisa e reavaliado após o desenho.*

| Princípio | Verificação no plano | Status |
|---|---|---|
| I. Núcleo primeiro e redirecionamento resiliente | Links continuam em `302` pelo mesmo fluxo; o código de produto não redireciona (a spec manda exibir a página) e **não escreve no banco no caminho do acesso**: só enfileira. Leituras em cache miss são consultas simples. Código curto continua com sete caracteres, único globalmente (mesma tabela). Destinos de link seguem `http`/`https` | PASS |
| II. Privacidade por construção | Beacon e visitas usam o mesmo HMAC (IPv4 completo, IPv6 `/64`); respostas de página, beacon e mídia sem `Set-Cookie`; sem IP em logs; aviso e `/privacidade` atualizados (R15); imagens recodificadas descartam metadados; nenhum recurso de terceiros | PASS |
| III. Eventos assíncronos e contagem correta | `view` entra na mesma fila e no mesmo worker; descarte contabilizado; unicidade por `INSERT … ON CONFLICT` com `event_type` na chave (R8); crawlers e `is_bot` fora das estatísticas; links `direct` seguem "não rastreável" e **sem métrica**; agregação/limpeza idempotentes reaproveitadas | PASS |
| IV. Isolamento por organização e integridade | Todas as tabelas novas têm `org_id` e todo repositório recebe `orgID`; IDs UUID e UTC; migração versionada com `Down`; produtos e categorias com exclusão lógica, sem substituir as regras de purga; a remoção da FK de `short_codes` é compensada por transação e teste (R4) | PASS |
| V. Simplicidade operacional e stack | Mesmo binário e banco; `html/template`, HTMX/Alpine só no painel; pouco JavaScript público; configuração por ambiente documentada; **duas dependências novas justificadas abaixo** | PASS (ver Complexity Tracking) |
| VI. Teste e validação por épico | Cada épico lista seus testes; integração com PostgreSQL real; `-race` onde há cache/fila; regressão dos totais da fase 1; quickstart reproduzível; nada declarado sem execução | PASS |
| VII. Escopo controlado | Fora de escopo respeitado (preços, ranking, veredito, avaliações, comparador, integrações); sitemap/robots e site público ficam na fase 3; a fase 1 foi aceita antes de começar; nenhuma questão em aberto decidida (a dos links `direct` já estava decidida pelo responsável) | PASS |

**Re-check pós-design**: PASS. O único ponto de atenção é a correção das consultas de estatística da
fase 1, que hoje somam eventos sem filtrar tipo (R8); é tratada como parte do épico de eventos, com
testes de regressão.

## Complexity Tracking

| Desvio | Por que é necessário | Alternativa mais simples rejeitada |
|---|---|---|
| `gen2brain/webp` (WebAssembly embutido, ~8 MB a mais no binário) | FR-020 pede formatos modernos e o build precisa continuar estático e sem CGO; prova de conceito: ~18 ms para codificar 640 px, saída ~4× menor que JPEG (sintético) | Só JPEG/PNG da biblioteca padrão: contradiz FR-020 e pesa mais no LCP |
| `golang.org/x/image` | Redimensionamento de boa qualidade (CatmullRom) que a biblioteca padrão não tem | Reduzir por vizinho mais próximo: qualidade ruim |
| Cache de páginas com versão por organização | Atender SC-004/SC-007 (rápido e nunca desatualizado) sem `max-age` | Renderizar a cada acesso: mais consultas por abertura; `max-age`: páginas desatualizadas |
| Volume de mídia no deploy | Escolha do responsável (armazenamento pelo próprio serviço) | Armazenamento externo: descartado |

## Project Structure

### Documentation (this feature)

```text
specs/002-paginas-de-produto/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── public-routes.md
│   ├── api-v1.md
│   ├── admin-ui.md
│   └── config.md
├── checklists/requirements.md
└── tasks.md             # /speckit-tasks
```

### Source Code (repository root)

Estrutura da fase 1 mantida; só o que muda ou nasce:

```text
migrations/
└── 00004_products.sql            categories, products, media, product_media, product_spec_*,
                                  product_related; colunas em affiliate_links e marketplaces;
                                  CHECKs de short_codes/click_*; visitor_seen.event_type
cmd/hublinks/                     (sem novos subcomandos)
internal/
├── domain/                       Product, Category, Media; validações de slug, textos e ficha
├── media/                        NOVO: validação, decodificação, variantes WebP e armazenamento em disco
├── store/                        products.go, categories.go, media.go; Resolve tipado (codes.go)
├── service/                      products.go, categories.go, media.go (invalida caches e versão)
├── events/                       Event.Kind/TargetType; writer e visitor_seen com event_type
├── redirect/                     alvo tipado no Cache e no Handler; Recorder para produto
├── web/                          product.go (render, cache de páginas, ETag), beacon via api/
├── api/                          products.go, categories.go, media.go, beacon.go, stats de produto
├── admin/                        products.go, categories.go, media.go; marketplaces (logotipo); lixeira; dashboard
├── stats/                        filtros de tipo nas consultas existentes; products.go
├── maintenance/                  purga de produtos/categorias, mídia órfã e arquivos
├── config/                       MEDIA_DIR, MEDIA_MAX_BYTES, PAGE_CACHE_MB
└── server/                       novas rotas: /p/{slug}, /media/…, beacon, /admin/products…, /api/v1/…
web/
├── templates/
│   ├── public/product.html       página pública
│   ├── admin/products/…          list, form, show, confirmação; admin/categories/…
│   └── components/               galeria, acordeão de ficha, carrossel, barra fixa, editor de ficha
└── static/js/product.js          beacon, "Mostrar todos", barra fixa
deploy/                           volume hublinks-media, MEDIA_DIR, /data/media com dono nonroot
docs/                             operacao.md (backup de mídia, migração de visitor_seen), pendencias.md,
                                  guia-de-estilo.md (componentes novos)
```

**Structure Decision**: mesmo módulo e mesma árvore da fase 1. Novo pacote só onde há responsabilidade
própria: `internal/media` (pipeline de imagem e disco), usado por `service` e `web`. A página pública
e seu cache ficam em `internal/web`, onde já estão a página de privacidade e os templates públicos;
o beacon fica em `internal/api` por estar sob `/api/v1`.

## Ordem de entrega (épicos)

Cada épico termina com testes passando e o trecho do quickstart correspondente. A numeração de tarefas
sai do `/speckit-tasks`.

1. **Fundação de dados**: migração `00004`, domínio, repositórios, validações; correção dos filtros de
   tipo em `internal/stats` com testes de regressão; `visitor_seen.event_type`.
2. **Mídia**: pacote `internal/media`, envio (`POST /admin/media`, `/api/v1/media`), rota `/media/…`,
   limpeza de órfãs, `MEDIA_DIR`/volume/Dockerfile.
3. **Produtos e categorias no painel e na API** (História 1, 3 e 6): CRUD, slug, publicação, lixeira,
   vínculo de links, ficha técnica, relacionados, logotipo de marketplace.
4. **Página pública** (História 2 e 5): render, cache e `ETag`, `/p/{slug}`, alvo tipado em
   `/{código}`, SEO e JSON-LD, estilos, `product.js`, aviso e privacidade.
5. **Visitas e estatísticas** (História 4): beacon, `Kind`/`TargetType` nos eventos, estatísticas de
   produto no painel e na API, cartão no dashboard, agregação/retenção/purga estendidas.
6. **Endurecimento**: carga com 1.000 produtos, métricas (cache, beacon, mídia), auditoria de
   privacidade estendida, `docs/operacao.md`, guia de estilo, passo a passo do quickstart em navegador
   e Docker.
