# Guia de estilo do painel e das páginas públicas

Diretrizes da seção 11.1 do escopo, convertidas em regras de uso. O guia vivo, com todos os componentes
renderizados nos dois temas, está em `/admin/ui` (somente com `APP_ENV=development`).

## Princípios

- **Mobile first.** O cadastro de links costuma acontecer no celular; a largura de referência é 375 px, sem
  rolagem horizontal.
- **Acessível.** Contraste WCAG AA, foco visível, navegação por teclado, rótulo em todo campo, `alt` em toda
  imagem, `lang="pt-BR"`.
- **Sem recursos de terceiros nas páginas públicas.** Fontes, ícones, scripts e estilos são locais.
- **Interface em português do Brasil.** Mensagens de erro dizem o que fazer, não só o que falhou.
- **HTMX para o servidor, Alpine só para o que é local** (menu, modal, copiar, avisos). Sem SPA.

## Tokens

Definidos em um só lugar, `web/static/css/base.css`, e reaproveitados pelo Tailwind (`web/assets/app.css`).
O modo escuro existe em dois blocos que não podem divergir (`.dark` e o fallback de `prefers-color-scheme`);
`web/design_test.go` verifica a igualdade e o contraste.

| Token | Claro | Escuro | Uso |
|---|---|---|---|
| `--bg` | `#f6f7f9` | `#0e1420` | fundo da página |
| `--surface` | `#ffffff` | `#161e2e` | cartões, tabelas, campos |
| `--ink` | `#172033` | `#eef2f8` | texto principal |
| `--muted` | `#5b6475` | `#a3adbf` | texto secundário e rótulos |
| `--line` | `#d9dde5` | `#2c3850` | divisórias decorativas |
| `--line-strong` | `#7c8598` | `#66769a` | contorno de campos e trilho do interruptor (≥ 3:1) |
| `--brand` / `--brand-ink` | `#2563eb` / `#ffffff` | `#6ea0ff` / `#0b1220` | ação principal e links |
| `--danger` | `#b42318` | `#ff8a80` | erro e ação destrutiva |
| `--ok` | `#067647` | `#5fd39a` | sucesso e estado ativo |
| `--warn-bg` | `#fff4d6` | `#3a3012` | fundo de avisos |
| `--focus` | `#1d4ed8` | `#9bbcff` | anel de foco (≥ 3:1) |

Regras de contraste (verificadas em teste): texto em 4,5:1 ou mais; foco e contornos de componentes em 3:1 ou
mais. Não use `--line` para delimitar um campo; ela é só decorativa.

Espaçamento `--space-1` a `--space-8` (0,25 a 2 rem), raios `--radius` (cartões), `--radius-sm` (campos e
botões) e `--radius-pill` (selos), sombra `--shadow` (nula no escuro). Fonte Inter hospedada localmente, com
`font-display: swap`. Quem prefere menos movimento (`prefers-reduced-motion`) não vê transições.

## Tema

`theme_init` define `.dark` ou `.light` no `<html>` antes da primeira pintura (sem flash), a partir da escolha
salva em `localStorage` ou da preferência do sistema. O botão de tema alterna e lembra a escolha; os gráficos
são redesenhados porque leem as cores das variáveis CSS.

## Componentes

Cada componente é um `{{define}}` em `web/templates/components/` e recebe um `dict`. Use sempre o componente em
vez de repetir o HTML.

