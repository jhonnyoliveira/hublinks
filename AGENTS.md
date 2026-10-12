# Orientações para agentes — Hub Links

## Fonte de verdade

- Leia `docs/referencias/escopo.md` antes de implementar. Esse documento contém as decisões, o modelo indicativo, as fases e os critérios de aceite.
- Instruções explícitas do responsável prevalecem sobre este arquivo. Não altere ADRs nem resolva questões em aberto sem aprovação dele.
- A referência `docs/referencias/layout-referencia.png` serve de inspiração visual nas fases pertinentes; não copie marca, textos ou dados.
- Comunique resultados e dúvidas em português do Brasil.
- Documentos de apoio: `docs/desenvolvimento.md` (comandos e verificação), `docs/operacao.md` (deploy, backup, métricas, carga), `docs/guia-de-estilo.md` (interface), `docs/pendencias.md` (decisões, assunções e pendências) e `docs/verificacao.md` (o que foi validado e por quem).

## Momento atual e próximo trabalho

**Fase 1 (encurtador com analytics): concluída e aceita pelo responsável em 2026-10-12.** A aplicação Go tem configuração, banco e migrações, autenticação, painel (`html/template` + HTMX), catálogo, redirecionamento, fila de eventos, estatísticas, bots e prévia, retenção e lixeira, deploy por compose e endurecimento. O histórico está em `specs/001-encurtador-analytics/` (spec, plano e `tasks.md` concluído) e o registro das validações em `docs/verificacao.md`. Pendência aberta: o item 8 de `docs/pendencias.md` (links dentro de texto, no tema escuro, só se distinguem pela cor).

**Fase 2 — Páginas de produto (CMS): autorizada pelo responsável em 2026-10-12 e ainda não implementada.** Está especificada em `specs/002-paginas-de-produto/` (`spec.md`, `plan.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md` e `tasks.md`, com 61 tarefas em 9 fases), na branch `002-paginas-de-produto`. Implemente somente o que o `tasks.md` descreve, na ordem dos IDs e das fases (Setup, Foundational e depois US1 a US6), com testes primeiro, e siga as instruções do topo daquele arquivo. Decisões da fase 2 que valem sempre:
- imagens enviadas pelo painel e servidas pelo próprio serviço, sem recursos de terceiros;
- a página abre também no código curto do produto, com `/p/{slug}` como endereço canônico;
- o catálogo é dimensionado para até 1.000 produtos publicados;
- links `direct` continuam sem métrica (a visita mede a página, não o link).

Fica para a fase 3 e **não** deve ser feito agora: `sitemap.xml`, `robots.txt`, home, busca e navegação por categorias no site público. Ao concluir um épico da fase 2, atualize esta seção.

### Pontos de atenção

- **Testes de integração** exigem `TEST_DATABASE_URL`; sem ela, são pulados em silêncio. Use `make test-ci`, que falha se houver testes pulados. Detalhes e comandos em `docs/desenvolvimento.md`.
- **CSS:** a fonte única é `web/static/css/base.css`; `make css` gera o build do Tailwind com hash. Arquivos de template não podem começar com `_` (o `go:embed` os ignora).
- **Verificação em navegador:** `scripts/browser-check` (Chromium headless com Playwright e axe-core, fora do repositório; `make browser-setup` instala e `make browser-check ARGS="--base … --path …"` roda) confere layout a 375 px sem rolagem horizontal, os dois temas, ordem de foco, acessibilidade e contraste, recursos de terceiros e erros de console, e grava capturas em `var/browser-check/`. Ele **não** substitui o olhar do responsável sobre o visual nem testa fluxos (cliques de HTMX, formulários); ainda não há suíte funcional versionada.
- **Métricas:** `METRICS_ADDR` (padrão `:9091`; vazio desativa) serve `/metrics` em porta separada. O processo usa `config.LoadEnv`, que distingue vazio de não definido; `config.Load` (testes) não distingue.
- **Verificação de origem** (`http.CrossOriginProtection` e `RequireCSRF`) compara `Origin` com o `Host` da requisição: o proxy reverso do deploy precisa preservá-lo.
- **Fila de eventos:** `server.Build(ctx, …)` inicia o worker com o contexto do processo; `Run` espera `App.Drained` antes de fechar o pool. `server.New` (testes) não tem ciclo de vida.
- **Manutenção:** `hublinks maintenance run --day` refaz `click_daily` a partir de `click_events`; por isso é recusado (`ErrDayCleaned`) para dias já consolidados e fora de `EVENTS_RETENTION_DAYS`.
- **Imagem de produção:** é `distroless` (sem shell); o healthcheck é `hublinks healthcheck`. Este ambiente não tem Docker: build da imagem e `compose up` são validados pelo responsável. Não os declare aprovados; registre a pendência em `docs/verificacao.md`.
- **Ambiente de desenvolvimento:** as portas 8080 e 9091 do compose de desenvolvimento e o banco `hublinks` são do responsável. Em testes manuais use `scripts/manual-run.sh` com `MANUAL_DB`, `HTTP_ADDR` e `METRICS_ADDR` próprios, e encerre somente os processos que você iniciou (sem `pkill` por nome).

## Arquitetura e organização

