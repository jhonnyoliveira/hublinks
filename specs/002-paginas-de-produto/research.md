# Pesquisa (Fase 0): Páginas de produto (CMS)

Contexto fixo (ADRs, constituição e fase 1, não reavaliados aqui): Go, PostgreSQL, um binário,
`html/template` + HTMX, Tailwind standalone, sem cookies públicos, eventos assíncronos em lote,
links `direct` sem métrica. Decisões da spec já tomadas: mídia enviada e servida pelo próprio
serviço; página exibida no endereço curto com `/p/{endereço}` como canônico; catálogo de até
1.000 produtos publicados.

Não há itens NEEDS CLARIFICATION. As provas de conceito citadas foram feitas em módulo
descartável fora do repositório.

## R1. Imagens: validação, redimensionamento e formato de saída

- **Decision**: aceitar JPEG, PNG e WebP. Decodificar com a biblioteca padrão (`image/jpeg`,
  `image/png`) e `github.com/gen2brain/webp` (decodificação), redimensionar com
  `golang.org/x/image/draw` (CatmullRom) e gravar **somente variantes WebP** em larguras fixas
  (320, 640, 960 e 1280 px, limitadas à largura da origem; sempre ao menos uma). O original não é
  guardado: a recodificação descarta metadados (EXIF, GPS) e qualquer carga embutida.
- **Rationale**: o build é estático (`CGO_ENABLED=0`, distroless). Prova de conceito com
  `gen2brain/webp` v0.6.4 (WebAssembly embutido, sem CGO): compilou com `CGO_ENABLED=0`, codificou
  uma imagem de 640×480 em ~18 ms, com 2,9 KB contra 12,2 KB do JPEG equivalente (imagem
  sintética, só indicativa), e decodificou de volta. Atende o orçamento de desempenho do escopo
  (formatos modernos, `srcset`) sem ferramenta externa.
- **Limites**: arquivo ≤ 5 MiB (`MEDIA_MAX_BYTES`), no máximo 24 megapixels (lidos por
  `image.DecodeConfig` antes de decodificar, contra bombas de descompressão) e corpo limitado por
  `http.MaxBytesReader`. Tipo detectado pelo conteúdo (`http.DetectContentType`), nunca pela
  extensão ou pelo `Content-Type` enviado.
- **Alternatives**: AVIF (codificação lenta e pesada, ganho marginal sobre WebP; fica como evolução);
  só JPEG/PNG (contradiz o requisito de formatos modernos, FR-020); CGO com libvips (quebra a imagem
  distroless estática); armazenar o original (mais espaço e risco, sem requisito que o exija).
- **Custo**: binário ~8 MB maior (WebAssembly embutido) e nova dependência indireta de peso; registrado
  em Complexity Tracking.

## R2. Armazenamento e entrega da mídia

- **Decision**: arquivos em `MEDIA_DIR` (padrão `./var/media` em desenvolvimento; `/data/media` no
  compose, em volume nomeado), no caminho `{id[0:2]}/{id}/{largura}.webp`, com metadados na tabela
  `media`. Servidos por `GET /media/{id}/{largura}.webp` com
  `Cache-Control: public, max-age=31536000, immutable`, sem cookies. O `id` é um UUID v4 aleatório e o
  conteúdo nunca muda (substituir uma imagem cria outro `id`).
- **Rationale**: um volume nomeado preserva o "sobe com um comando" e o backup entra no
  `docs/operacao.md`. Conteúdo imutável permite cache agressivo de navegador e de proxy.
- **Permissões**: no `deploy/Dockerfile` a pasta `/data/media` é criada no estágio de build e copiada
  com `--chown=65532:65532` (usuário `nonroot` do distroless); o volume nomeado herda esse dono na
  primeira montagem.
- **Segurança**: o caminho vem de `uuid.Parse` e de uma lista fechada de larguras (nunca de texto
  livre), então não há travessia de diretório. Imagens não publicadas são servidas a quem souber o
  UUID aleatório (122 bits); aceito para evitar sessão em rota de imagem, e registrado como risco
  baixo.
- **Alternatives**: bytea no PostgreSQL (infla o banco e o backup lógico, e dificulta cache);
  armazenamento externo (descartado pelo responsável).

## R3. Órfãos e exclusão de mídia

- **Decision**: o envio cria a mídia sem vínculo; ela passa a pertencer a um produto
  (`product_media`) ou a um marketplace (`logo_media_id`) quando o formulário é salvo. O job diário
  de manutenção apaga mídia sem vínculo com mais de 24 h, e a exclusão definitiva de um produto
  apaga suas mídias exclusivas (linhas e arquivos). A remoção de arquivo falha de forma
  tolerante: se o arquivo já não existe, o job segue.