| Componente | Parâmetros | Quando usar |
|---|---|---|
| `button` | `Label`, `Type`, `Variant` (`primary`, `danger`, `ghost`), `Icon`, `Small`, `Disabled`, `AriaLabel` | Uma ação por vez. Um botão primário por tela. Sem `Label`, `AriaLabel` é obrigatório. |
| `field` | `ID`, `Name`, `Label`, `Type`, `Value`, `Required`, `Error`, `Hint`, `Pattern`, `MaxLength`, `Autocomplete`, `Autofocus` | Todo campo de texto. Liga rótulo, ajuda e erro (`aria-describedby`, `aria-invalid`). |
| `table` | `Caption`, `Columns`, `Rows`, `EmptyText` | Dados simples em texto. Listagens com ações usam o HTML próprio da tela, com `table-cards`. |
| `card` | `Title`, `Text`, `Href`, `Action` | Atalho ou bloco de conteúdo com uma ação. |
| `stat` | `Title`, `Value`, `Hint` | Indicador numérico (cliques, únicos, links ativos). Formate o valor com `num`. |
| `badge` | `Text`, `Tone` (`ok`, `off`, `warn`, `brand`, `inactive`) | Estado curto. Nunca só pela cor: o texto diz o estado. |
| `policy_badge` | política (`shorten` ou `direct`) | Política do link; `direct` mostra “não rastreável”. |
| `modal` | (casca única) | Formulários curtos e confirmações. Conteúdo chega por HTMX em `#modal-body`; é um `<dialog>` nativo. |
| `toast` / `toast_region` | evento `toast` com `{message, kind}` | Retorno de ações (`HX-Trigger`). Região viva, some em 4 s. |
| `tabs` | `Label`, `Items` | Seções de uma mesma entidade. Links comuns, sem JavaScript. |
| `period_picker` | `Items`, `Target` | Período (7, 30 e 90 dias). Com `Target`, o HTMX troca só esse contêiner e atualiza a URL. |
| `period_custom` | `Period`, `Action` | Intervalo personalizado (De/Até), ao lado do `period_picker`. |
| `chart` | `Label`, `Data`, `Rows` | Série diária (uPlot). Sempre acompanha a tabela alternativa em “Ver dados em tabela”. |
| `pager` | `Page`, `Pages`, `Total`, `PrevURL`, `NextURL`, `Target` | Paginação de listagens via HTMX. |
| `empty_state` | `Text`, `Icon`, `ActionHref`, `ActionLabel` | Toda listagem vazia: diga o próximo passo, não deixe tela em branco. |
| `skeleton` | — | Espaço reservado enquanto o HTMX carrega. |
| `privacy_notice` | — | Aviso informativo das páginas públicas (dispensável, sem cookie, sem pedido de consentimento). |
| `theme_init` | — | Resolve o tema antes da folha de estilo. |

Ícones são SVG inline (Lucide) pela função `icon "nome"`; são decorativos, então o botão ou o link que os
contém precisa de texto ou `aria-label`.

## Tabelas no celular

Em telas com até 40 rem, as tabelas com a classe `table-cards` viram cartões: o cabeçalho fica só para leitores
de tela, cada célula mostra seu rótulo (`data-label`) e as colunas secundárias (`hide-sm`) somem. A primeira
célula (o nome do item) não leva `data-label` e funciona como título do cartão. Células de ação usam
`class="actions"`.

## Padrão CRUD

1. **Listagem** em tabela, com busca, filtros e paginação via HTMX (`hx-get`, `hx-push-url`), botão “Novo” no
   topo e estado vazio com orientação.
2. **Formulário** de criação e edição em página própria (links) ou em modal (entidades simples: marketplaces e
   canais), com validação inline (`422` devolve o formulário com os erros).
3. **Detalhe** quando há estatísticas (links): URLs com botão “Copiar”, seletor de período, gráfico e tabela
   por canal.
4. **Exclusão** com confirmação em modal e retorno por toast; os itens vão para a lixeira e podem ser
   restaurados dentro do prazo.

Formulários de página inteira usam `data-unsaved="false"` e carregam `unsaved.js`: ao editar, o navegador avisa
antes de fechar a aba ou de navegar para fora; salvar o formulário limpa o aviso.

## Links não rastreáveis

Links com a política `direct` mostram a URL original e o selo “Não rastreável”. Nunca exibem zero cliques; se
houve contagem antes da mudança de política, ela aparece como histórico.

## Páginas públicas

Layout `public`, sem HTMX nem Alpine. Prévia de compartilhamento e página de privacidade carregam só o CSS do
site, mostram o aviso de privacidade e não emitem cookies. A ação principal fica visível sem rolar, a imagem tem
largura e altura declaradas e o texto é escapado pelo `html/template`.

## Como validar uma tela nova

1. Abra `/admin/ui` e reutilize os componentes existentes antes de criar HTML novo.
2. Confira a 375 px, nos dois temas, navegando só com Tab: o foco deve estar sempre visível.
3. Rode `go test ./web ./internal/admin`: os testes cobrem contraste dos tokens, rótulos, `alt`, `lang`, botão
   de tema e estados vazios.