- Go e PostgreSQL; um serviço/binário e um banco, com deploy por imagem Docker multi-stage (`deploy/`).
- Painel e páginas públicas renderizados com `html/template`. Painel com HTMX, API JSON em `/api/v1` e camada de serviço compartilhada; sem SPA.
- O binário é `cmd/hublinks` (subcomandos `serve`, `migrate`, `maintenance`, `healthcheck`). A organização real do código está em `internal/`, `migrations/`, `web/{assets,static,templates}`, `deploy/` e `docs/`. Crie um pacote novo só quando houver responsabilidade própria (a fase 2 cria `internal/media`); evite pacotes vazios.
- Configuração por ambiente; exemplos contêm somente valores fictícios. Documente variáveis, comandos e dependências quando forem introduzidos.
- Migrações são versionadas. IDs em UUID e timestamps em UTC, conforme o modelo do escopo.
- Toda tabela de domínio deve ter `org_id`, e toda consulta de domínio deve ser restrita à organização. A resolução inicial do código global identifica a organização antes das operações de domínio. Consulte a seção 6 do escopo para entidades globais e registre inconsistências antes de definir o esquema.
- Entidades principais usam exclusão lógica. Não substitua as regras específicas de limpeza de dados por exclusão lógica genérica.

## Invariantes do produto

- Redirecionamentos de links usam **302**, nunca 301. Destinos cadastrados aceitam somente `http` e `https`.
- Código curto: exatamente sete caracteres `[0-9a-z]`, único globalmente. Segmentos de canal são únicos por organização e respeitam as palavras reservadas da seção 6 do escopo.
- Gravação de eventos é assíncrona, em fila de memória e lotes. Fila cheia ou falha do worker não pode bloquear o redirecionamento nem a abertura de páginas; descarte deve ser contabilizado em métrica.
- Crawlers conhecidos recebem preview Open Graph sem gerar eventos. Eventos classificados como bots não entram nas estatísticas.
- Política `direct` exibe a URL original e a indicação **"não rastreável"**, nunca zero cliques. O responsável decidiu que esses links não terão métrica: não crie contador nem proxy de acesso para eles.
- Unicidade é permanente por alvo e por alvo/canal; preserve a distinção entre `is_unique_target` e `is_unique_url`. O worker calcula as flags ao inserir, com tratamento de concorrência.
- Agregação diária deve ser idempotente. Só remova eventos de dias já agregados; preserve contagens após a limpeza. Retenções padrão: eventos reais 395 dias, bots 30 dias, agregados indefinidamente e `visitor_seen` enquanto o alvo existir.

## Segurança e privacidade

- Nunca persista IP puro nem o exponha em logs. Nunca versione senhas, tokens, `PEPPER` ou outros segredos.
- Identificador de visitante: HMAC-SHA256 de IP normalizado e user-agent, conforme seção 8 do escopo; IPv4 completo e IPv6 `/64`. Não rotacione o `PEPPER`, pois isso altera a unicidade permanente.
- Confie em `X-Forwarded-For` somente quando a conexão vier de proxy configurado em `TRUSTED_PROXIES`.
- Não emita cookies de rastreamento. Respostas públicas (redirecionamento, preview, páginas de produto, beacon e mídia) não podem conter `Set-Cookie`.
- Sessão administrativa usa cookie `HttpOnly`, `Secure`, `SameSite=Lax`, com proteção CSRF. Senhas usam argon2 ou bcrypt; o administrador inicial é criado por ambiente somente quando não houver usuários.
- Toda página HTML pública exibe o aviso informativo de privacidade com link para `/privacidade`. Não transforme o aviso em um fluxo de consentimento.
- Texto vindo do administrador é sempre escapado por `html/template`; nunca emita HTML cru.

## Interface

- Interface em português do Brasil, mobile first, acessível e com modos claro e escuro.
- Tailwind CSS pelo binário standalone, sem depender de Node/npm no build da aplicação; compilação no Docker e CSS minificado com hash no nome.
- HTMX para interações com servidor; Alpine.js apenas para interações locais pequenas. SVG inline para ícones e fontes hospedadas localmente.
- Tokens e componentes reutilizáveis conforme seção 11.1 do escopo e `docs/guia-de-estilo.md`; componentes novos entram em `/admin/ui` (disponível só em desenvolvimento). Bibliotecas de gráficos somente nas telas que as utilizam; nenhum recurso de terceiros nas páginas públicas.

## Limites de escopo

Não implemente integrações com APIs de marketplaces, conversões/vendas/comissões, preços ou alertas de preço, ranking/pontuação/radar, avaliações de usuários ou veredito. Respeite também os itens adiados e fora de escopo da seção 11.1 do escopo. Site público e catálogo público são da fase 3, multiusuário da fase 4 e SaaS da fase 5; não os antecipe.

## Forma de trabalhar e validar

- Implemente somente o trabalho solicitado, em incrementos entregáveis. Inspecione o estado do repositório e preserve alterações do responsável. Mantenha mudanças focadas no épico e na solicitação atual.
- Inspecione os arquivos Docker, `Dockerfile.dev` e `.devcontainer/` antes de modificá-los e preserve a infraestrutura de desenvolvimento existente.
- Para ambiguidades do escopo, registre a dúvida em `docs/pendencias.md` e consulte o responsável antes de implementar a parte dependente; continue o trabalho independente. Registre itens `[ASSUNÇÃO]` em `docs/` para confirmação.
- Cada épico deve ter testes automatizados dos comportamentos relevantes e instruções reproduzíveis de execução. Use testes de integração com PostgreSQL real para migrações, integridade e consultas.
- Antes de concluir, execute `gofmt` nos arquivos alterados, `go test ./...` e `go vet ./...`. Use `go test -race ./...` quando alterar filas, workers, caches ou outra lógica concorrente.
- Não declare verificações como aprovadas se não foram executadas. Ao concluir, informe mudanças, validações realizadas e limitações concretas.
- Atualize a seção "Momento atual e próximo trabalho" quando um épico for concluído, mantendo este arquivo alinhado ao repositório e evitando duplicar a especificação.
