# Verificação final (T109)

Registro do que foi executado e do que ainda depende de uma pessoa ou de um ambiente com Docker (Docker já validado, T104).
Atualizado em 2026-10-12, na branch `001-encurtador-analytics`.

## Executado e aprovado

| Verificação | Resultado |
|---|---|
| `gofmt -l .` | vazio |
| `go vet ./...` | sem avisos |
| `go test -race ./...` com `TEST_DATABASE_URL` (PostgreSQL real) | todos os pacotes passam |
| `go test -race` em events, redirect, auth, httpx, maintenance e metrics | passam |
| `make test-ci` (falha se algum teste for pulado) | 0 testes pulados |
| Servidor real (curl, banco temporário): cadastro, redirecionamento, cliques e únicos, prévia de crawler, robôs, bloqueio de login, `429`, `/privacidade` | conforme a spec (ver `AGENTS.md`) |
| `hublinks maintenance run` / `--day` / fuso divergente / lixeira | conforme a spec |
| Carga: 500 req/s por 60 s no redirecionamento | p95 0,88 ms (meta < 50 ms); descarte 0 % (meta < 0,1 %) — `docs/operacao.md` |
| Privacidade (passo 10): 19 linhas de log sob requisições com `X-Forwarded-For`, 0 com IPv4; 0 colunas de IP no banco | aprovado; `internal/audit` mantém a invariante |
| Estágio de CSS do Dockerfile e `go build` da imagem reproduzidos fora do Docker | CSS com hash gerado, binário estático, `/healthz` e `hublinks healthcheck` OK |
| T104 — `docker build` e `docker compose up` (passo 2 do `quickstart.md`) | validado pelo responsável em 2026-10-12 |

## Pendente (não pôde ser executado neste ambiente)

1. **Passo 9 do `quickstart.md` (SC-009)**: sem navegador aqui. Percorrer login → novo link → detalhe →
   dashboard a 375 px, nos dois temas, só com o teclado: sem rolagem horizontal, foco visível, tema lembrado,
   estados vazios, gráfico, aviso do `unsaved.js` ao sair do formulário de link com alterações e contraste
   conferido no DevTools. `scripts/manual-run.sh` sobe o ambiente.
2. **Passo 3 do `quickstart.md` cronometrado (SC-001)**: cadastro de marketplace, canal e link em menos de 3
   minutos.
3. **Aceite do responsável** dos checkpoints das histórias 2 a 7.
