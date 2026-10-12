# Feature Specification: Fase 2 — Páginas de produto (CMS)

**Feature Branch**: `002-paginas-de-produto`

**Created**: 2026-10-12

**Status**: Draft

**Input**: User description: "fase 2" (Fase 2 de `docs/referencias/escopo.md`: Páginas de produto, CMS)

## Clarifications

### Session 2026-10-12

- Q: Como a mídia (imagem do produto, galeria, logotipo de marketplace) é fornecida e armazenada? → A: Envio de arquivos pelo painel, guardados e servidos pelo próprio serviço em área persistente (opção A); sem armazenamento externo e sem URLs externas nas páginas públicas.
- Q: Ao abrir o código curto do produto, o navegador fica nesse endereço mostrando a página ou é levado ao endereço amigável? → A: A página é exibida no próprio endereço curto (com ou sem canal), com `/p/{endereço}` como endereço canônico; a abertura gera 1 clique e 1 visita, separados (opção A).
- Q: Quantos produtos publicados o catálogo deve comportar com bom desempenho nesta fase? → A: Até 1.000 produtos publicados (opção B).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Publicar uma página de produto pelo painel (Priority: P1)

O administrador cadastra um produto (título, descrição curta e longa, imagem, categoria opcional), vincula a ele
os links de afiliado que já existem, confere uma pré-visualização e publica. O produto passa a ter uma página
pública com endereço amigável e um código curto para divulgação.

**Why this priority**: sem produto publicado não existe página; é o núcleo que as demais histórias
enriquecem e medem.

**Independent Test**: criar um produto com título, imagem e um link de afiliado, publicá-lo e conferir, no
painel e pela interface programática, o endereço amigável e o código curto gerados; despublicar, excluir e
restaurar. A pré-visualização e a abertura do endereço público dependem da página pública (História 2).

**Acceptance Scenarios**:

1. **Given** um administrador autenticado com um link de afiliado ativo, **When** cadastra um produto, vincula o
   link e publica, **Then** o endereço público do produto responde com a página e o painel mostra o endereço
   amigável e o código curto para copiar.
   (A resposta do endereço público é verificada junto com a História 2.)
2. **Given** um produto em rascunho, **When** um visitante acessa seu endereço, **Then** recebe "não encontrado";
   **And** o administrador autenticado consegue ver a pré-visualização.
   (Verificado junto com a História 2, que entrega a página pública e a pré-visualização.)
3. **Given** um título sem hífen no endereço sugerido, **When** o administrador salva o produto, **Then** o
   endereço final contém ao menos um hífen e é único na organização; um endereço repetido é recusado com
   mensagem clara.
4. **Given** um produto publicado, **When** o administrador o exclui, **Then** ele vai para a lixeira, o endereço
   e o código curto passam a responder "não encontrado" e ele pode ser restaurado por 30 dias.

---

### User Story 2 - Visitante vê o produto e vai à loja (Priority: P1)

Quem chega por rede social ou mensageiro, quase sempre no celular, vê a página do produto: imagem, título,
descrição curta e, em destaque, o caminho para "Onde comprar", com os links de afiliado agrupados por
marketplace. Ao rolar, uma barra fixa mantém a ação principal à mão. Mais abaixo vêm a descrição longa, a ficha
técnica e os produtos relacionados.

**Why this priority**: é o propósito central do hub (levar o visitante à loja); sem essa página a fase não
entrega valor ao público.

**Independent Test**: com um produto publicado e dois links de lojas diferentes, abrir a página num celular
(375 px) e chegar a cada loja pelos botões. Entrega valor sozinho.

**Acceptance Scenarios**:

1. **Given** um produto publicado com links de duas lojas, **When** o visitante abre a página no celular,
   **Then** vê a ação principal sem rolar e, em "Onde comprar", um botão por link, agrupados por loja, sem
   preço.
2. **Given** um link com política `shorten`, **When** o visitante toca no botão, **Then** passa pelo
   redirecionamento rastreável do link (um clique registrado); **And** num link `direct` o botão leva à URL
   original e nenhuma métrica é registrada.
3. **Given** que o visitante rola a página, **When** a visão geral sai da tela, **Then** uma barra fixa com
   miniatura, título e botão leva à seção "Onde comprar" (no celular, como barra de ação no rodapé).
