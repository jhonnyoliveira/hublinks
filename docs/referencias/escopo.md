# Hub Links — Escopo

## 1. Visão

Plataforma de gerenciamento de links para programas de afiliados. Une três capacidades:

1. **Encurtador de URLs com analytics** — núcleo do produto, construído primeiro.
2. **CMS de páginas de produto** — hub de links de afiliados por produto, sem preço nem estoque.
3. **Site público por organização** — navegação por categorias e busca, renderizado no servidor e otimizado para SEO.

**Pergunta que o sistema responde:** "minha divulgação está fazendo sentido?" Ele mede alcance e intenção (visitas e cliques únicos) por canal e por destino. **A conversão (vendas, comissão) fica nas plataformas dos marketplaces e está fora de escopo**, sem integração com suas APIs.

**Fora de escopo permanente (decidido):** integração com APIs de marketplaces e medição de conversão, comparação de preços e alertas de preço, pontuação/ranking/radar de produtos, avaliações de usuários e frase de "veredito". Não implementar nem reintroduzir sem nova decisão do responsável.

**Uso inicial:** próprio, de um único usuário. A evolução para SaaS multi-tenant é futura, e por isso o modelo de dados já nasce preparado para ela (ver ADR-005).

## 2. Princípios de projeto

- **Núcleo primeiro.** Redirecionamento e contagem confiáveis vêm antes de qualquer interface rica.
- **Redirecionamento rápido.** O caminho de clique é o mais crítico do sistema; nenhuma escrita em banco deve bloqueá-lo.
- **Guardar eventos brutos pelo período de retenção** (seção 8.1). Regras de contagem podem mudar; os eventos permitem recalcular enquanto existirem.
- **Preparado para multi-tenant, sem custo agora.** Toda tabela de domínio tem `org_id`.
- **Degradar com graça.** Se o rastreamento falhar, o usuário ainda deve chegar ao destino.
- **Simplicidade operacional.** Um binário/serviço, um banco, deploy por imagem Docker.

## 3. Glossário

| Termo | Definição |
|---|---|
| **Organização (org)** | Espaço de trabalho dono de produtos, links e canais. Na fase 1 existe uma só. |
| **Marketplace** | Plataforma de destino (Amazon, Shopee, Mercado Livre...). |
| **Link de afiliado** | URL de afiliado de um marketplace, cadastrada pelo usuário. Pode haver vários por marketplace e por produto. |
| **Produto** | Página-hub com informações descritivas e uma lista de links de afiliado. |
| **Código curto (`code`)** | Identificador curto e único que resolve para um produto ou link de afiliado. |
| **Canal** | Origem de divulgação (WhatsApp, Telegram, YouTube, Instagram...), representado por um segmento de URL. Configurável por org. |
| **Visitante** | Identificador anônimo derivado de IP e user-agent, sem cookie (ver seção 8). Aproxima um dispositivo/rede; não é uma pessoa identificada. |
| **Clique único** | Primeiro evento de um visitante para determinado alvo, de forma permanente (ver seção 8). |

## 4. Escopo por fase

### Fase 1 — Encurtador com analytics (MVP)

**Entra:**
- Autenticação de usuário único (administrador).
- CRUD de marketplaces, com política de encurtamento.
- CRUD de canais.
- CRUD de links de afiliado, com título, imagem opcional e URL original.
- Geração de código curto e URLs por canal.
- Redirecionamento com registro de evento.
- Página de pré-visualização para crawlers (Open Graph).
- Contagem de cliques únicos e totais, filtro de bots.
- Painel web administrativo (login, CRUD de marketplaces, canais e links, e dashboards de métricas por link, canal e dia).
- Aviso de privacidade e página pública `/privacidade` (ver seção 8).
- Deploy via Docker.

**Fora:** páginas de produto completas, site público, categorias, busca, múltiplos usuários, domínio customizado, qualquer integração com APIs de marketplaces.

### Fase 2 — Páginas de produto (CMS)
Produtos com categorias, mídia, descrição curta e longa, **grupos de especificações por produto exibidos como acordeão** (seção 11.1), metatags e produtos relacionados. Página pública do produto renderizada no servidor, com cache e beacon de visita, seção "Onde comprar" com os links de afiliado, barra fixa de ação e carrossel de relacionados.

### Fase 3 — Site público da organização
Home, navegação por categorias, busca, produtos relacionados, sitemap, dados estruturados, otimização de performance.

### Fase 4 — Multiusuário
Convites, papéis **Administrador** e **Mantenedor** (ver seção 7), múltiplas organizações por usuário.

### Fase 5 — SaaS
Domínio customizado por org (DNS, TLS automático), planos e limites, cobrança. **Não detalhado nesta versão.**

## 5. Decisões de arquitetura (ADRs resumidos)

### ADR-001 — Linguagem: Go
Alternativa descartada: Python + FastAPI. Go foi escolhido por: desempenho no caminho de redirecionamento, binário único, imagem Docker pequena, `html/template` suficiente para SSR e concorrência simples para a gravação assíncrona de eventos.

