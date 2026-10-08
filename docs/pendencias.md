# Pendências e assunções para confirmação do responsável

Itens registrados conforme `AGENTS.md`. Nada aqui altera ADRs; cada item aguarda confirmação.

## 1. Lixeira de 30 dias refina a regra de `visitor_seen` [DECIDIDO NA CLARIFICAÇÃO]

O escopo (seção 8.1) diz: "ao excluir um link ou produto, apagar as linhas correspondentes de
`visitor_seen`".

Na clarificação de 2026-10-08, o responsável escolheu uma lixeira com restauração por 30 dias.
Com isso, `visitor_seen` passa a ser apagado na **exclusão definitiva** (após o prazo), e não
ao enviar o item à lixeira. Seção 8.1 do escopo atualizada em 2026-10-08.

Referência: `specs/001-encurtador-analytics/spec.md`, FR-008a a FR-008d.

## 2. Colisão de chave em `visitor_seen` [DECIDIDO]

O modelo do escopo usa o UUID nulo em `channel_id` para dois casos: "sem canal" (URL curta) e a
linha de unicidade por alvo. Essas chaves colidem. Exemplo: um visitante que entrou pelo canal
`wapp` e depois pela URL curta teria `is_unique_url = false` na URL curta, quando o correto
seria `true`.

**Decisão (2026-10-08, aprovada pelo responsável):** acrescentar a coluna `scope` (`url` | `target`) à chave primária e também
`org_id` à tabela (exigido para tabelas de domínio).

Detalhes: `specs/001-encurtador-analytics/research.md` (R5) e `data-model.md`.

## 3. Exclusão de canal com links ativos [DECIDIDO]

FR-008c impede excluir "marketplace ou canal que possua links ativos". Na fase 1, links não
pertencem a canais, então a regra aplicada literalmente bloquearia a exclusão de qualquer canal
enquanto existir um link ativo.

**Decisão (2026-10-08, aprovada pelo responsável; FR-008c atualizado):**
- a trava vale só para marketplaces;
- para canais, a confirmação informa quantos links ativos terão as URLs daquele canal
  invalidadas.

## 4. Tabela `products` adiada para a fase 2 [DECIDIDO]

O escopo prevê um "esqueleto na fase 1", mas nenhum requisito da fase 1 usa produtos.

**Decisão (2026-10-08, aprovada pelo responsável):** criar a tabela por migração na fase 2. Na fase 1, `target_type` aceita apenas
`affiliate_link`.

## 5. Questão em aberto 3 do escopo: métrica de links `direct` [ABERTA]

A fase 1 segue a proposta do escopo: nenhuma métrica, apenas a marca "não rastreável". A métrica
da fase 2 não foi decidida.
