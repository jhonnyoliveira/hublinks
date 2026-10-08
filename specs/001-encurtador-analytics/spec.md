# Feature Specification: Fase 1 — Encurtador de links com analytics (MVP)

**Feature Branch**: `001-encurtador-analytics`

**Created**: 2026-10-08

**Status**: Draft

**Input**: User description: "fase 01" (Fase 1 de `docs/referencias/escopo.md`: Encurtador com analytics)

## Clarifications

### Session 2026-10-08

- Q: Qual fuso horário define o "dia" usado na consolidação diária, nos gráficos de série diária e no seletor de período? → A: Fuso configurável por variável de ambiente, padrão `America/Sao_Paulo`, fixo depois que existirem dados consolidados; timestamps continuam gravados em UTC.
- Q: Quando a política efetiva de um link é `direct`, ele deve ter código curto, e o que acontece se alguém acessar esse código? → A: Todo link recebe código; acesso ao código de link `direct` redireciona sem registrar evento; o painel exibe só a URL original e "não rastreável", preservando o histórico anterior.
- Q: A partir de quantos códigos distintos, acessados pelo mesmo IP em quantos segundos, um acesso passa a ser marcado como robô? → A: 5 ou mais códigos distintos em 10 segundos, configurável; o acesso que atinge o limite e os seguintes dentro da janela são marcados como robô.
- Q: Quais limites de proteção contra abuso valem para as tentativas de login e para as rotas públicas de redirecionamento? → A: Login: 5 falhas em 15 min, por IP e por e-mail, bloqueiam por 15 min; rotas públicas: 300 requisições/min por IP, excedente recebe "muitas requisições" sem gerar evento; valores configuráveis.
- Q: Depois que um link, canal ou marketplace é excluído (exclusão lógica), o que acontece com as URLs já divulgadas, com o código curto e com as estatísticas antigas? → A: Lixeira de 30 dias com restauração; durante e após esse prazo o código e as URLs respondem "não encontrado"; após 30 dias a exclusão vira definitiva e os registros de unicidade são apagados; código nunca reutilizado; contagens consolidadas permanecem nos totais; marketplace ou canal com links ativos só pode ser excluído após remover ou mover esses links.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Cadastrar link e divulgar por canal (Priority: P1)

O administrador entra no painel, cadastra um marketplace, um canal (ex.: WhatsApp) e um link de afiliado
(título, imagem opcional e URL original). O sistema gera um código curto e mostra as URLs prontas para
copiar: uma curta e uma por canal. Quem acessa uma dessas URLs é levado ao destino.

**Why this priority**: sem cadastro e redirecionamento não existe produto; é o núcleo que as demais
histórias medem.

**Independent Test**: cadastrar marketplace, canal e link pelo painel, copiar a URL por canal, acessá-la
como visitante comum e verificar que chega ao destino cadastrado.

**Acceptance Scenarios**:

1. **Given** um marketplace e um link cadastrados, **When** o administrador abre o detalhe do link,
   **Then** vê a URL curta e uma URL por canal, cada uma com botão de copiar.
2. **Given** um link ativo, **When** um visitante acessa a URL curta ou a URL por canal, **Then** é
   redirecionado temporariamente (não permanente) ao destino original.
3. **Given** um código inexistente ou link inativo, **When** alguém o acessa, **Then** recebe "não encontrado".
4. **Given** um canal inexistente na URL, **When** alguém a acessa, **Then** recebe "não encontrado".
5. **Given** o formulário de link, **When** o administrador informa uma URL que não seja http/https,
   **Then** o cadastro é recusado com mensagem clara.

---

### User Story 2 - Ver cliques totais e únicos (Priority: P1)

O administrador consulta, no painel, cliques totais e únicos por link, por canal e por dia, com seletor de
período (7, 30, 90 dias ou personalizado), para responder "minha divulgação está fazendo sentido?".

**Why this priority**: é a razão de existir do produto.

**Independent Test**: acessar o mesmo link várias vezes com diferentes visitantes e canais e conferir os
totais e únicos exibidos no painel.

**Acceptance Scenarios**:

1. **Given** dois acessos do mesmo visitante (mesmo IP e user-agent) ao mesmo link, **When** o
   administrador consulta as estatísticas, **Then** vê 2 cliques e 1 único.
2. **Given** o mesmo visitante com outro user-agent, **When** acessa o mesmo link, **Then** conta como novo único.
3. **Given** o mesmo visitante acessando o link por dois canais, **When** consulta o alcance do alvo,
   **Then** conta 1 único no alcance do alvo e 1 único em cada canal.
4. **Given** um período selecionado, **When** o administrador troca o período, **Then** todas as métricas
   da tela refletem o novo período.
5. **Given** um link de marketplace com política "direct", **When** aparece em listas e detalhes,
   **Then** exibe a URL original e a marca "não rastreável", nunca zero cliques.

---

### User Story 3 - Prévia de compartilhamento sem distorcer métricas (Priority: P2)

Quando o link é colado em mensageiros ou redes sociais, o robô de prévia recebe uma página com título e
imagem do link, sem ser redirecionado e sem alterar contadores.

**Why this priority**: prévia boa aumenta cliques; contar robôs falsearia os números.

**Independent Test**: acessar a URL com identificação de um crawler conhecido e verificar a prévia e a
ausência de qualquer aumento nos contadores.

**Acceptance Scenarios**:

1. **Given** um crawler conhecido, **When** acessa a URL do link, **Then** recebe página com título,
   imagem e descrição do link, sem redirecionamento.
2. **Given** essa prévia, **When** o administrador vê as estatísticas, **Then** os contadores não mudaram.
3. **Given** um mesmo IP que já acessou 4 códigos distintos nos últimos 10 segundos, **When** acessa
   um 5º código distinto, **Then** esse acesso é redirecionado normalmente, mas marcado como robô e
   não entra nas métricas; os 4 anteriores continuam contando.
4. **Given** a página de prévia, **When** exibida, **Then** mostra o aviso informativo de privacidade com link para `/privacidade`.

---

### User Story 4 - Operar no celular com modo claro e escuro (Priority: P2)

O administrador cadastra e consulta links pelo celular, alternando entre modo claro e escuro. A interface
é em português do Brasil, acessível e consistente entre telas (listagem, formulário, detalhe, exclusão
com confirmação, estados vazios com orientação).

**Why this priority**: o cadastro costuma ocorrer fora do computador.

**Independent Test**: percorrer o fluxo de cadastro e consulta em tela de celular e alternar o tema.

**Acceptance Scenarios**:

1. **Given** uma tela de celular, **When** o administrador usa listagens e formulários, **Then** tudo é legível e operável sem rolagem horizontal da página.
2. **Given** a preferência do sistema ou a escolha manual, **When** o administrador alterna o tema, **Then** a escolha é lembrada.
3. **Given** nenhuma entidade cadastrada, **When** abre uma listagem, **Then** vê orientação do próximo passo.

---

### User Story 5 - Acesso restrito e política de privacidade (Priority: P2)

Somente o administrador autenticado acessa o painel. O administrador inicial é criado na primeira
execução a partir de configuração do ambiente, se não houver usuários. Qualquer pessoa lê a página
pública de privacidade, que descreve a identificação anônima, a finalidade, os prazos de retenção e como
solicitar exclusão.

**Why this priority**: protege os dados e atende à transparência exigida.

**Independent Test**: acessar o painel sem sessão, autenticar e abrir a página pública de privacidade.

**Acceptance Scenarios**:

1. **Given** um visitante sem sessão, **When** tenta abrir qualquer tela do painel, **Then** é levado ao login.
2. **Given** 5 tentativas de login com senha errada em 15 minutos, **When** ocorre a 6ª tentativa,
   mesmo com a senha correta, **Then** ela é recusada até que se passem 15 minutos.
3. **Given** uma instalação sem usuários, **When** a aplicação inicia com credenciais iniciais configuradas, **Then** o administrador é criado; com usuários existentes, nada é criado.
4. **Given** qualquer resposta ao visitante (redirecionamento, prévia), **When** inspecionada, **Then** não define cookies.

