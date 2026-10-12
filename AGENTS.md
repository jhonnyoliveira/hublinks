# Orientações para agentes — Hub Links

## Fonte de verdade

- Leia `docs/referencias/escopo.md` antes de implementar. Esse documento contém as decisões, o modelo indicativo, as fases e os critérios de aceite.
- Instruções explícitas do responsável prevalecem sobre este arquivo. Não altere ADRs nem resolva questões em aberto sem aprovação dele.
- A referência `docs/referencias/layout-referencia.png` serve de inspiração visual nas fases pertinentes; não copie marca, textos ou dados.
- Comunique resultados e dúvidas em português do Brasil.

## Momento atual e próximo trabalho

O repositório já contém a aplicação Go, a fundação de configuração, banco, migrações, saúde, autenticação, renderização por `html/template`, catálogo e redirecionamento. O `package.json` continua genérico e não define a stack de testes.

A fundação (T001–T032) foi auditada contra a descrição de cada tarefa e a **User Story 1** (T033–T052) está implementada, testada e **aceita no checkpoint**: os cenários 1–5 da spec e os passos 1, 3, 6, 9 e 10 do `quickstart.md` foram exercitados contra o servidor real (curl e Chromium headless). Ficaram de fora, por dependerem de outras histórias, o deploy por compose (US7), a contagem de cliques e a resiliência do worker (US2), a prévia de crawler (US3) e a agregação/limpeza (US6).

A **User Story 2 — Ver cliques totais e únicos** (T053–T069) está implementada e testada: fila e worker de eventos (lotes, retentativas, reinício após pânico, drenagem no desligamento), flags de unicidade, estatísticas por link/canal/dia (`internal/stats`), rotas `/api/v1/stats/*`, dashboard com gráfico (uPlot), estatísticas nas telas de links e servidor de métricas em `METRICS_ADDR`. Os cenários da spec foram exercitados contra o servidor real (curl): 2 cliques/1 único com o mesmo UA, novo único com outro UA e alcance por canal. O gráfico (`web/static/js/chart.js`) foi verificado em navegador pelo responsável no passo 9 do `quickstart.md`. O checkpoint foi aceito pelo responsável.

A **User Story 3 — prévia de compartilhamento** (T070–T077) está implementada e testada: crawlers conhecidos recebem a prévia Open Graph (`redirect/preview.go`) sem gerar evento, a detecção de automação (5 códigos distintos em 10 s pelo mesmo IP → `is_bot`) limpa as chaves expiradas, e `/privacidade` e o aviso das páginas públicas seguem a configuração. Foi exercitada contra o servidor real (curl): prévia `200` com metatags, o 5º código distinto gravado como robô e fora das estatísticas. O checkpoint da US3 foi aceito pelo responsável; o item 5 do `quickstart.md` está coberto.

A **User Story 4 — painel no celular, claro e escuro** (T078–T084) está implementada e testada no servidor: tabelas que viram cartões abaixo de 40 rem (`table-cards` + `data-label`), estados vazios com orientação (incluindo o onboarding do dashboard), token `--line-strong` (contorno de campos e interruptor a 3:1), proteção contra perda de alterações (`unsaved.js`, sem Alpine, por delegação de eventos), foco do menu móvel, `docs/guia-de-estilo.md` e testes de rótulos, `alt`, `lang`, tema e `/admin/ui` só em desenvolvimento. O responsável validou o passo 9 do `quickstart.md` em navegador (375 px, dois temas, teclado, gráfico e `unsaved.js`); este ambiente não tem Chromium. O checkpoint da US4 foi aceito pelo responsável.

A **User Story 5 — acesso restrito** (T085–T089) está implementada e testada: o bloqueio de login (5 falhas em 15 min por IP ou por e-mail normalizado → 15 min, `429` com o prazo na mensagem e `Retry-After`) e o limite de taxa do redirecionamento público (300/min por IP; a 301ª recebe `429` sem chegar ao handler) agora respeitam `TRUSTED_PROXIES` (antes, atrás de um proxy, todos compartilhavam a chave do proxy), descartam entradas ociosas e têm testes com relógio injetável e `-race`. O limite usa janela deslizante aproximada, e não `x/time/rate` com rajada de 60 como sugeria o T089, porque o T086 e o FR-034 exigem que as 300 primeiras requisições passem. Foi exercitada contra o servidor real (curl, com proxy confiável): a 6ª tentativa de login com a senha certa é recusada, 300 requisições passam e a 301ª recebe `429`. O checkpoint da US5 foi aceito pelo responsável.

