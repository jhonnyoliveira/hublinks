# Orientações para agentes — Hub Links

## Fonte de verdade

- Leia `docs/referencias/escopo.md` antes de implementar. Esse documento contém as decisões, o modelo indicativo, as fases e os critérios de aceite.
- Instruções explícitas do responsável prevalecem sobre este arquivo. Não altere ADRs nem resolva questões em aberto sem aprovação dele.
- A referência `docs/referencias/layout-referencia.png` serve de inspiração visual nas fases pertinentes; não copie marca, textos ou dados.
- Comunique resultados e dúvidas em português do Brasil.

## Momento atual e próximo trabalho

O repositório contém a especificação e infraestrutura de desenvolvimento, mas ainda não tem aplicação Go. O `package.json` é um arquivo inicial genérico; seu script de teste não valida o projeto e não define a stack.

O próximo épico é **1 — Fundação**: estrutura Go, configuração por variáveis de ambiente, conexão PostgreSQL, migrações versionadas, `GET /healthz`, imagem Docker da aplicação e compose com app e banco. Não considere esse épico implementado apenas pela existência do ambiente de desenvolvimento.

- Implemente somente o trabalho solicitado, em incrementos entregáveis. Criar este arquivo não autoriza iniciar a aplicação.
- Siga a sequência da seção 13 do escopo: fundação; modelo e CRUD; redirecionamento; eventos; bots e preview; estatísticas e retenção; painel; endurecimento.
- Não antecipe CMS, catálogo público, multiusuário ou SaaS. A fase 2 só começa após o aceite da fase 1.
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
- Política `direct` exibe a URL original e a indicação **“não rastreável”**, nunca zero cliques. Não decida a métrica futura desses links sem confirmação da questão em aberto.
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