4. **Given** um link do produto inativo ou na lixeira, **When** a página é aberta, **Then** esse link não
   aparece; sem nenhum link disponível, a página informa que não há lojas no momento e não mostra o botão
   principal.
5. **Given** que o visitante usa só o teclado, **When** navega pela página, **Then** alcança todos os botões e
   controles com foco visível.

---

### User Story 3 - Montar a ficha técnica e os produtos relacionados (Priority: P2)

Produtos de categorias diferentes têm especificações muito diferentes. Para cada produto, o administrador
define seus próprios grupos de especificações (por exemplo, "Tela", "Bateria"), cada um com itens nome/valor,
e escolhe quais outros produtos aparecem como relacionados. Na página pública, os grupos viram um acordeão e os
relacionados, um carrossel sem preço.

**Why this priority**: enriquece a página e a torna útil para decidir a compra, mas a página já funciona sem
ficha técnica e sem relacionados.

**Independent Test**: criar três grupos com itens para um produto, reordená-los, escolher dois relacionados e
conferir a ordem e o comportamento do acordeão e do carrossel na página pública.

**Acceptance Scenarios**:

1. **Given** o formulário do produto, **When** o administrador adiciona, remove e reordena grupos e itens,
   **Then** a ordem salva é a exibida na página, sem limite fixo de grupos ou itens.
2. **Given** um produto com vários grupos, **When** o visitante abre a página, **Then** só o primeiro grupo
   está aberto, há um controle "Mostrar todos" e cada grupo tem âncora própria.
3. **Given** produtos relacionados escolhidos, **When** um deles é despublicado ou excluído, **Then** deixa de
   aparecer no carrossel sem afetar os demais.
4. **Given** o formulário, **When** o administrador tenta relacionar um produto a ele mesmo, **Then** a ação é
   recusada.

---

### User Story 4 - Medir visitas e cliques do produto (Priority: P2)

O administrador vê, para cada produto, quantas vezes a página foi aberta (visitas) e quantos acessos o link
curto do produto recebeu (cliques), em separado, com totais e únicos, por canal e por dia. Como a página fica em
cache, a visita é registrada pela própria página no navegador do visitante, sem cookie.

**Why this priority**: o hub existe para medir a divulgação; mas as páginas já cumprem sua função sem as
estatísticas.

**Independent Test**: abrir a página três vezes (dois visitantes distintos) e usar o link curto uma vez; o
painel mostra 3 visitas (2 únicas) e 1 clique, separados.

**Acceptance Scenarios**:

1. **Given** um produto publicado, **When** um visitante real abre a página, **Then** exatamente uma visita é
   registrada, sem cookie e sem bloquear a exibição da página.
2. **Given** a mesma pessoa abrindo a página duas vezes, **When** o painel é consultado, **Then** mostra 2
   visitas e 1 visitante único.
3. **Given** uma abertura pelo link curto do produto (com ou sem canal), **When** o painel é consultado, **Then** 1
   clique e 1 visita aparecem, separados, ambos com o canal correspondente.
4. **Given** um crawler conhecido ou um acesso classificado como robô, **When** abre a página, **Then** nenhuma
   visita entra nas estatísticas.
5. **Given** produtos com links `direct`, **When** as estatísticas são exibidas, **Then** esses links
   continuam "não rastreáveis" e nenhuma visita ou clique é atribuído a eles.

---

### User Story 5 - Compartilhar a página e ser encontrado (Priority: P2)

Quando o endereço do produto é colado numa rede social ou mensageiro, a prévia mostra título, imagem e
descrição do produto. Nos buscadores, a página tem metatags, endereço canônico e dados estruturados do
produto, sem preço.

**Why this priority**: a maior parte do tráfego vem de compartilhamento; uma prévia boa é o que faz o clique
acontecer.

**Independent Test**: colar o endereço amigável e o código curto de um produto num validador de prévia e numa
ferramenta de inspeção de metadados; ambos devem mostrar título, imagem e descrição, e o endereço canônico.

**Acceptance Scenarios**:

1. **Given** um produto publicado, **When** um crawler de rede social acessa o endereço, **Then** recebe título,
   imagem, descrição, URL e tipo, sem gerar visita.