- **Rationale**: o formulário envia a imagem antes de salvar (HTMX), então há abandono natural;
  sem limpeza o volume só cresceria.
- **Alternatives**: apagar na hora ao desvincular (perde a possibilidade de "desfazer" a edição no
  formulário e complica a restauração da lixeira).

## R4. Resolução de código: produto ou link

- **Decision**: `short_codes` continua sendo a única tabela de códigos (espaço global único,
  FR-004). O `Resolve` do catálogo passa a devolver um alvo tipado
  (`Target{Kind, Link, Product}`), e o `redirect.Cache` guarda o alvo inteiro. O handler decide:
  `link` segue o fluxo de redirecionamento da fase 1; `product` entrega a página (R6).
- **Rationale**: preserva a unicidade global e o cache de 5 min / 30 s negativo; um código de
  produto não publicado, na lixeira ou excluído resolve como `ErrNotFound` (FR-003).
- **Integridade**: `short_codes.target_id` tem hoje FK para `affiliate_links`; a migração a remove
  (a tabela passa a ser polimórfica) e a coerência é garantida por transação no serviço e por teste
  de integração.
- **Invalidação**: ao salvar, publicar, despublicar, excluir ou restaurar um produto, o serviço
  invalida a entrada do código no `Cache` (como já faz com links) e incrementa a versão do cache de
  páginas (R5).

## R5. Cache de páginas renderizadas

- **Decision**: cache em memória de HTML renderizado, chave `(produto, segmento do canal)`, com
  orçamento de bytes (`PAGE_CACHE_MB`, padrão 64) e remoção do menos usado recentemente. Cada entrada
  guarda o HTML, o `ETag` (hash do conteúdo) e a **versão do catálogo** com que foi gerada. Qualquer
  escrita que afete páginas (produto, categoria, link, marketplace, logotipo, mídia, relacionados)
  incrementa uma versão atômica por organização; a leitura descarta entradas de versão antiga e
  renderiza de novo. Respostas levam `Cache-Control: no-cache` e `ETag`: o navegador sempre
  revalida, recebe `304` quando nada mudou e a próxima abertura já vê a edição (SC-007).
- **Rationale**: o conteúdo varia por canal só nos hrefs dos botões e no payload do beacon, então
  uma variante por canal é pequena; o orçamento evita crescer com `produtos × canais` (1.000 ×
  canais × ~40 KB). Versão global por org é simples e correta; com 1.000 produtos, rerenderizar
  sob demanda custa poucas consultas por página.
- **`no-cache` e a contagem**: sem `max-age`, todo acesso chega ao servidor, onde o clique do
  código curto é registrado; o `304` evita reenviar o corpo.
- **Alternatives**: `max-age` curto (mostraria edições com atraso, contra FR-021/SC-007); cache de
  HTML por produto sem canal e substituição por JS dos links (os botões deixariam de funcionar sem
  JavaScript, contra a spec); invalidação fina por dependência (complexidade sem ganho para 1.000
  produtos).

## R6. Roteamento das páginas públicas

- **Decision**: `GET /p/{slug}` é uma rota própria do mux (precede `GET /`, e `p` já é palavra
  reservada de canal). Os endereços `/{code}` e `/{canal}/{code}` continuam no manipulador
  `GET /`, que passa a bifurcar por tipo de alvo. A página é a mesma nos três endereços; o
  canônico é sempre `BASE_URL/p/{slug}`. `HEAD` responde como `GET` sem corpo e sem evento.
- **Rationale**: reaproveita limite de taxa, `Cache`, detecção de crawler e erro 404 da fase 1.
- **Clique do código curto**: registrado como `click` com `target_type=product`, enfileirado pelo
  mesmo `Recorder` (extendido), e ignorado para crawlers (FR-031) e `HEAD`.
- **Slug**: padrão `^[a-z0-9]+(-[a-z0-9]+)+$`, até 80 caracteres, único por organização entre linhas
  não excluídas definitivamente (índice parcial, como o segmento de canal). A sugestão remove
  acentos (`golang.org/x/text/unicode/norm`, já presente como dependência indireta) e, se o
  resultado não tiver hífen, acrescenta `-` e os 4 primeiros caracteres do ID do produto.

## R7. Beacon de visita

