# Hub Links Constitution

## Core Principles

### I. Núcleo primeiro e redirecionamento resiliente
Redirecionamento e contagem confiáveis vêm antes de qualquer interface rica.
- Redirecionamentos MUST usar HTTP **302**, nunca 301, para que navegadores não cacheiem e não
  deixem de gerar eventos.
- Nenhuma escrita em banco MUST bloquear o caminho de clique; falha de rastreamento MUST NOT
  impedir que o usuário chegue ao destino.
- Destinos cadastrados MUST aceitar somente `http` e `https`, sem open redirect fora do cadastro.
- Código curto MUST ter exatamente sete caracteres `[0-9a-z]` e ser único globalmente.

Justificativa: o caminho de clique é o mais crítico do sistema (meta p95 < 50 ms no servidor).

### II. Privacidade por construção
- IP puro MUST NOT ser persistido nem aparecer em logs.
- O visitante é identificado apenas por `HMAC-SHA256(PEPPER, IP_normalizado|user_agent)` (IPv4
  completo; IPv6 `/64`). O `PEPPER` MUST NOT ser rotacionado nem versionado.
- `X-Forwarded-For` MUST ser confiado somente quando a conexão vier de proxy em `TRUSTED_PROXIES`.
- Respostas públicas (redirecionamento, preview, beacon) MUST NOT conter `Set-Cookie`. O único
  cookie é o de sessão administrativa (`HttpOnly`, `Secure`, `SameSite=Lax`, com CSRF).
- Páginas HTML públicas MUST exibir aviso informativo e link para `/privacidade`; o aviso
  MUST NOT virar fluxo de consentimento.
- Segredos, senhas e tokens MUST NOT ser versionados; exemplos usam apenas valores fictícios.

Justificativa: LGPD e menor exposição legal (ADR-009), em troca de contagem de únicos aproximada.

### III. Eventos assíncronos e contagem correta
- Eventos MUST ser gravados de forma assíncrona (fila em memória, worker em lote). Fila cheia ou
  falha do worker MUST NOT bloquear o redirecionamento; descartes MUST ser contabilizados em métrica.
- Unicidade é permanente por alvo e por alvo/canal; `is_unique_target` e `is_unique_url` MUST
  permanecer distintos e ser calculados pelo worker na inserção, com tratamento de concorrência.
- Crawlers conhecidos recebem preview Open Graph sem gerar evento; eventos `is_bot` MUST NOT
  entrar nas estatísticas.
- Links com política `direct` MUST exibir a URL original e "não rastreável", nunca zero cliques.
- A agregação diária MUST ser idempotente; a limpeza MUST remover apenas eventos de dias já
  agregados, preservando as contagens. Retenções padrão: eventos reais 395 dias, bots 30 dias,
  agregados indefinidamente, `visitor_seen` enquanto o alvo existir.

### IV. Isolamento por organização e integridade de dados
- Toda tabela de domínio MUST ter `org_id`, e toda consulta de domínio MUST filtrar por `org_id`.
  A resolução inicial do código global identifica a organização antes das operações de domínio.
- IDs MUST ser UUID e timestamps MUST ser UTC.
- Migrações MUST ser versionadas desde a fundação.
- Entidades principais MUST usar exclusão lógica, sem substituir as regras específicas de limpeza.
- Segmentos de canal MUST ser únicos por organização e respeitar as palavras reservadas do escopo.

### V. Simplicidade operacional e stack definida
- A stack é Go + PostgreSQL: um serviço/binário, um banco, deploy por imagem Docker multi-stage.
- Páginas e painel MUST ser renderizados com `html/template`; o painel usa HTMX, e a API JSON em
  `/api/v1` compartilha a camada de serviço. SPA MUST NOT ser introduzida.
- Tailwind MUST ser compilado pelo binário standalone (sem Node/npm); Alpine.js apenas para
  interações locais pequenas; fontes e ícones locais; nenhum recurso de terceiros em páginas públicas.
- Configuração MUST vir de variáveis de ambiente (12-factor), documentadas ao serem introduzidas.
- Pacotes MUST seguir a estrutura sugerida no escopo conforme necessários, sem pacotes vazios.

### VI. Teste e validação por épico (INEGOCIÁVEL)
- Cada épico MUST entregar testes automatizados dos comportamentos relevantes e instruções
  reproduzíveis de execução.
- Migrações, integridade e consultas MUST ter testes de integração com PostgreSQL real.
- Antes de concluir: `gofmt` nos arquivos alterados, `go test ./...` e `go vet ./...`; lógica
  concorrente (filas, workers, caches) MUST também passar em `go test -race ./...`.
- Verificações não executadas MUST NOT ser declaradas como aprovadas.

### VII. Escopo controlado e decisões do responsável
- O trabalho segue a sequência do escopo (fundação; modelo e CRUD; redirecionamento; eventos; bots
  e preview; estatísticas e retenção; painel; endurecimento), em incrementos entregáveis. A fase 2
  só começa após o aceite da fase 1.
- Itens fora de escopo permanente MUST NOT ser implementados: integração com APIs de marketplaces,
  conversão/vendas/comissões, preços e alertas, ranking/pontuação/radar, avaliações e veredito.
- ADRs e questões em aberto MUST NOT ser alterados ou decididos sem aprovação do responsável.
  Ambiguidades são registradas em `docs/` e consultadas; itens `[ASSUNÇÃO]` são registrados em `docs/`.

## Restrições Adicionais

- Segurança: senhas com argon2 ou bcrypt; administrador inicial criado por ambiente somente quando
  não houver usuários; rate limit nas rotas públicas e limitação de tentativas de login.
- Observabilidade: logs estruturados, métricas (requisições, eventos descartados, latência) e
  `GET /healthz`.
- Interface: português do Brasil, mobile first, acessível (WCAG AA, foco visível, teclado), modos
  claro e escuro, tokens e componentes reutilizáveis.
- Comunicação com o responsável e documentação: português do Brasil.

## Fluxo de Desenvolvimento e Qualidade

- `docs/referencias/escopo.md` é a fonte de verdade e MUST ser lido antes de implementar; instruções
  explícitas do responsável prevalecem sobre `AGENTS.md`.
- Mudanças MUST ser focadas no épico e na solicitação atual, preservando alterações do responsável
  e a infraestrutura de desenvolvimento (Docker, `.devcontainer/`).
- Revisões MUST verificar conformidade com esta constituição, em especial isolamento por `org_id`,
  ausência de IP puro/cookies/segredos e não bloqueio do redirecionamento.
- Ao concluir um épico, a seção "Momento atual e próximo trabalho" de `AGENTS.md` MUST ser atualizada.

## Governance

Esta constituição prevalece sobre outras práticas do projeto; em conflito com `AGENTS.md` ou
templates, ela vale, e o escopo (`docs/referencias/escopo.md`) vale para decisões de produto.

- Emendas: exigem proposta documentada, aprovação do responsável e, quando alterarem princípios,
  plano de migração para artefatos existentes.
- Versionamento (semântico): MAJOR para remoção ou redefinição incompatível de princípios; MINOR
  para novo princípio/seção ou expansão material; PATCH para esclarecimentos e correções de redação.
- Conformidade: todo plano, PR e revisão MUST verificar aderência aos princípios; complexidade
  adicional MUST ser justificada. Orientação de execução: `AGENTS.md`.

**Version**: 1.0.0 | **Ratified**: 2026-10-08 | **Last Amended**: 2026-10-08