2. **Given** a página de um produto, **When** inspecionada, **Then** tem um único título principal, hierarquia
   de títulos correta, endereço canônico sempre igual ao endereço amigável `/p/{endereço}`, inclusive quando a página é aberta pelo código curto, e dados estruturados do tipo produto sem
   informação de oferta ou preço.
3. **Given** que o administrador preencheu título e descrição para busca, **When** a página é inspecionada,
   **Then** as metatags usam esses valores; quando em branco, usam o título e a descrição curta do produto.

---

### User Story 6 - Organizar em categorias e identificar lojas com logotipo (Priority: P3)

O administrador cria categorias para agrupar produtos e define um logotipo para cada marketplace, que aparece na
seção "Onde comprar".

**Why this priority**: melhora a organização e o reconhecimento das lojas, mas a página funciona sem isso.

**Independent Test**: criar uma categoria, associá-la a um produto, definir o logotipo de um marketplace e
conferir ambos no painel e na página pública.

**Acceptance Scenarios**:

1. **Given** uma categoria, **When** o administrador a associa a produtos, **Then** a listagem de produtos pode
   ser filtrada por ela.
2. **Given** uma categoria com produtos, **When** o administrador tenta excluí-la, **Then** a exclusão é
   bloqueada e o painel indica quais produtos a impedem.
3. **Given** um marketplace com logotipo, **When** a página de produto é aberta, **Then** o logotipo aparece
   junto dos botões daquela loja; sem logotipo, aparece o nome.

---

### Edge Cases

- O mesmo conteúdo existe em dois endereços (o curto e o amigável): buscadores recebem sempre o endereço amigável
  como canônico; abrir `/p/{endereço}` gera uma visita sem canal e nenhum clique.
- Endereço amigável repetido, sem hífen, em branco ou com caracteres inválidos é recusado com mensagem clara.
- Alterar o endereço amigável de um produto já divulgado: o endereço antigo deixa de funcionar; o código curto e
  os links por canal continuam válidos. O painel avisa isso antes de salvar.
- Produto sem imagem: a página usa uma imagem padrão, com texto alternativo, sem quebrar o layout.
- Arquivo de imagem de formato não aceito ou grande demais é recusado com mensagem; o produto não é salvo pela
  metade.
- Descrição longa com marcação HTML ou scripts: é exibida como texto, nunca interpretada.
- Produto publicado sem nenhum link ativo: a página abre, informa que não há lojas no momento e não exibe botão
  de compra.
- Um link de afiliado só pode pertencer a um produto; vincular um link já vinculado a outro produto exige
  desvinculá-lo antes, com mensagem indicando qual produto o possui.
- Marketplace na lixeira: seus links somem de "Onde comprar", como links na lixeira.
- Produto relacionado despublicado, excluído ou sem imagem não quebra o carrossel.
- Edição de um produto publicado: a próxima abertura da página já mostra a versão nova; despublicar ou excluir
  vale imediatamente para quem abrir a página depois.
- Visitante sem JavaScript ou com bloqueador: a página e os botões funcionam; a visita pode não ser registrada
  (limitação aceita e documentada).
- Recarregar a página várias vezes: cada abertura é uma visita; a contagem de únicos não cresce.
- Uso abusivo (muitas aberturas ou registros de visita do mesmo IP): limitado como as demais rotas públicas.
- Grupo de especificações sem itens, ou item com valor muito longo: o formulário recusa o salvamento com mensagem
  indicando o grupo ou item; na página pública, grupos sem itens nunca aparecem.

## Requirements *(mandatory)*

### Functional Requirements

**Produtos no painel**

- **FR-001**: O administrador MUST poder criar, listar (com busca, filtro e paginação), editar, publicar/despublicar e
  excluir produtos, cada um com título, endereço amigável, categoria opcional, descrição curta, descrição longa em
  texto simples, imagem principal, galeria opcional e título e descrição para busca opcionais.
- **FR-002**: O endereço amigável MUST ser único na organização, conter apenas letras minúsculas, números e hífens,
  conter ao menos um hífen (para nunca colidir com um código curto) e ser sugerido a partir do título.
- **FR-003**: Produtos MUST usar a mesma exclusão lógica e a mesma lixeira de 30 dias (configurável) dos demais
  itens; enquanto estiver na lixeira ou após a exclusão definitiva, o endereço amigável, o código curto e as URLs por
  canal do produto MUST responder "não encontrado" e não gerar evento.