A **User Story 6 — histórico preservado com retenção automática** (T090–T099) está implementada e testada: consolidação diária por dia local em `REPORT_TZ` (recupera todos os dias passados pendentes, nunca o dia corrente), limpeza em lotes de 10 000 só de dias consolidados, expiração da lixeira (`purged_at` e `visitor_seen` apagado) com invalidação do cache de resolução, fuso fixado na primeira consolidação, agendador com `pg_try_advisory_lock` e métricas de duração/falhas, e o comando `hublinks maintenance run [--day YYYY-MM-DD]`. Foi exercitada contra o banco real pelo binário: 2 360 eventos → totais de cliques e únicos idênticos depois da limpeza, segunda execução sem efeito, `--day` recusado para hoje e para dias fora da retenção, servidor recusando `REPORT_TZ` diferente e link com 31 dias na lixeira purgado. O checkpoint da US6 foi aceito pelo responsável.

Atenção: `--day` refaz `click_daily` a partir de `click_events`; por isso é recusado (`ErrDayCleaned`) para dias já consolidados e fora de `EVENTS_RETENTION_DAYS`, cujos eventos podem já ter sido removidos.

A **User Story 7 — subir o serviço com um comando** (T100–T103) foi auditada e corrigida: o estágio de CSS do `deploy/Dockerfile` copiava só `web/assets`, sem `base.css` nem os templates que `app.css` importa e varre (o build da imagem falharia), não levava o `manifest.json` para o estágio de build e usava Alpine (o Tailwind standalone é glibc). Agora usa Debian slim e copia todas as entradas. Também entraram `hublinks healthcheck` (a imagem final não tem shell), o healthcheck e o `restart` no compose, `POSTGRES_PASSWORD` (a `DATABASE_URL` do compose é montada a partir dela), `time/tzdata` embutido e testes que protegem o Dockerfile, o compose e a cobertura do `.env.example`. **T104 foi validado pelo responsável** (`docker build` e `docker compose up`, passo 2 do `quickstart.md`); este ambiente não tem Docker, e antes disso o estágio de CSS, o `go build` estático e a execução do binário haviam sido reproduzidos à mão.

A **Fase 10 — endurecimento** (T105–T108 e T110) está concluída: métricas HTTP por rota (padrão do mux) e status e o histograma `hublinks_redirect_duration_seconds`; `scripts/loadtest.sh` executado de verdade (500 req/s por 60 s: p95 de 0,88 ms no cliente e ≤ 0,5 ms no servidor, 0 eventos descartados); `docs/operacao.md` completo (variáveis, backup, `PEPPER`/`REPORT_TZ`, jobs, métricas e alertas, carga); e a auditoria de privacidade, agora automatizada em `internal/audit` (IP e cookies só nos arquivos permitidos, nenhuma coluna de IP, nenhum IP em logs).

**A fase 1 está concluída e aceita** (T104 e T109 incluídos; checkpoints das histórias 2 a 7 aceitos em 2026-10-12). O registro está em `docs/verificacao.md`; as decisões e assunções confirmadas, em `docs/pendencias.md` (nada pendente).

**Fase 2 — Páginas de produto (CMS): autorizada pelo responsável em 2026-10-12** e especificada em `specs/002-paginas-de-produto/` (`spec.md`, `plan.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md` e `tasks.md`, com 61 tarefas em 9 fases), na branch `002-paginas-de-produto`. Ela ainda **não foi implementada**: nenhuma tarefa do `tasks.md` está concluída. Implemente somente o que o `tasks.md` descreve, na ordem dos IDs e das fases (Setup, Foundational e depois US1 a US6), com testes primeiro, e siga as instruções do topo daquele arquivo. Decisões da fase 2 que valem sempre: imagens enviadas pelo painel e servidas pelo próprio serviço (sem recursos de terceiros), a página abre também no código curto do produto com `/p/{slug}` como canônico, o catálogo é dimensionado para até 1.000 produtos publicados, e os links `direct` continuam sem métrica (a visita mede a página, não o link). Fica para a fase 3, e **não** deve ser feito agora: `sitemap.xml`, `robots.txt`, home, busca e navegação por categorias no site público. Quando um épico da fase 2 terminar, atualize esta seção.