### ADR-002 — Banco: PostgreSQL
Eventos de clique crescem rápido e as consultas são agregações; PostgreSQL atende bem e já suporta o futuro multi-tenant. SQLite foi descartado por dificultar a evolução para SaaS. Migrações versionadas desde o início.

### ADR-003 — Redirecionamento: HTTP 302
Usa-se **302**, nunca 301, para impedir que navegadores cacheiem o redirecionamento e deixem de gerar eventos.

### ADR-004 — Gravação de eventos assíncrona
O handler de redirecionamento resolve o destino, responde, e envia o evento a um canal/fila em memória consumida por um worker que grava em lote. Falha na gravação nunca bloqueia nem quebra o redirecionamento. Se o buffer encher, descarta-se o evento e incrementa-se uma métrica de erro (aceita-se perda pequena).

### ADR-005 — Multi-tenant desde a modelagem
Toda tabela de domínio tem `org_id`. Todas as consultas filtram por `org_id`. Na fase 1 existe uma org padrão criada na inicialização. Evita reescrita na fase 5.

### ADR-006 — Renderização no servidor com cache
Páginas públicas (fase 2+) são HTML renderizado no servidor, com cache em memória e cabeçalhos HTTP (`Cache-Control`, `ETag`). Invalidação ao editar o produto. Como a página em cache não passa pela lógica de contagem, **visitas são registradas por beacon** (seção 9).

### ADR-007 — URL original para marketplaces restritivos
Cada marketplace tem uma política `shorten_policy`: `shorten` (exibe o link curto, rastreável) ou `direct` (exibe a URL original, sem rastreamento de clique). A política pode ser sobrescrita por link de afiliado. No painel, links `direct` aparecem como **"não rastreável"**, e nunca como zero cliques.

### ADR-008 — Painel web renderizado no servidor com HTMX 
O painel administrativo existe desde a fase 1, implementado com `html/template` + **HTMX**, servido pelo mesmo binário Go, sem SPA. Motivos: um único deploy, reaproveitamento dos templates no SSR público (fase 2) e facilidade para agentes de IA implementarem. Estilo, componentes e interatividade seguem as diretrizes da seção 11.1. A API JSON `/api/v1` continua existindo, e o painel a consome ou compartilha a mesma camada de serviço. Autenticação do painel por cookie de sessão (`HttpOnly`, `Secure`, `SameSite=Lax`) com proteção CSRF.

### ADR-009 — Identificação do visitante sem cookies
Alternativas consideradas: cookie sempre; cookie só com consentimento; página intermediária de consentimento; fingerprint reforçado. Escolhido: **somente HMAC de IP + user-agent, sem cookie algum de rastreamento**. Motivo: o sistema serve a análises comparativas entre canais, não a precisão científica; perder ou duplicar alguns cliques é aceitável diante da simplicidade e da menor exposição legal de não gravar nada no dispositivo do visitante. Consequências: contagem de únicos aproximada (seção 8), nenhuma lógica de consentimento, banner meramente informativo. Se no futuro a precisão se tornar crítica, reavaliar a adoção de um cookie opcional.

## 6. Modelo de dados

Tipos indicativos; o agente deve adaptar à convenção do projeto. Todos os ids em UUID, timestamps em UTC.

```
organizations(id, name, slug, created_at)

users(id, email, password_hash, created_at)
memberships(user_id, org_id, role)            -- role: admin | maintainer

marketplaces(id, org_id, name, shorten_policy)   -- shorten | direct

channels(id, org_id, name, segment)              -- segment: wapp, tgrm, ytb, insta... (único por org)

short_codes(code PK, org_id, target_type, target_id, created_at)
                                                 -- target_type: affiliate_link | product

affiliate_links(id, org_id, marketplace_id, product_id NULL,
                title, image_url NULL, destination_url,
                shorten_policy_override NULL, active, created_at)

products(id, org_id, slug UNIQUE per org, title, ...)   -- esqueleto na fase 1; detalhado na fase 2

click_events(id, occurred_at, org_id, event_type, code, target_type, target_id,
             channel_id NULL, visitor_id,       -- event_type: click | view (view: beacon, fase 2+)
             is_unique_url, is_unique_target, is_bot,
             referer, user_agent)

visitor_seen(target_type, target_id, channel_id NOT NULL, visitor_id, first_seen_at)
             -- PK (target_type, target_id, channel_id, visitor_id); base da unicidade
             -- "sem canal" usa o UUID nulo (00000000-0000-0000-0000-000000000000), pois colunas de PK não aceitam NULL
             -- guarda também uma linha com channel_id = UUID nulo para a unicidade por alvo (is_unique_target)

click_daily(org_id, day, event_type, target_type, target_id, channel_id NOT NULL,
            clicks, unique_url, unique_target)
            -- PK (org_id, day, event_type, target_type, target_id, channel_id); agregado diário, sem bots (ver seção 8.1)
            -- channel_id nulo em click_events vira o UUID nulo aqui
```