- **FR-004**: Todo produto MUST ter um código curto próprio (sete caracteres `[0-9a-z]`, único globalmente,
  compartilhando o espaço de códigos dos links de afiliado), gerado automaticamente.
- **FR-005**: O administrador MUST poder vincular e desvincular links de afiliado a um produto; um link MUST
  pertencer a no máximo um produto.
- **FR-006**: O administrador MUST poder definir, por produto, grupos de especificações (nome e posição) com itens
  nome/valor em texto simples (posição incluída), podendo adicionar, remover e reordenar grupos e itens, sem tipos,
  unidades nem comparação numérica.
- **FR-007**: O administrador MUST poder escolher e ordenar produtos relacionados; um produto MUST NOT ser
  relacionado a si mesmo.
- **FR-008**: O administrador MUST poder criar, listar, editar e excluir categorias (nome e endereço), associar cada
  produto a no máximo uma categoria e filtrar a listagem por categoria; uma categoria com produtos fora da lixeira
  MUST NOT poder ser excluída, e o painel MUST indicar quais produtos a impedem.
- **FR-009**: O administrador MUST poder definir um logotipo por marketplace.
- **FR-010**: O administrador MUST poder pré-visualizar um produto em rascunho; visitantes MUST NOT ter acesso a ele.
- **FR-011**: O administrador MUST poder enviar imagens pelo painel (imagem principal e galeria do produto,
  logotipo de marketplace). O sistema MUST aceitar somente arquivos de imagem de formatos e tamanho permitidos,
  guardá-los em área persistente do próprio serviço (incluída no backup e na documentação de operação) e servi-los
  ele mesmo, em tamanhos adequados a cada tela, de modo que as páginas públicas não dependam de recursos de
  terceiros. Imagens externas por URL MUST NOT ser usadas nas páginas públicas de produto.

**Página pública do produto**

- **FR-012**: O sistema MUST servir a página de um produto publicado no endereço amigável sob um prefixo
  reservado (`/p/{endereço}`) e também nos endereços do código curto (FR-025), renderizada no servidor, sem cookies e sem recursos de terceiros; produto em
  rascunho, na lixeira ou inexistente MUST responder "não encontrado".