---

### User Story 6 - Histórico preservado com retenção automática (Priority: P3)

Diariamente, o sistema consolida as contagens do dia anterior e remove detalhes antigos além do prazo de
retenção, sem alterar as contagens totais e únicas de períodos passados.

**Why this priority**: controla o crescimento dos dados e a exposição de privacidade sem perder histórico.

**Independent Test**: consolidar e limpar dados antigos e comparar as contagens antes e depois; repetir a
consolidação e verificar que nada duplica.

**Acceptance Scenarios**:

1. **Given** eventos de dias já consolidados além do prazo, **When** a limpeza roda, **Then** são removidos e as contagens do período permanecem idênticas.
2. **Given** um dia já consolidado, **When** a consolidação roda novamente, **Then** os valores não mudam.
3. **Given** eventos de dias ainda não consolidados, **When** a limpeza roda, **Then** não são removidos.
4. **Given** eventos de robôs, **When** passam de 30 dias, **Then** são removidos; eventos reais, após 395 dias.
5. **Given** um link excluído há menos de 30 dias, **When** o administrador o restaura, **Then** ele volta
   a redirecionar e um visitante que já o acessara antes não conta como novo único.
6. **Given** um link na lixeira há mais de 30 dias, **When** a limpeza roda, **Then** a exclusão se torna
   definitiva, os registros de unicidade daquele alvo são apagados e as contagens consolidadas permanecem nos totais.

---

### User Story 7 - Subir o serviço com um comando (Priority: P3)

O responsável sobe aplicação e banco com um único comando a partir de variáveis de ambiente, e consegue
verificar a saúde do serviço.

**Why this priority**: viabiliza a entrega e a operação.

**Independent Test**: configurar o ambiente de exemplo, subir o conjunto e consultar o endpoint de saúde.

**Acceptance Scenarios**:

1. **Given** variáveis de ambiente válidas, **When** o responsável sobe o conjunto, **Then** painel e redirecionamento ficam disponíveis e a verificação de saúde responde com sucesso.
2. **Given** configuração obrigatória ausente, **When** a aplicação inicia, **Then** falha com mensagem indicando o que falta.

---

### Edge Cases

- Fila de gravação cheia ou falha na gravação: o visitante ainda é redirecionado; o descarte é contabilizado e exposto como métrica.
- Gravação de eventos indisponível: o redirecionamento continua funcionando.
- Dois acessos simultâneos do mesmo visitante ao mesmo alvo: apenas um é contado como único.
- Segmento de canal igual a palavra reservada ou já existente na organização: recusado.
- Exclusão de marketplace com links fora da lixeira: recusada, indicando os links que impedem.
- Exclusão de canal: permitida após confirmação que informa quantos links ativos terão a URL daquele canal invalidada.
- Acesso a URL divulgada de link na lixeira ou excluído definitivamente: "não encontrado", sem evento.
- Tentativa de criar canal com segmento de canal na lixeira: recusada enquanto ele puder ser restaurado.
- IP vindo de cabeçalho de proxy não confiável: ignorado, usa-se o endereço da conexão.
- Visitante com IPv6: dispositivos da mesma rede /64 são tratados como o mesmo endereço.
- Mudança de IP (rede móvel) ou IP compartilhado: contagem de únicos é aproximada; limitação visível no painel.
- Política "direct" no marketplace com sobrescrita no link: vale a do link.
- Política muda de `shorten` para `direct` após divulgação: URLs já divulgadas continuam redirecionando, sem novos eventos; o histórico anterior permanece. Na mudança inversa, a contagem recomeça a partir dali.
- Período personalizado inválido (fim antes do início): recusado com mensagem.
- Acesso perto da meia-noite: é atribuído ao dia do fuso de relatório (ex.: 23h30 em Brasília conta no próprio dia, não no dia seguinte em UTC).

## Requirements *(mandatory)*

### Functional Requirements