**Regras de integridade**
- `short_codes.code` é único globalmente (resolvido antes do `org_id` na URL).
- `channels.segment` é único por org e não pode colidir com palavras reservadas (`api`, `admin`, `p`, `static`, `healthz`, `beacon`, `privacidade`).
- `affiliate_links.destination_url` aceita apenas `http` e `https`.
- Para `target_type = product` (fase 2), o acesso ao link curto é registrado como `click` (alcance do link divulgado) e a abertura da página, inclusive pela URL amigável, como `view` (beacon). Os dois são exibidos separadamente nas estatísticas.
- Para todas as entidades principais usaremos `soft delete`.

## 7. Papéis (fase 4; fase 1 usa só Administrador)

| Papel             | Pode                                                                                                                           |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| **Administrador** | Tudo do mantenedor + gerenciar usuários e convites + configurações avançadas (marketplaces, canais, política de encurtamento). |
| **Mantenedor**    | Gerenciar produtos e links de afiliado; ver estatísticas.                                                                      |

Sem permissões granulares: dois papéis fixos.

## 8. Contagem de acessos únicos

**Definição (decidida):** clique único é **permanente**: o visitante conta uma vez por alvo, sem janela de expiração.

**Identificação do visitante sem cookie de rastreamento**

O sistema **não emite nenhum cookie de rastreamento ou de analytics**. O visitante é identificado apenas por:

`visitor_id = HMAC-SHA256(PEPPER, IP_normalizado + "|" + user_agent)`

- `PEPPER` é um segredo fixo da aplicação. **Não** rotacionar, pois isso quebraria a unicidade permanente.
- **Normalização do IP:** IPv4 completo; IPv6 reduzido ao prefixo `/64`. Reduz duplicatas causadas por privacidade de endereço IPv6.
- O IP real do cliente é obtido do cabeçalho `X-Forwarded-For` **somente** quando a requisição vem de um proxy listado em `TRUSTED_PROXIES`; caso contrário usa-se o endereço da conexão.
- O mesmo cálculo é usado no redirecionamento e no beacon, de modo que ambos produzem o mesmo `visitor_id`.

Os cookies existentes se limitam ao **cookie de sessão do painel administrativo** (estritamente necessário, nunca enviado a visitantes). O `ADR-009` registra a decisão.

**Dois níveis de unicidade, ambos gravados no evento**
- `is_unique_url`: primeiro evento do visitante naquele alvo **e** naquele canal (corresponde à tabela de estatísticas por URL/canal).
- `is_unique_target`: primeiro evento do visitante naquele alvo, em qualquer canal (alcance real, sem somar a mesma pessoa várias vezes).

Determinados por `INSERT ... ON CONFLICT DO NOTHING` em `visitor_seen`.

**Filtro de bots**
- Lista de user-agents de crawlers conhecidos (WhatsApp, Telegram, facebookexternalhit, Twitterbot, Slackbot, Googlebot etc.): recebem a página de preview (seção 9.2) e **não geram evento**.
- Heurística simples: mesmo IP acessando muitos códigos distintos em poucos segundos → o evento é gravado com `is_bot = true`, **não conta** nas métricas e é mantido só por 30 dias (seção 8.1), para depurar o filtro.

**Limitações assumidas (documentar no painel)**
- IP que muda (rede móvel, troca entre Wi-Fi e 4G) faz o mesmo visitante ser contado como novo, **inflando os únicos**.
- IP compartilhado (CGNAT móvel, redes corporativas) com o mesmo user-agent pode fundir visitantes distintos, **reduzindo os únicos**.
- O objetivo é uma **estimativa consistente para comparar canais**, não precisão científica. Esse trade-off foi aceito conscientemente em troca de não usar cookies.

**Privacidade (LGPD)**
- Nunca gravar IP puro, apenas o hash.
- **Aviso de privacidade desde a fase 1.** Como não há cookie de rastreamento, o aviso é **informativo**, não um pedido de consentimento. Componentes:
  - Página pública `GET /privacidade` descrevendo que a contagem de acessos usa um identificador anônimo derivado de IP e user-agent (hash irreversível, sem cookies), a finalidade (estatísticas de divulgação), os prazos de retenção (seção 8.1) e como solicitar a exclusão.
  - Aviso curto em toda página HTML servida ao visitante (página de preview na fase 1; páginas de produto e site público a partir da fase 2), com link para `/privacidade`. O descarte do aviso é lembrado em `localStorage`, sem cookie.
- O redirecionamento 302 não exibe interface; a transparência nele é garantida pela `/privacidade`.
- Recomenda-se revisão jurídica da base legal (legítimo interesse) antes de abrir o serviço a terceiros (fase 5).

### 8.1 Retenção de dados

Retenção em camadas:

| Dado | Retenção | Motivo |
|---|---|---|
| `click_events` (usuários reais) | **13 meses** (`EVENTS_RETENTION_DAYS=395`) | Cobre um ciclo anual para comparar sazonalidade |
| `click_events` com `is_bot = true` | **30 dias** (`BOT_EVENTS_RETENTION_DAYS=30`) | Servem apenas para depurar o filtro de bots |
| `click_daily` | **Para sempre** | Agregado pequeno que preserva todo o histórico analítico |
| `visitor_seen` | **Enquanto o alvo existir** (inclusive na lixeira) | Base da unicidade permanente; apagar faria visitantes antigos serem contados como novos |
| Itens na lixeira (links, canais, marketplaces) | **30 dias** (`TRASH_RETENTION_DAYS=30`) | Permite restaurar exclusões acidentais; depois a exclusão é definitiva |

Regras:
- As flags `is_unique_url` e `is_unique_target` são calculadas na inserção do evento. Somá-las por dia em `click_daily` dá contagens exatas de únicos, mesmo após a remoção do evento bruto.
- **Job de agregação** (diário): agrega o dia anterior em `click_daily`, ignorando eventos de bot. Deve ser idempotente (reexecutar o mesmo dia não duplica valores).
- **Job de limpeza** (diário): remove eventos além da retenção **somente se o dia correspondente já estiver agregado**.
- **Lixeira:** excluir um link, canal ou marketplace o envia à lixeira (exclusão lógica), de onde pode ser restaurado por 30 dias. Durante esse prazo, códigos e URLs do item respondem 404 e as linhas de `visitor_seen` são mantidas, para que a restauração não faça visitantes antigos contarem como novos.
- **Exclusão definitiva:** ao fim do prazo da lixeira, o job de limpeza marca o item como excluído definitivamente e apaga as linhas correspondentes de `visitor_seen` (links e, a partir da fase 2, produtos). Códigos curtos nunca são reutilizados e as contagens de `click_daily` permanecem nos totais.
- As consultas do painel leem de `click_daily` para períodos fechados e de `click_events` para o dia corrente.
- Prazos configuráveis por variável de ambiente; devem ser descritos na página `/privacidade`.
- Perde-se com a limpeza apenas o detalhe fino (`referer`, `user_agent`, `visitor_id` por evento), o que também reduz o risco de privacidade.

## 9. Fluxos e rotas

### 9.1 Rotas públicas (fase 1)

| Rota | Comportamento |
|---|---|
| `GET /{code}` | Resolve `short_codes`. Registra evento (canal nulo). Redireciona 302 ao destino. |
| `GET /{canal}/{code}` | Idem, com `channel_id` pelo `segment`. Canal inexistente → 404. |
| `GET /healthz` | Verificação de saúde. |
| `GET /privacidade` | Política de privacidade (seção 8). |
| `GET /admin/...` | Painel administrativo (seção 9.4). |

`code`: exatamente 7 caracteres `[0-9a-z]`, sem hífen. Slugs amigáveis (fase 2) obrigatoriamente contêm hífen, evitando colisão com códigos.

### 9.2 Fluxo de redirecionamento

```
Requisição → resolver code (cache em memória, fallback banco)
  ├─ não existe/inativo → 404
  ├─ User-Agent é crawler (preview) → 200 com HTML mínimo + metatags Open Graph (sem redirecionar, sem contar)
  └─ usuário real → calcular visitor_id (HMAC de IP normalizado + user-agent; sem cookie)
        → montar evento (visitor_id, canal, referer, ua)
        → enfileirar evento (assíncrono; o worker calcula as flags de unicidade ao gravar)
        → 302 para destination_url
```

**Importante:** a página de preview para crawlers é o que faz a prévia aparecer corretamente no WhatsApp/Telegram. Metatags: `og:title`, `og:image`, `og:description`, `og:url`, `og:type`, `twitter:card`.

### 9.3 Beacon de visita (fase 2+)

Páginas públicas em cache incluem um script mínimo que, ao carregar, dispara `navigator.sendBeacon('/api/v1/beacon/view', {code, canal?})`. O servidor calcula o `visitor_id` da mesma forma que no redirecionamento (sem cookie) e grava o evento do tipo `view`. Resposta `204`. Falha do beacon é silenciosa. Beacon não substitui o evento de clique em links `shorten`, que continuam passando pelo redirecionamento.

### 9.4 Painel web (fase 1)

Rotas sob `/admin`, todas protegidas por sessão, exceto o login.

| Rota | Tela |
|---|---|
| `GET/POST /admin/login`, `POST /admin/logout` | Autenticação |
| `GET /admin` | Dashboard: totais e únicos no período, top links, comparação entre canais, série diária |
| `GET /admin/links` | Lista de links de afiliado com cliques totais/únicos; busca e filtro por marketplace |
| `GET /admin/links/new`, `GET /admin/links/{id}/edit` | Formulário de link (título, imagem, URL, marketplace, política) |
| `GET /admin/links/{id}` | Detalhe: URLs geradas com botão de copiar (curta e por canal), estatísticas por canal e série diária |
| `GET /admin/marketplaces` | CRUD de marketplaces e política de encurtamento |
| `GET /admin/channels` | CRUD de canais |

