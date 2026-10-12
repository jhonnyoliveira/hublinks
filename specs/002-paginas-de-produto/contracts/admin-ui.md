# Contrato: painel `/admin` (Fase 2)

Acrescenta-se a `specs/001-encurtador-analytics/contracts/admin-ui.md`. Mesmos padrões: sessão,
CSRF em escritas, HTMX para trocar contêineres, tabelas que viram cartões abaixo de 40 rem,
estados vazios com orientação, proteção contra perda de alterações, modos claro e escuro.

| Rota | Tela / ação |
|---|---|
| `GET /admin/products` | listagem com busca, filtros (status, categoria), paginação e visitas/cliques do período; estado vazio "Cadastre seu primeiro produto" |
| `GET /admin/products/new`, `POST /admin/products` | formulário de criação (título, slug sugerido, categoria) |
| `GET /admin/products/{id}` | detalhe com endereço amigável e código curto para copiar, estatísticas separadas de visitas e cliques (por canal e por dia) e os links do produto com a marca "não rastreável" nos `direct` |
| `GET /admin/products/{id}/edit`, `POST /admin/products/{id}` | formulário completo em seções: dados, imagens, links vinculados, ficha técnica, relacionados, SEO |
| `GET /admin/products/{id}/preview` | pré-visualização do rascunho, com aviso "Rascunho" e sem registrar eventos |
| `POST /admin/products/{id}/publish`, `…/unpublish` | publica/despublica; confirmação ao despublicar |
| `GET/POST /admin/products/{id}/delete`, `POST …/restore` | confirmação, lixeira e restauração |
| `GET /admin/categories` e demais rotas CRUD | mesmo padrão dos canais; exclusão bloqueada indica os produtos |
| `POST /admin/media` | envio de imagem pelo formulário (HTMX); devolve o trecho com a miniatura e o campo oculto |
| `GET /admin/products/related-search?q=` | busca HTMX de produtos publicados para relacionar |
| `GET /admin/lixeira` | passa a listar também produtos e categorias |
| `GET /admin/marketplaces/{id}/edit` | ganha o campo de logotipo (envio, prévia e remover) |
| Dashboard `GET /admin` | novo cartão "Visitas às páginas" (total e únicos) |

Formulário do produto: grupos e itens da ficha se adicionam, removem e movem com botões acessíveis
por teclado; o formulário avisa antes de sair com alterações não salvas; ao trocar o slug de um
produto já divulgado, um aviso informa que o endereço antigo deixa de funcionar.