Pontos de atenção:

- **Testes de integração** exigem `TEST_DATABASE_URL`; sem ela, são pulados em silêncio. Use `make test-ci`, que falha se houver testes pulados. Detalhes e comandos em `docs/desenvolvimento.md`.
- **CSS:** a fonte única é `web/static/css/base.css`; `make css` gera o build do Tailwind com hash. Arquivos de template não podem começar com `_` (o `go:embed` os ignora).
- **JavaScript do painel** (HTMX, Alpine, modal, tema) é verificado em Chromium headless com Playwright fora do repositório; ainda não há suíte de navegador versionada.
- **Métricas:** `METRICS_ADDR` (padrão `:9091`; vazio desativa) serve `/metrics` em porta separada. O processo usa `config.LoadEnv`, que distingue vazio de não definido; `config.Load` (testes) não distingue.
- **Verificação de origem** (`http.CrossOriginProtection` e `RequireCSRF`) compara `Origin` com o `Host` da requisição: o proxy reverso do deploy precisa preservá-lo.
- **Fila de eventos:** `server.Build(ctx, …)` inicia o worker com o contexto do processo; `Run` espera `App.Drained` antes de fechar o pool. `server.New` (testes) não tem ciclo de vida.

- Implemente somente o trabalho solicitado, em incrementos entregáveis. Criar este arquivo não autoriza iniciar a aplicação.
- Siga a sequência da seção 13 do escopo: fundação; modelo e CRUD; redirecionamento; eventos; bots e preview; estatísticas e retenção; painel; endurecimento.
- O CMS de produtos (fase 2) segue `specs/002-paginas-de-produto/tasks.md`. Não antecipe site público e catálogo público (fase 3), multiusuário (fase 4) ou SaaS (fase 5).
- Preserve a infraestrutura de desenvolvimento existente ao adicionar o deploy da aplicação; inspecione os arquivos Docker e `.devcontainer/` antes de modificá-los.

## Arquitetura e organização

- Go e PostgreSQL; um serviço/binário e um banco, com deploy por imagem Docker multi-stage.
- Painel e páginas públicas renderizados com `html/template`. Painel com HTMX, API JSON em `/api/v1` e camada de serviço compartilhada; sem SPA.
- Use a estrutura sugerida na seção 12 conforme cada pacote se tornar necessário: `cmd/server`, `internal/{config,domain,store,redirect,events,stats,maintenance,api,admin,web}`, `migrations`, `web/{assets,static}`, `deploy` e `docs`. Evite pacotes vazios apenas para reproduzir a árvore.
- Configuração por ambiente; exemplos contêm somente valores fictícios. Documente variáveis, comandos e dependências quando forem introduzidos.
- Migrações são versionadas desde a fundação. IDs em UUID e timestamps em UTC, conforme o modelo do escopo.
- Toda tabela de domínio deve ter `org_id`, e toda consulta de domínio deve ser restrita à organização. A resolução inicial do código global identifica a organização antes das operações de domínio. Consulte a seção 6 para entidades globais e registre inconsistências antes de definir o esquema.
- Entidades principais usam exclusão lógica. Não substitua as regras específicas de limpeza de dados por exclusão lógica genérica.

## Invariantes do produto