- **FR-013**: A página MUST conter, nesta ordem: visão geral (imagem, título, descrição curta e botão principal "Ver
  onde comprar"), "Onde comprar", descrição longa, ficha técnica em acordeão e produtos relacionados em carrossel;
  seções sem conteúdo MUST ser omitidas.
- **FR-014**: "Onde comprar" MUST listar os links ativos do produto agrupados por marketplace, com logotipo (ou nome)
  e um botão por link, sem preço, sem ordenação por preço, sem selo de melhor preço e sem abas novo/usado. Links
  inativos ou na lixeira, e links de marketplaces na lixeira, MUST NOT aparecer.
- **FR-015**: Cada botão MUST respeitar a política efetiva do link: `shorten` leva pelo redirecionamento rastreável
  do link (propagando o canal, quando a página foi aberta por uma URL com canal); `direct` leva à URL original,
  sem qualquer métrica.
- **FR-016**: Ao rolar a página, uma barra fixa com miniatura, título e botão que leva a "Onde comprar" MUST ficar
  visível; no celular, ela MUST ser uma barra de ação no rodapé.
- **FR-017**: A ficha técnica MUST ser um acordeão com um item por grupo, cada um com âncora própria, com somente o
  primeiro aberto por padrão e um controle "Mostrar todos", operável por teclado.
- **FR-018**: A página MUST ser utilizável em celular (mobile first), com a ação principal visível sem rolar, e
  respeitar os modos claro e escuro, contraste, foco visível e texto alternativo em imagens.
- **FR-019**: A página MUST exibir o aviso informativo de privacidade (dispensável e lembrado sem cookie) com link
  para a página de privacidade, e essa página MUST passar a descrever também o registro de visitas.
- **FR-020**: A página MUST cumprir o orçamento de desempenho: imagens responsivas e em formatos modernos, carga
  adiada para imagens abaixo da dobra, dimensões declaradas, CSS e JavaScript mínimos.
- **FR-021**: O sistema MUST manter as páginas publicadas em cache com validadores HTTP; salvar, despublicar,
  restaurar ou excluir um produto, e alterar um item exibido na página (link, marketplace, relacionado, categoria,
  logotipo), MUST refletir na próxima abertura da página.
- **FR-022**: As rotas públicas de produto e o registro de visitas MUST ser limitados por IP como as demais rotas
  públicas, sem gerar evento nas requisições recusadas.

**Visitas, cliques e estatísticas**

- **FR-023**: Cada abertura da página por um visitante real MUST registrar uma visita, feita pelo navegador após o
  carregamento, sem cookie, de forma assíncrona e em lotes, sem atrasar nem impedir a exibição; falha do registro
  MUST ser silenciosa e descartes MUST ser contabilizados em métrica.
- **FR-024**: A visita MUST usar o mesmo identificador anônimo de visitante dos cliques (sem IP puro em banco ou
  logs), registrar o canal quando houver, e ignorar crawlers conhecidos e acessos classificados como robôs,
  que MUST NOT entrar nas estatísticas.
- **FR-025**: Acessar o código curto do produto (`/{código}` ou `/{canal}/{código}`) MUST exibir a página do produto
  no próprio endereço, sem redirecionar, e registrar um clique do link divulgado; a visita da página MUST ser
  registrada em seguida pelo navegador, com o canal da URL. Uma mesma abertura MUST resultar em exatamente 1
  clique e 1 visita, exibidos separadamente.
- **FR-026**: Cliques e visitas do produto MUST ter unicidade permanente por produto e por produto/canal,
  calculada corretamente sob concorrência, preservando a distinção entre alvo e URL.
- **FR-027**: O painel MUST exibir, por produto, visitas e cliques (totais e únicos) em separado, por canal e por
  dia, com filtro de período, além de visitas e cliques por produto na listagem; o detalhe MUST manter a indicação
  "não rastreável" dos links `direct`, sem atribuir a eles visitas ou cliques.
- **FR-028**: Eventos de visita MUST seguir a consolidação diária idempotente, a retenção por camadas e a limpeza
  já aplicadas aos cliques (somente dias consolidados são limpos; contagens preservadas).
- **FR-029**: A interface programática autenticada MUST expor as estatísticas de produto e permitir gerenciar
  produtos, categorias e a ficha técnica com o mesmo comportamento do painel.

**Compartilhamento e SEO**

- **FR-030**: A página MUST ter um único título principal, hierarquia de títulos correta, metatags e de
  compartilhamento (título, imagem, descrição, URL e tipo), endereço canônico sempre igual ao endereço amigável `/p/{endereço}`, inclusive quando a página é aberta pelo código curto, e dados
  estruturados do tipo produto sem informação de oferta ou preço.
- **FR-031**: Crawlers conhecidos MUST receber a página (ou prévia equivalente) com os metadados de
  compartilhamento, sem gerar visita nem clique.
- **FR-032**: Quando título e descrição para busca estiverem em branco, as metatags MUST usar o título e a
  descrição curta do produto.

### Key Entities

- **Produto**: item divulgado, com título, endereço amigável, descrições, imagens, categoria opcional, estado
  (rascunho/publicado/lixeira) e código curto; agrupa links de afiliado, grupos de especificações e relacionados.
- **Categoria**: agrupador de produtos, com nome e endereço; cada produto tem no máximo uma.
- **Imagem (mídia)**: arquivo de imagem associado a um produto (principal e galeria) ou a um marketplace
  (logotipo).
- **Grupo de especificações**: seção da ficha técnica de um produto, com nome e posição.
- **Item de especificação**: par nome/valor em texto simples dentro de um grupo, com posição.
- **Produto relacionado**: relação ordenada entre um produto e outro produto sugerido.
- **Link de afiliado** (existente): passa a poder pertencer a um produto; mantém sua política e seu código.
- **Marketplace** (existente): ganha logotipo.
- **Evento de visita**: abertura da página de um produto por um visitante real, distinto do clique, com canal e
  identificador anônimo.
- **Código curto** (existente): passa a poder identificar um produto, além de um link de afiliado.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: O administrador publica um produto básico (título, imagem, descrição curta e um link vinculado) e
  copia o endereço e o código curto em menos de 5 minutos, sem usar a interface programática.
- **SC-002**: Em celular de desempenho médio e rede móvel comum, a página de produto atinge, para 75% das
  aberturas, maior elemento visível em até 2,5 s, deslocamento de layout abaixo de 0,1 e resposta à interação
  abaixo de 200 ms.
- **SC-003**: Em uma tela de celular de 375 px de largura, a ação principal "Ver onde comprar" fica visível sem
  rolar em 100% dos produtos publicados.
- **SC-004**: Com um catálogo de até 1.000 produtos publicados, a página responde em menos de 100 ms para 95% dos
  acessos, medido no servidor.
- **SC-005**: Em teste controlado com 100 aberturas por navegador real, 100 geram uma visita registrada e
  nenhuma abertura por crawler conhecido gera visita.
- **SC-006**: Visitas e cliques aparecem em separado em 100% das telas e respostas de estatísticas de produto, e
  o mesmo visitante abrindo a página duas vezes resulta em 2 visitas e 1 único.
- **SC-007**: Depois de salvar, despublicar ou excluir um produto, 100% das aberturas seguintes mostram o novo
  estado (nenhuma página desatualizada servida).
- **SC-008**: Nenhuma resposta das páginas públicas de produto contém cookie ou carrega recurso de terceiros, e
  nenhum IP puro é encontrado em dados persistidos ou logs.
- **SC-009**: Links `direct` aparecem como "não rastreável" em 100% das telas do produto e nenhuma visita ou
  clique é atribuído a eles.
- **SC-010**: O cadastro e a consulta de produtos funcionam em tela de celular, nos dois temas e só com o
  teclado.
- **SC-011**: Com um catálogo de até 1.000 produtos publicados e sob 200 aberturas de página por segundo durante 60 s, não há erros nas páginas e a perda de
  visitas por descarte permanece abaixo de 0,1%.

## Assumptions

- O catálogo dimensionado para esta fase é de até 1.000 produtos publicados; além disso, o desempenho não é
  garantido e o dimensionamento é revisto.
- **[ASSUNÇÃO]** O espaço de armazenamento das imagens cresce com o catálogo; limites de tamanho por arquivo e
  formatos aceitos são definidos no planejamento e documentados em `docs/operacao.md`.
- **[ASSUNÇÃO]** A fase 2 estende a organização padrão e o administrador único da fase 1; papéis e
  multiusuário continuam na fase 4.
- **[ASSUNÇÃO]** O endereço amigável público é `/p/{endereço}` (prefixo `p` já reservado no escopo); o endereço
  é sugerido a partir do título e sempre contém hífen.
- **[ASSUNÇÃO]** Categorias são uma lista simples, sem hierarquia; cada produto tem no máximo uma. Navegação por
  categorias no site público fica para a fase 3.
- **[ASSUNÇÃO]** A descrição longa e os valores da ficha são texto simples, com parágrafos por quebra de linha;
  não há editor de texto rico.
- **[ASSUNÇÃO]** Produtos têm dois estados visíveis ao administrador (rascunho e publicado), além da lixeira,
  com as mesmas regras de 30 dias dos demais itens.
- **[ASSUNÇÃO]** Os produtos relacionados são escolhidos manualmente; não há sugestão automática.
- **[ASSUNÇÃO]** A visita é a abertura da página; ela não é atribuída aos links `direct` do produto. Seguindo a
  decisão do responsável de que esses links não têm métrica, a visita mede a página, não o link.
- **[ASSUNÇÃO]** `sitemap.xml`, `robots.txt`, home, busca e navegação por categorias pertencem à fase 3; a fase 2
  só garante metatags, endereço canônico e dados estruturados por página.
- **[ASSUNÇÃO]** O endereço amigável antigo de um produto não é redirecionado depois de alterado; o código curto
  é o endereço estável para divulgação.
- **Fora de escopo (decidido)**: preços, melhor preço, alerta de preço, ranking, pontuação, radar, veredito,
  avaliações de usuários, comparador entre produtos, barras comparativas, prós e contras, modelos de grupos por
  categoria e integrações com APIs de marketplaces.
- Depende da fase 1 concluída: links de afiliado, marketplaces, canais, redirecionamento, fila de eventos,
  identificação anônima do visitante, estatísticas, consolidação, lixeira, aviso e página de privacidade.