Requisitos de usabilidade:
- Cada link exibe as URLs de todos os canais prontas para copiar com um clique.
- Links `direct` aparecem com a marca **"não rastreável"**, nunca como zero.
- Seletor de período (7, 30, 90 dias, personalizado) em todas as telas de métricas.
- Utilizável em tela de celular, já que o cadastro de links costuma acontecer fora do computador.

### 9.5 API administrativa (fase 1)

Autenticada pela mesma sessão do painel, prefixo `/api/v1`, JSON.

- `POST/GET/PATCH/DELETE /marketplaces`
- `POST/GET/PATCH/DELETE /channels`
- `POST/GET/PATCH/DELETE /affiliate-links` (cria `short_codes` automaticamente)
- `GET /stats/links` — cliques totais/únicos por link, com filtro de período
- `GET /stats/links/{id}` — detalhe por canal e série diária
- `GET /stats/channels` — comparação entre canais

## 10. Estatísticas (saída esperada)

Por link, com totais e únicos lado a lado:

| URL | Canal | Cliques | Únicos |
|---|---|---|---|
| https://hub.xpto.com/3p4mj5 | curta | 98 | 71 |
| https://hub.xpto.com/wapp/3p4mj5 | whatsapp | 10 | 9 |
| https://hub.xpto.com/ytb/3p4mj5 | youtube | 77 | 60 |

Por destino (link de afiliado): marketplace, URL, cliques, únicos, e marcação **"não rastreável"** quando a política for `direct`.

Métricas adicionais: série diária, comparação entre canais, alcance único por alvo.

## 11. Requisitos não funcionais

| Requisito | Meta (fase 1) |
|---|---|
| Latência do redirecionamento | p95 < 50 ms no servidor (sem contar rede) |
| Disponibilidade do redirecionamento | Não depende da gravação do evento |
| Perda de eventos | Tolerância baixa; medir e expor métrica |
| Segurança | Senhas com argon2/bcrypt; validação de esquema da URL de destino; proteção contra open redirect fora do cadastro; rate limit nas rotas públicas e limitação de tentativas no login |
| Observabilidade | Logs estruturados, métricas (requisições, eventos descartados, latência), `/healthz` |
| Deploy | Imagem Docker multi-stage + `docker-compose` (app + PostgreSQL) |
| Configuração | Variáveis de ambiente (12-factor), incluindo `PEPPER`, `DATABASE_URL`, `BASE_URL`, `TRUSTED_PROXIES`, `ADMIN_EMAIL`, `ADMIN_PASSWORD` (cria o administrador inicial na primeira execução, se não houver usuários), `EVENTS_RETENTION_DAYS`, `BOT_EVENTS_RETENTION_DAYS` |
| SEO (fase 2+) | SSR, metatags, `sitemap.xml`, `robots.txt`, `schema.org/Product` sem `offers`, URLs canônicas |

### 11.1 Diretrizes de UI/UX (painel e páginas públicas)

A interface deve ser **moderna, limpa e agradável**, tanto no painel quanto para visitantes. Um agente de implementação deve seguir estas diretrizes e criar um design system pequeno, reutilizado em todas as telas.

**Tecnologia de front-end**
- **Tailwind CSS** compilado pelo **binário standalone** (sem Node nem npm), executado em um estágio do `Dockerfile`. O CSS final é minificado e versionado por hash no nome do arquivo.
- **HTMX** para interações com o servidor (formulários, filtros, paginação, troca parcial de seções).
- **Alpine.js** apenas para interações locais pequenas (menus, modais, abas, botão "copiar").
- Ícones em **SVG inline** (por exemplo, o conjunto Lucide), sem fonte de ícones.
- Gráficos com uma biblioteca JS leve (Chart.js ou uPlot), carregada **somente** nas telas que a usam.
- Fontes **hospedadas localmente** (sem Google Fonts), por privacidade e desempenho; uma família sans-serif moderna, com `font-display: swap`.
- Nenhum recurso de terceiros em páginas públicas.

**Design system**
- Tokens de design (cores, espaçamento, raios, sombras, tipografia) definidos em um só lugar e reutilizados.
- Componentes padronizados: botão, campo de formulário, tabela, cartão, selo de status, modal, aviso (toast), abas, seletor de período, estado vazio e esqueleto de carregamento.
- Modo claro e escuro, respeitando `prefers-color-scheme`, com alternância manual lembrada em `localStorage`.
- Contraste mínimo WCAG AA, foco visível, navegação por teclado, rótulos acessíveis e `alt` nas imagens.
- Idioma da interface: português do Brasil.