- **Decision**: `POST /api/v1/beacon/view`, público, corpo `text/plain` com JSON
  `{"k":"<código do produto>","c":"<segmento do canal ou vazio>"}` (um `Blob` `text/plain` evita
  pré-voo CORS no `navigator.sendBeacon`). Resposta `204`, sem corpo e sem cookie, para qualquer
  entrada (inclusive inválida ou inexistente, sem revelar nada). Limite de corpo de 512 bytes.
  Resolve o código pelo mesmo `Cache`, o canal pelo cache de canais, ignora crawlers e enfileira um
  evento `view` no mesmo `Queue`. O detector de automação por código distinto também se aplica
  (marca `is_bot`).
- **Rationale**: a página está em cache e não passa pela lógica de contagem; o escopo (seção 9.3) e
  a spec (FR-023) exigem registro pelo navegador. O código vem embutido na página, então o beacon
  funciona igual nos três endereços.
- **Proteção**: rota explícita registrada antes do coringa `GET /api/v1/` (que exige sessão);
  `http.CrossOriginProtection` já protege `POST`; limitador de taxa próprio (mesmo valor
  `PUBLIC_RATE_LIMIT`).
- **Alternatives**: `fetch` com `keepalive` (mais código, sem ganho); pixel de imagem (GET com efeito
  colateral, cacheável por proxies).

## R8. Eventos, unicidade e tabelas existentes

- **Decision**: `events.Event` ganha `Kind` (`click`/`view`) e `TargetType`
  (`affiliate_link`/`product`). As CHECKs de `click_events`, `click_daily` e `short_codes` são
  relaxadas na migração. `visitor_seen` ganha `event_type` (padrão `click`) e entra na chave
  primária, para que a unicidade de visitas e a de cliques do mesmo produto sejam independentes
  (a spec exige exibi-las separadas). `is_unique_url` e `is_unique_target` mantêm o significado.
- **Rationale**: o worker já calcula unicidade com `INSERT … ON CONFLICT DO NOTHING`; basta incluir
  o tipo no conflito. A agregação e a limpeza da fase 1 já agrupam por `event_type`/`target_type`.
- **Atenção, correção necessária**: as consultas de `internal/stats` somam `click_events`/`click_daily`
  **sem filtrar tipo**. Com eventos de produto no mesmo banco, os totais de links passariam a
  incluir cliques e visitas de produto. A fase 2 filtra explicitamente
  `event_type='click' AND target_type='affiliate_link'` nas consultas existentes e cria consultas
  próprias para produto (testes de regressão cobrem os totais da fase 1).
- **Alternatives**: valores novos em `scope` para visitas (evita reconstruir a chave primária, mas
  mistura dois conceitos num campo); tabelas separadas para visitas (duplicaria agregação,
  retenção e limpeza).
- **Custo da migração**: reconstruir o índice da chave primária de `visitor_seen`; no volume
  esperado (≤ milhões de linhas) leva segundos. Documentado em `docs/operacao.md`.

## R9. Estatísticas de produto no painel

- **Decision**: `internal/stats` ganha `ProductSummary`, `ProductsList` e `ProductDetail`
  (visitas e cliques, totais e únicos, por canal e por dia), reaproveitando o CTE diário
  (`click_daily` para dias fechados, `click_events` para o dia corrente) com parâmetro de tipo.
  No painel inicial entra um cartão "Visitas às páginas" (total e únicos); "Cliques" continua
  significando acessos a links de afiliado.
- **Rationale**: manter o significado do número da fase 1 evita somar acessos distintos de uma mesma
  jornada (abrir a página e depois clicar em comprar).
- **Links `direct`**: nenhuma consulta de produto agrega por link; o detalhe do produto lista os
  links com a marca "não rastreável" e sem contadores (decisão do responsável).

## R10. Edição da ficha técnica e dos relacionados

- **Decision**: formulário único do produto; grupos e itens editados com Alpine (adicionar, remover,
  mover para cima/baixo com botões, acessíveis por teclado, sem arrastar-e-soltar) e enviados como
  campos indexados. O servidor substitui grupos e itens do produto numa transação, preservando a
  ordem recebida. Relacionados escolhidos por busca HTMX (produtos publicados, exceto o próprio),
  também reordenáveis por botões.
- **Limites de proteção** (a spec diz "sem limite fixo" para o administrador; estes tetos são
  técnicos e altos): 50 grupos por produto, 100 itens por grupo, nome ≤ 120 e valor ≤ 1.000
  caracteres, 20 relacionados. Mensagem clara ao ultrapassar.
- **Alternatives**: edição item a item por HTMX (muitas viagens ao servidor e estado parcial);
  arrastar-e-soltar (pior acessibilidade).

## R11. Texto e HTML