**Cadastros e acesso**
- **FR-001**: O sistema MUST autenticar um único administrador por e-mail e senha, com sessão protegida contra requisições forjadas. Após 5 falhas de login em 15 minutos, contadas por IP e por e-mail, novas tentativas MUST ser bloqueadas por 15 minutos (valores configuráveis), sem revelar se o e-mail existe.
- **FR-002**: O sistema MUST criar o administrador inicial na primeira execução a partir da configuração do ambiente, somente se não existirem usuários.
- **FR-003**: O administrador MUST poder criar, listar (com busca, filtro e paginação), editar e excluir marketplaces, definindo a política de encurtamento (`shorten` ou `direct`).
- **FR-004**: O administrador MUST poder criar, listar, editar e excluir canais, cada um com um segmento de URL único na organização e fora das palavras reservadas.
- **FR-005**: O administrador MUST poder criar, listar, editar, ativar/desativar e excluir links de afiliado com título, imagem opcional, URL original, marketplace e política opcional que sobrescreve a do marketplace.
- **FR-006**: O sistema MUST aceitar como destino somente URLs `http` e `https`.
- **FR-007**: Ao criar qualquer link (inclusive de política `direct`), o sistema MUST gerar automaticamente um código curto de exatamente sete caracteres `[0-9a-z]`, único globalmente.
- **FR-008**: Todas as entidades principais MUST usar exclusão lógica; exclusões MUST pedir confirmação no painel.
- **FR-008a**: Links, canais e marketplaces excluídos MUST ir para uma lixeira, de onde o administrador pode restaurá-los por 30 dias (configurável). Após o prazo, a exclusão MUST se tornar definitiva e irreversível pelo painel.
- **FR-008b**: Enquanto um link estiver na lixeira ou após a exclusão definitiva, seu código e suas URLs por canal MUST responder "não encontrado" e não gerar evento; o mesmo vale para URLs com o segmento de um canal excluído. Um código curto MUST NOT ser reutilizado, mesmo após a exclusão definitiva.
- **FR-008c**: Um marketplace que possua links fora da lixeira MUST NOT poder ser excluído; o painel MUST indicar quais links impedem a exclusão. A confirmação de exclusão de um canal MUST informar quantos links ativos terão suas URLs daquele canal invalidadas.
- **FR-008d**: O segmento de um canal na lixeira MUST permanecer reservado, para permitir a restauração sem conflito.
- **FR-009**: Toda operação de domínio MUST ser restrita à organização do contexto; existe uma organização padrão criada na inicialização.

**Redirecionamento**
- **FR-010**: O sistema MUST responder `/{código}` e `/{canal}/{código}` com redirecionamento temporário (302) ao destino do link ativo.
- **FR-011**: O sistema MUST responder "não encontrado" para código inexistente, link inativo ou canal inexistente.
- **FR-012**: O redirecionamento MUST NOT depender da gravação do evento; falha ou lentidão da gravação não pode impedir nem atrasar a chegada ao destino.
- **FR-013**: Respostas ao visitante (redirecionamento, prévia, erro) MUST NOT conter cookies.
- **FR-014**: Links cuja política efetiva é `direct` MUST exibir no painel apenas a URL original (sem URLs curtas para copiar) e a marca "não rastreável". O código curto continua existindo: acessá-lo MUST redirecionar ao destino sem registrar evento. Estatísticas registradas enquanto o link era `shorten` MUST permanecer visíveis.

**Eventos e unicidade**
- **FR-015**: Cada acesso real MUST gerar um evento registrado de forma assíncrona, em lotes, com canal (quando houver), referenciador e user-agent.
- **FR-016**: Quando a fila de eventos estiver cheia, o sistema MUST descartar o evento e contabilizar o descarte em métrica.
- **FR-017**: O sistema MUST identificar o visitante por um identificador anônimo irreversível derivado do IP normalizado (IPv4 completo; IPv6 por prefixo /64) e do user-agent, sem gravar IP puro em banco ou logs.
- **FR-018**: O sistema MUST confiar no endereço informado por cabeçalho de proxy somente quando a conexão vier de um proxy configurado como confiável.
- **FR-019**: Cada evento MUST registrar duas marcas de unicidade permanentes: primeiro acesso do visitante àquele alvo naquele canal e primeiro acesso àquele alvo em qualquer canal, calculadas corretamente sob concorrência.