- Redirecionamentos usam **302**, nunca 301. Destinos cadastrados aceitam somente `http` e `https`.
- Código curto: exatamente sete caracteres `[0-9a-z]`, único globalmente. Segmentos de canal são únicos por organização e respeitam as palavras reservadas da seção 6.
- Gravação de eventos é assíncrona, em fila de memória e lotes. Fila cheia ou falha do worker não pode bloquear o redirecionamento; descarte deve ser contabilizado em métrica.
- Crawlers conhecidos recebem preview Open Graph sem gerar eventos. Eventos classificados como bots não entram nas estatísticas.
- Política `direct` exibe a URL original e a indicação **“não rastreável”**, nunca zero cliques. O responsável decidiu que esses links não terão métrica, também nas fases seguintes: não crie contador nem proxy de acesso para eles.
- Unicidade é permanente por alvo e por alvo/canal; preserve a distinção entre `is_unique_target` e `is_unique_url`. O worker calcula as flags ao inserir, com tratamento de concorrência.
- Agregação diária deve ser idempotente. Só remova eventos de dias já agregados; preserve contagens após a limpeza. Retenções padrão: eventos reais 395 dias, bots 30 dias, agregados indefinidamente e `visitor_seen` enquanto o alvo existir.

## Segurança e privacidade

- Nunca persista IP puro nem o exponha em logs. Nunca versione senhas, tokens, `PEPPER` ou outros segredos.
- Identificador de visitante: HMAC-SHA256 de IP normalizado e user-agent, conforme seção 8; IPv4 completo e IPv6 `/64`. Não rotacione o `PEPPER`, pois isso altera a unicidade permanente.
- Confie em `X-Forwarded-For` somente quando a conexão vier de proxy configurado em `TRUSTED_PROXIES`.
- Não emita cookies de rastreamento. Respostas públicas de redirecionamento, preview e beacon não podem conter `Set-Cookie`.
- Sessão administrativa usa cookie `HttpOnly`, `Secure`, `SameSite=Lax`, com proteção CSRF. Senhas usam argon2 ou bcrypt; o administrador inicial é criado por ambiente somente quando não houver usuários.
- Inclua `/privacidade` e aviso informativo nas páginas HTML públicas na fase 1, conforme o escopo. Não transforme o aviso em um fluxo de consentimento.

## Interface, quando chegar ao épico correspondente

- Interface em português do Brasil, mobile first, acessível e com modos claro e escuro.
- Tailwind CSS pelo binário standalone, sem depender de Node/npm; compilação no Docker e CSS minificado com hash no nome.
- HTMX para interações com servidor; Alpine.js apenas para interações locais pequenas. SVG inline para ícones e fontes hospedadas localmente.
- Tokens e componentes reutilizáveis conforme seção 11.1. Bibliotecas de gráficos somente nas telas que as utilizam; nenhum recurso de terceiros nas páginas públicas.
- O épico 7 inclui guia de estilo em `docs/` e `/admin/ui` disponível apenas em desenvolvimento.

## Limites de escopo

Não implemente integrações com APIs de marketplaces, conversões/vendas/comissões, preços ou alertas de preço, ranking/pontuação/radar, avaliações de usuários ou veredito. Respeite também os itens adiados e fora de escopo da seção 11.1.

## Forma de trabalhar e validar

- Inspecione o estado do repositório e preserve alterações do responsável. Mantenha mudanças focadas no épico e na solicitação atual.
- Para ambiguidades do escopo, registre a dúvida em `docs/` e consulte o responsável antes de implementar a parte dependente; continue o trabalho independente. Registre itens `[ASSUNÇÃO]` em `docs/` para confirmação.
- Cada épico deve ter testes automatizados dos comportamentos relevantes e instruções reproduzíveis de execução. Use testes de integração com PostgreSQL real para migrações, integridade e consultas quando essas partes forem implementadas.
- Quando o módulo Go existir, execute `gofmt` nos arquivos alterados, `go test ./...` e `go vet ./...`. Use `go test -race ./...` quando alterar filas, workers, caches ou outra lógica concorrente.
- Para a fundação, valide configuração, conexão/migrações, `/healthz`, build da imagem e inicialização pelo compose, conforme o que foi implementado.
- Não declare verificações como aprovadas se não foram executadas. Ao concluir, informe mudanças, validações realizadas e limitações concretas.
- Atualize a seção “Momento atual e próximo trabalho” quando um épico for concluído, mantendo este arquivo alinhado ao repositório e evitando duplicar toda a especificação.