**Painel administrativo: CRUD clássico**
- Padrão de todas as entidades (marketplaces, canais, links e, depois, produtos, categorias e usuários):
  1. **Listagem** em tabela, com busca, filtros, ordenação e paginação via HTMX, sem recarregar a página, e botão "Novo" no topo.
  2. **Formulário** de criação e edição em página própria (ou modal para entidades simples), com validação inline.
  3. **Detalhe** quando houver estatísticas (links e produtos).
  4. **Exclusão** com confirmação.
- Navegação lateral fixa em telas grandes e menu recolhível em celular (mobile first; o cadastro de links costuma ocorrer no celular). No celular, as tabelas se adaptam (colunas secundárias ocultas ou linhas em formato de cartão).
- Dashboard com cartões de resumo (cliques, únicos, links ativos), gráfico de série diária e rankings (top links, comparação entre canais).
- Formulários com feedback imediato, mensagens de erro claras e proteção contra perda de dados não salvos.
- Cada link mostra suas URLs (curta e por canal) com botão "copiar" e retorno visual de sucesso.
- Estados vazios com orientação ("Cadastre seu primeiro link") em vez de telas em branco.

**Páginas públicas (preview na fase 1; produto e site na fase 2+)**
- Mobile first, pois a maior parte do tráfego virá de redes sociais e mensageiros.
- A ação principal (botões dos marketplaces) fica em destaque, visível sem rolar a página no celular.
- Orçamento de desempenho: CSS crítico inline ou arquivo único pequeno, JavaScript mínimo, imagens responsivas (`srcset`), formatos modernos (WebP/AVIF), `loading="lazy"` abaixo da dobra, dimensões declaradas para evitar mudança de layout.
- Metas de Core Web Vitals: LCP < 2,5 s, CLS < 0.1, INP < 200 ms.
- Aparência consistente com o design system do painel, com identidade visual configurável por organização no futuro (logo e cor principal).

**Página de produto: inspiração [REFERÊNCIA DO RESPONSÁVEL]**