**Robôs e prévia**
- **FR-020**: Crawlers conhecidos MUST receber página de prévia com título, imagem, descrição, URL e tipo (metadados de compartilhamento), sem redirecionamento e sem gerar evento.
- **FR-021**: Quando um mesmo IP acessar 5 ou mais códigos distintos em uma janela de 10 segundos (ambos configuráveis no ambiente), o acesso que atinge o limite e os seguintes desse IP dentro da janela MUST ser classificados como automação. Esses acessos continuam redirecionados e MUST ser registrados com marca de robô, excluídos de todas as métricas e mantidos apenas por 30 dias.
- **FR-022**: Toda página HTML pública MUST exibir aviso informativo de privacidade, dispensável e lembrado sem cookie, com link para a página de privacidade; o aviso MUST NOT ser pedido de consentimento.
- **FR-023**: O sistema MUST servir uma página pública de privacidade com identificação anônima, finalidade, prazos de retenção e como solicitar exclusão.

**Estatísticas e retenção**
- **FR-024**: O sistema MUST expor, via interface programática autenticada, cliques totais e únicos por link, por canal e por dia, com filtro de período, e o detalhe de um link por canal e série diária.
- **FR-025**: O painel MUST exibir dashboard (totais e únicos no período, links ativos, top links, comparação entre canais, série diária), lista de links com cliques totais/únicos e detalhe do link com estatísticas por canal e série diária, todos com seletor de período (7, 30, 90 dias, personalizado).
- **FR-026**: O painel MUST exibir as limitações da contagem de únicos (mudança de IP infla; IP compartilhado reduz).
- **FR-027**: O sistema MUST consolidar diariamente as contagens do dia anterior, ignorando robôs, de forma idempotente.
- **FR-027a**: O "dia" usado na consolidação, na série diária, no seletor de período e nos prazos de retenção MUST seguir um fuso horário de relatório configurado no ambiente (padrão `America/Sao_Paulo`); os momentos dos eventos MUST continuar armazenados em UTC. O fuso MUST NOT ser alterado depois que existirem dados consolidados.
- **FR-028**: O sistema MUST remover diariamente eventos além da retenção (395 dias para reais, 30 para robôs; configuráveis) somente se o dia já estiver consolidado, preservando contagens totais e únicas.
- **FR-029**: Consolidados MUST ser mantidos indefinidamente; registros de unicidade MUST ser mantidos enquanto o alvo existir, inclusive durante a lixeira, e apagados somente na exclusão definitiva. As contagens consolidadas de itens excluídos MUST continuar compondo os totais do dashboard e da comparação entre canais.
- **FR-030**: Consultas de períodos fechados MUST usar os consolidados; o dia corrente, os eventos.

**Interface**
- **FR-031**: O painel MUST estar em português do Brasil, ser utilizável em celular, ter modos claro e escuro (respeitando a preferência do sistema, com alternância manual lembrada), contraste e navegação por teclado adequados e componentes visuais padronizados.
- **FR-032**: Listagens vazias MUST orientar o próximo passo.
- **FR-033**: Nenhuma página pública MUST carregar recursos de terceiros.

**Operação e segurança**
- **FR-034**: O sistema MUST limitar as rotas públicas a 300 requisições por minuto por IP (configurável); requisições excedentes MUST receber resposta de "muitas requisições", sem redirecionar e sem gerar evento.
- **FR-035**: O sistema MUST expor verificação de saúde, logs estruturados e métricas de requisições, eventos descartados e latência.
- **FR-036**: A configuração MUST vir do ambiente (incluindo segredo do identificador de visitante, banco, URL base, proxies confiáveis, credenciais iniciais, prazos de retenção e da lixeira, fuso horário de relatório, limites de detecção de automação, de login e de taxa); o sistema MUST falhar na partida com mensagem clara se faltar configuração obrigatória.
- **FR-037**: Estrutura de dados MUST evoluir por migrações versionadas, executadas na inicialização ou por comando documentado.
- **FR-038**: O sistema MUST subir, junto com seu banco, por um único comando a partir de variáveis de ambiente, com documentação de execução.