- **Decision**: descrição longa e valores da ficha são texto simples, até 20.000 caracteres a
  descrição. A página converte quebras de linha duplas em parágrafos e simples em `<br>` por uma
  função de template que escapa tudo (`html/template`). Nenhum HTML do administrador é emitido cru.
- **Rationale**: atende "texto simples" da spec e elimina XSS por conteúdo.

## R12. Página pública: estrutura, SEO e JavaScript

- **Decision**: layout `public` existente + `public/product.html`. Visão geral com imagem principal
  (`<img srcset sizes width height fetchpriority=high>`), "Onde comprar" em cartões por marketplace,
  ficha em `<details>` (um por grupo, âncora = `id` do grupo, abre sozinho se o `#hash` apontar para
  ele), carrossel de relacionados em CSS com `scroll-snap` (sem JavaScript, `loading=lazy`), barra
  fixa em CSS com um `IntersectionObserver` de poucas linhas. O único script, `static/js/product.js`
  (< 2 KB, sem dependências), faz: beacon, "Mostrar todos" e a barra fixa. Sem JavaScript, os botões
  e o acordeão nativo funcionam; só a visita deixa de ser registrada (limitação documentada).
- **SEO**: `<title>`, `meta description`, Open Graph (`og:type=product`), `twitter:card`,
  `link rel=canonical`, `h1` único e JSON-LD `schema.org/Product` (`name`, `description`, `image`,
  `category`) **sem** `offers`, `aggregateRating` ou preço. `robots` indexável para publicados.
- **Desempenho**: CSS é o `app.css` já compilado e com hash (≈ 19 KB); imagens abaixo da dobra com
  `loading=lazy`; dimensões declaradas contra mudança de layout.
- **Alternatives**: bibliotecas de carrossel/acordeão (peso e JavaScript sem necessidade).

## R13. Limite de taxa e rotas de mídia

- **Decision**: `GET /p/{slug}` usa a mesma instância do limitador de `GET /`; o beacon usa uma
  instância própria com o mesmo valor; `GET /media/…` não tem limite (como `/static/`).
- **Rationale**: uma página tem ~10–30 imagens; contá-las no limite de 300/min por IP bloquearia
  usuários legítimos após poucas páginas. Imagens são estáticas, imutáveis e baratas de servir.
- **Risco aceito**: leitura de disco por requisições repetidas; mitigado pelo cache longo e
  porque o volume é de arquivos pequenos.

## R14. Configuração nova

- **Decision**: `MEDIA_DIR` (padrão `./var/media`; `/data/media` no compose), `MEDIA_MAX_BYTES`
  (padrão `5242880`), `PAGE_CACHE_MB` (padrão `64`). Todas opcionais, documentadas no contrato de
  configuração e no `.env.example`. A aplicação cria `MEDIA_DIR` se não existir e **aborta na
  partida** se não puder gravar nele, citando a variável.
- **Rationale**: erro cedo evita descobrir o problema no primeiro envio.

## R15. Privacidade

- **Decision**: a página `/privacidade` passa a descrever que a abertura de páginas de produto é
  contada com o mesmo identificador anônimo (HMAC de IP e user-agent, sem cookies), que visitas têm
  a mesma retenção dos cliques e que navegadores sem JavaScript não são contados. A página de
  produto exibe o aviso informativo existente (`privacy_notice`).

## R16. Trabalho fora do código de produto

- **Decision**: a manutenção estende a expiração da lixeira a produtos e categorias (marca
  `purged_at`, apaga `visitor_seen` com `target_type='product'`, desvincula links, apaga mídias
  exclusivas e invalida caches) e acrescenta a limpeza de mídia órfã. `docs/operacao.md` ganha
  backup do volume de mídia, a migração de `visitor_seen` e as novas variáveis. A `docs/pendencias.md`
  registra as assunções desta fase.

## R17. Organização nas rotas públicas

- **Decision**: `/p/{slug}` e o beacon resolvem a organização pela **única organização da instalação**
  (`SELECT id FROM organizations ORDER BY created_at LIMIT 1`, lida uma vez e guardada em memória).
  Os códigos curtos continuam resolvendo a organização pelo próprio `short_codes`.
- **Rationale**: o slug é único por organização (não globalmente) e esta fase tem uma só organização,
  criada na partida. Evita inventar roteamento por domínio, que pertence à fase 5.
- **Consequência**: com mais de uma organização (fases 4 e 5), `/p/{slug}` precisará do domínio ou de um
  prefixo para identificar a organização; registrado como assunção em `docs/pendencias.md`.
