# Contrato: painel `/admin` (HTML + HTMX)

Todas as rotas exigem sessão, exceto o login; sem sessão → `303` para `/admin/login?next=…`.
Formulários levam o campo oculto `csrf_token`; requisições HTMX enviam `X-CSRF-Token`
(configurado em `hx-headers` no layout). Requisições com `HX-Request` recebem fragmentos;
as demais, a página completa.

| Rota | Tela / ação |
|---|---|
| `GET /admin/login`, `POST /admin/login` | Formulário. Erro genérico; bloqueio após 5 falhas em 15 min (mensagem com prazo) |
| `POST /admin/logout` | Encerra a sessão e apaga o cookie |
| `GET /admin` | Dashboard: cartões (cliques, únicos, links ativos), série diária (uPlot), top links, comparação entre canais, seletor de período, aviso de limitações da contagem |
| `GET /admin/links` | Lista com busca, filtro por marketplace, ordenação e paginação (HTMX); cliques e únicos; "não rastreável" para `direct` |
| `GET /admin/links/new`, `POST /admin/links` | Formulário de criação com validação inline |
| `GET /admin/links/{id}` | Detalhe: URLs (curta e por canal) com botão copiar; estatísticas por canal; série diária |
| `GET /admin/links/{id}/edit`, `POST /admin/links/{id}` | Edição; aviso ao sair com alterações não salvas |
| `POST /admin/links/{id}/delete`, `POST /admin/links/{id}/restore` | Lixeira, com confirmação em modal |
| `GET /admin/marketplaces` (+ new/edit/delete/restore) | CRUD em modal (entidade simples) |
| `GET /admin/channels` (+ new/edit/delete/restore) | CRUD em modal; a confirmação de exclusão informa quantos links terão a URL do canal invalidada |
| `GET /admin/trash` | Itens na lixeira com dias restantes e ação de restaurar |
| `GET /admin/ui` | Guia de componentes; responde `404` fora de `APP_ENV=development` |

**Componentes**: botão, campo, tabela responsiva (cartões no celular), cartão, selo de status,
modal, toast, abas, seletor de período, estado vazio, esqueleto de carregamento e alternância de
tema (`localStorage`, respeitando `prefers-color-scheme`).

**Período**: os parâmetros `period`, `from` e `to` são os mesmos da API e são preservados na URL
(`hx-push-url`).