Referência: página de produto do site [versus.com/br/samsung-galaxy-s25-ultra](https://versus.com/br/samsung-galaxy-s25-ultra), de um site especialista em celulares. A página não pôde ser acessada automaticamente (HTTP 403); a análise abaixo vem de uma **captura de tela fornecida pelo responsável** (`layout-referencia.png`, nesta pasta). Serve como **inspiração de estrutura e estilo, não como cópia**: marca, textos e dados da referência não devem ser reproduzidos.

**Estrutura observada na referência (de cima para baixo)**
1. Faixa promocional e cabeçalho com logotipo, categorias, busca e menu.
2. **Visão geral:** imagem do produto, título, posição no ranking, pontuação comparada com a média da categoria e botão de oferta. *(ranking e pontuação: fora de escopo)*
3. **Veredito:** frase-resumo em destaque. *(fora de escopo)*
4. **"Por que é melhor que a média?":** gráfico radar em 6 áreas e tabela "onde se destaca". *(fora de escopo)*
5. **Barra fixa** ao rolar a página, com miniatura do produto e botão de ofertas.
6. Atalhos de comparação com outros produtos.
7. **"Melhor preço agora":** tabela de lojas, com abas Novo/Usado, selo de melhor preço e alerta de preço. *(comparação de preços e alerta: fora de escopo; a lista de lojas vira "Onde comprar")*
8. **Alternativas mais baratas:** carrossel de cartões.
9. **Ficha completa:** acordeão com 9 grupos (design, tela, desempenho, câmeras, sistema, bateria, áudio, conectividade, diversos); cada linha mostra o valor e uma barra comparando com os demais produtos. *(barras comparativas: fora de escopo)*
10. **Avaliações de donos:** nota geral, notas por critério e cartões de comentários. *(fora de escopo)*
11. **Continue explorando:** comparações e melhores da categoria.
12. Chamada final e rodapé em colunas (categorias, institucional, jurídico).

**Decisão de aproveitamento**

| Elemento da referência                               | Decisão                        | Adaptação para o Hub Links                                                                                             |
| ---------------------------------------------------- | ------------------------------ | ---------------------------------------------------------------------------------------------------------------------- |
| Visão geral (imagem, título, resumo, ação principal) | **Adotar (fase 2)**            | Imagem, título, descrição curta e botão principal "Ver onde comprar"                                                   |
| Barra fixa ao rolar                                  | **Adotar (fase 2)**            | Barra fixa com miniatura, título e botão que leva à seção de links; no celular, barra de ação no rodapé                |
| Lista de lojas (origem: "Melhor preço agora")        | **Adotar como "Onde comprar" (fase 2)** | Seção com os links de afiliado do produto agrupados por marketplace, com logotipo e botão. É o propósito central do hub (seção 1). **Sem** preço, ordenação por preço, selo de melhor preço nem abas Novo/Usado |
| Alternativas mais baratas (carrossel)                | **Adotar adaptado (fase 2)**   | Carrossel de **produtos relacionados**, sem preço                                                                      |
| Ficha completa em acordeão por grupo                 | **Adotar (fase 2)**            | Grupos de especificações **definidos por produto** (ver abaixo), exibidos como acordeão, com âncoras. Sem barras comparativas |
| Breadcrumb, cabeçalho com busca, rodapé em colunas   | **Adotar (fase 3)**            | Parte do site público                                                                                                  |
| "Continue explorando" e melhores da categoria        | **Adotar (fase 3)**            | Grades de produtos por categoria e relacionados                                                                        |
| Comparador entre produtos                            | **Adiar**                      | Possível evolução, depois de existir um catálogo grande                                                                |
| Veredito (frase-resumo)                              | **Fora de escopo [DECIDIDO]**  | Foge do propósito                                                                                                      |
| Radar, pontuação, ranking e barras comparativas      | **Fora de escopo [DECIDIDO]**  | Foge do propósito                                                                                                      |
| Avaliações de donos                                  | **Fora de escopo [DECIDIDO]**  | Foge do propósito                                                                                                      |
| "Melhor preço agora" (comparação de preços)          | **Fora de escopo [DECIDIDO]**  | Foge do propósito; o sistema não mantém preço                                                                          |
| Alerta de preço                                      | **Fora de escopo [DECIDIDO]**  | Foge do propósito                                                                                                      |

Itens **fora de escopo** não devem ser implementados nem reintroduzidos em fases futuras sem nova decisão do responsável.

**Especificações por produto (ficha completa)**

Produtos de categorias diferentes têm especificações muito diferentes, então **não existe um esquema fixo de campos**. Cada produto define os próprios **grupos de especificações**, e cada grupo é exibido como um item do acordeão.

- Um produto tem N **grupos** (por exemplo, "Tela", "Bateria", "Câmeras"), com nome e posição.
- Cada grupo tem N **itens** nome/valor (por exemplo, "Tamanho" = `6,9"`), com posição.
- No cadastro do produto, o painel permite **adicionar, remover e reordenar** grupos e itens (HTMX + Alpine.js), sem limite fixo.
- Na página pública, cada grupo é um item de acordeão com âncora própria. Grupos recolhidos por padrão, exceto o primeiro, com um controle "Mostrar todos".
- Valores são texto simples. Não há tipos, unidades nem comparação numérica.
- **Evolução possível (fora da fase 2):** modelos de grupos reaproveitáveis por categoria, para agilizar o cadastro (por exemplo, "modelo Celular"). Não faz parte do escopo atual.

**Estilo visual observado:** cabeçalho e abertura em tema escuro com destaque em cor viva (verde e azul), corpo em fundo claro com cartões de bordas suaves, tipografia sans-serif, muito espaço em branco, selos pequenos de status e grande densidade de informação organizada em seções. A implementação deve seguir esse espírito (clareza, hierarquia e densidade), dentro do design system da seção 11.1.

**Implicações para o modelo de dados (a detalhar na fase 2):**
- `product_spec_groups(id, org_id, product_id, name, position)`
- `product_spec_items(id, org_id, group_id, name, value, position)`
- relação de produtos relacionados (`product_related(product_id, related_product_id, position)`);
- logotipo por marketplace (`marketplaces.logo_url`);
- lista de prós e contras: **fora do escopo** por ora, pois a ficha por grupos cobre a necessidade.

**SEO:** `h1` único, hierarquia correta de títulos, `schema.org/Product` (sem `offers`), metatags e Open Graph, URL canônica, âncoras na ficha técnica.

**Entregáveis de design para o épico 7:** guia curto de estilo (tokens e componentes) em `docs/`, e uma tela de referência com todos os componentes (`/admin/ui`, restrita ao ambiente de desenvolvimento).

## 12. Estrutura de repositório sugerida (Go)

```
/cmd/server          ponto de entrada
/internal/config     leitura de variáveis de ambiente
/internal/domain     entidades e regras
/internal/store      acesso ao PostgreSQL e migrações
/internal/redirect   handler de redirecionamento e filtro de bots
/internal/events     fila e worker de gravação
/internal/stats      consultas agregadas
/internal/maintenance jobs diários de agregação e limpeza (seção 8.1)
/internal/api        handlers JSON administrativos
/internal/admin      handlers e templates do painel web (fase 1)
/internal/web        templates e SSR público (fase 2+)
/web/static          CSS compilado, JS, fontes e imagens estáticas
/web/assets          entrada do Tailwind (CSS fonte) e configuração de tokens
/migrations          SQL versionado
/deploy              Dockerfile, docker-compose
/docs                ADRs e esta especificação
```

## 13. Plano de implementação (fase 1)

Cada épico deve resultar em código testado e entregável de forma independente.

1. **Fundação:** estrutura do projeto, config, conexão com banco, migrações, `/healthz`, Dockerfile e compose.
2. **Modelo e CRUD:** organizations (org padrão), users, marketplaces, channels, affiliate_links, short_codes; autenticação do administrador (sessão e CSRF), criação do administrador inicial por variáveis de ambiente.
3. **Redirecionamento:** rotas `/{code}` e `/{canal}/{code}`, cache de resolução, 302, 404.
4. **Eventos:** fila assíncrona, worker em lote, cálculo de `visitor_id` por HMAC (sem cookie), `visitor_seen`, flags de unicidade.
5. **Bots e preview:** detecção de crawlers, página Open Graph, flag `is_bot`.
6. **Estatísticas e retenção:** endpoints de agregação (API JSON), tabela `click_daily`, jobs diários de agregação e limpeza (seção 8.1).
7. **Painel web:** design system e pipeline de CSS (seção 11.1), layout base, telas de login, CRUD (marketplaces, canais, links), detalhe do link com URLs copiáveis e dashboards com seletor de período. Pode começar em paralelo ao épico 2, mas as telas de métricas dependem do épico 6.
8. **Endurecimento:** rate limit, métricas, testes de carga do redirecionamento, documentação de operação.

### Critérios de aceite da fase 1
- Criar um link de afiliado gera código curto e responde em `/{code}` e `/{canal}/{code}`.
- Acessar o link redireciona com 302 ao destino correto.
- A prévia de crawler (WhatsApp/Telegram) mostra título e imagem e **não** altera os contadores.
- Marketplace com política `direct` exibe a URL original e aparece como "não rastreável" nas estatísticas.
- O painel funciona em tela de celular e alterna entre modo claro e escuro; as telas seguem os componentes do design system (seção 11.1).
- Pelo painel web é possível, sem usar a API diretamente, entrar, cadastrar marketplace, canal e link, copiar as URLs geradas e ver cliques totais e únicos por link e por canal.
- Derrubar o worker de eventos não impede o redirecionamento.
- Após o job de agregação e limpeza, as contagens totais e únicas de um período antigo permanecem idênticas às de antes da remoção dos eventos brutos; o job é idempotente.
- Nenhuma resposta ao visitante (redirecionamento, preview, beacon) contém o cabeçalho `Set-Cookie`.
- Dois acessos de mesmo IP e user-agent ao mesmo link contam como 2 cliques e 1 único; mudar o user-agent conta como novo único.
- `GET /privacidade` responde com a política de privacidade, e a página de preview exibe o aviso com link para ela.
- A aplicação sobe com `docker compose up` a partir de variáveis de ambiente.

## 14. Riscos

| Risco | Mitigação |
|---|---|
| Políticas de marketplaces proíbem encurtar ou mascarar links | `shorten_policy` por marketplace e por link (ADR-007) |
| Contagem única imprecisa (IP móvel, CGNAT) | Aceito conscientemente em troca de não usar cookies (ADR-009); normalização de IPv6; limitações visíveis no painel |
| Crescimento da tabela de eventos | Retenção em camadas (seção 8.1); índices por `(target_id, occurred_at)`; particionamento se necessário |
| Escopo cresce antes do núcleo ficar sólido | Fases com critérios de aceite; fase 2 só começa após a fase 1 |
| LGPD no rastreamento | Sem cookies de rastreamento, sem IP puro (apenas HMAC), retenção limitada, aviso e `/privacidade` desde a fase 1; revisão jurídica da base legal antes da fase 5 |

## 15. Questões em aberto

1. **Aproveitamento da referência:** decidido (seção 11.1), exceto o comparador entre produtos, que segue adiado.
2. **Domínio curto:** ainda não existe; para desenvolvimento usar `localhost`. O `BASE_URL` é configurável.
3. **Métrica de links `direct`:** não há clique rastreado. Proposta: na fase 1, nenhuma métrica (apenas a marca "não rastreável"); a partir da fase 2, a `view` da página do produto serve como métrica parcial. Confirmar.

**Decididas:** Go (ADR-001), PostgreSQL (ADR-002), unicidade permanente, beacon para visitas, identificação do visitante sem cookies (ADR-009), aviso de privacidade desde a fase 1, painel web desde a fase 1 com HTMX (ADR-008), retenção em camadas (seção 8.1; prazos revisáveis).

## 16. Orientações para agentes de IA

- Não decidir itens listados em **Questões em aberto** nem trocar decisões dos ADRs sem aprovação do responsável.
- Implementar épico por épico, na ordem da seção 13, com testes automatizados.
- Não gravar IP puro, nem segredos no repositório.
- Toda consulta de domínio deve filtrar por `org_id`.
- Não implementar itens da lista de **fora de escopo permanente** (seção 1) nem os marcados como fora de escopo na seção 11.1.
- Itens marcados como **[ASSUNÇÃO]** podem ser seguidos, mas devem ser registrados em `docs/` para confirmação.
- Ao encontrar ambiguidade, registrar a dúvida em `docs/` e perguntar, em vez de assumir.