### Key Entities

- **Organização**: espaço de trabalho dono dos demais dados; existe uma só na fase 1.
- **Usuário (administrador)**: pessoa autenticada que opera o painel.
- **Marketplace**: plataforma de destino, com política de encurtamento `shorten` ou `direct`.
- **Canal**: origem de divulgação, representada por um segmento de URL único na organização.
- **Link de afiliado**: destino cadastrado (título, imagem opcional, URL original, política opcional, ativo/inativo), ligado a um marketplace.
- **Código curto**: identificador global de sete caracteres que resolve para um link de afiliado.
- **Evento de acesso**: registro de um acesso real (momento, alvo, canal, visitante anônimo, marcas de unicidade, marca de robô, referenciador, user-agent).
- **Registro de unicidade do visitante**: primeira vez que um visitante anônimo acessou um alvo, geral e por canal.
- **Consolidado diário**: totais e únicos por dia, alvo e canal, sem robôs.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: O administrador cadastra marketplace, canal e link e copia as URLs geradas em menos de 3 minutos, sem usar a interface programática.
- **SC-002**: 100% dos acessos a links ativos dentro do limite de taxa por IP chegam ao destino correto, inclusive com a gravação de eventos parada.
- **SC-003**: O redirecionamento responde em menos de 50 ms para 95% dos acessos, medido no servidor.
- **SC-004**: Dois acessos do mesmo IP e user-agent ao mesmo link resultam em 2 cliques e 1 único; mudar o user-agent resulta em novo único, em 100% dos casos testados.
- **SC-005**: Prévias de crawlers conhecidos não alteram nenhum contador (variação zero).
- **SC-006**: Após consolidação e limpeza, as contagens totais e únicas de um período antigo são idênticas às anteriores; reexecutar a consolidação não altera valores.
- **SC-007**: Nenhuma resposta ao visitante contém cookie e nenhum IP puro é encontrado em dados persistidos ou logs.
- **SC-008**: Links "direct" aparecem como "não rastreável" em 100% das telas de métricas, nunca como zero.
- **SC-009**: Cadastro e consulta de métricas funcionam em tela de celular, nos dois temas.
- **SC-010**: Um conjunto novo sobe com um único comando e a verificação de saúde responde com sucesso em até 2 minutos.
- **SC-011**: Perda de eventos por descarte é medida e visível em métrica, e permanece abaixo de 0,1% em operação normal.

## Assumptions

- Uso inicial por um único administrador e uma única organização; multiusuário, papéis, site público, produtos, categorias e domínio customizado estão fora desta fase.
- Fora de escopo permanente: integração com APIs de marketplaces, conversões/comissões, preço e alertas, ranking/pontuação, avaliações e veredito.
- **[ASSUNÇÃO]** Links `direct`: na fase 1 não há métrica alguma, apenas a marca "não rastreável"; a métrica futura (visualização da página de produto) depende de confirmação do responsável (questão em aberto 3 do escopo) e não é decidida aqui.
- **[ASSUNÇÃO]** Domínio curto ainda não existe; em desenvolvimento usa-se `localhost` e a URL base é configurável.
- Contagem de únicos é uma estimativa consistente para comparar canais, não precisão científica (aceito conscientemente).
- Retenção padrão: eventos reais 395 dias, robôs 30 dias, consolidados indefinidamente, registros de unicidade enquanto o alvo existir; prazos configuráveis.
- A lixeira de 30 dias (decidida pelo responsável nesta clarificação) refina a regra do escopo "ao excluir um link, apagar `visitor_seen`": a remoção ocorre na exclusão definitiva, não no envio à lixeira. Registrar o refinamento no escopo/`docs/`.
- O segredo do identificador de visitante é fixo e nunca rotacionado, pois isso quebraria a unicidade permanente.
- A revisão jurídica da base legal (legítimo interesse) fica para antes de abrir o serviço a terceiros.
- Depende de ambiente com contêineres para o deploy; a ordem de entrega segue os oito épicos da seção 13 do escopo.
