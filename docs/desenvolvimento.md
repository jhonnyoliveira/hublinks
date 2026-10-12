# Desenvolvimento

## Comandos

| Comando | O que faz |
|---|---|
| `make run` | Sobe o serviço (`hublinks serve`). Exige as variáveis de `specs/001-encurtador-analytics/contracts/config.md`. |
| `make css` | Compila `web/assets/app.css` com o Tailwind v4 standalone (sem Node/npm) e grava `web/static/css/app.<hash>.css` e `web/static/manifest.json`. O binário é baixado para `bin/` com a versão de `TAILWIND_VERSION` e **conferido contra o `sha256sums.txt` da release**. |
| `make test` / `make test-race` | `go test ./...` (com `-race` no segundo). |
| `make test-ci` | Como `make test`, mas **falha se algum teste for pulado**. Use para ter certeza de que os testes de integração rodaram. |
| `make lint` | `gofmt -l` e `go vet`. |

## Testes de integração

Eles usam PostgreSQL real e criam um banco temporário por teste. Sem `TEST_DATABASE_URL` eles são **pulados em silêncio**
(`go test` passa mesmo assim). O `Makefile` e o serviço `dev` do `docker-compose.yml` já a definem; fora deles, exporte
`TEST_DATABASE_URL=postgresql://hublinks:hublinks@postgres:5432/postgres`.

## Interface

- **Fonte única de estilo:** `web/static/css/base.css` (tokens, base e componentes em CSS puro). O Tailwind
  (`web/assets/app.css`) o importa e expõe os tokens como utilitários (`@theme inline`); sem `make css`, o layout usa
  `base.css` direto (`asset "app.css"` resolve o nome com hash pelo manifest e cai em `css/base.css`).
- **Tema:** classe `.dark`/`.light` no `<html>`, definida antes da primeira pintura (`theme_init`), a partir de
  `localStorage["theme"]` ou de `prefers-color-scheme`. Sem JavaScript, vale a preferência do sistema.
- **Contraste:** `web/design_test.go` lê os tokens do `base.css` e exige WCAG AA (4,5:1 para texto, 3:1 para o anel de foco).
  Ao mudar uma cor, rode `go test ./web`.
- **Componentes:** `web/templates/components/*.html` (parâmetros documentados no topo de cada arquivo) e ícones Lucide em
  `components/icons/*.svg` (`{{icon "nome"}}`). Com `APP_ENV=development`, `/admin/ui` mostra todos.
- **Arquivos de template não podem começar com `_`:** o `go:embed` de um diretório os ignora.
- **Dependências vendorizadas:** `web/static/vendor/VERSIONS.md` (versões, licenças e hashes).

## Verificação em navegador

Os testes Go cobrem o HTML que o servidor devolve, mas não o JavaScript (HTMX, Alpine, modal, tema) nem o layout real.
Para isso há o `scripts/browser-check`, que usa Chromium headless (Playwright) e axe-core, instalados **fora do
repositório**, em `~/.cache/hublinks-browser` (o `Dockerfile.dev` já os instala; fora dele, rode uma vez
`make browser-setup`, que é idempotente).

Suba o app contra um banco descartável, sem tocar no de desenvolvimento (`MANUAL_DB=hl_bc HTTP_ADDR=:8099 METRICS_ADDR=
scripts/manual-run.sh --reset --drop`) e rode:

```sh
make browser-check ARGS="--base http://localhost:8099 --login admin@example.com:senha-de-teste-123 \
  --path /admin/login --path /privacidade --path /admin --path /zzzzzzz=404"
```

Para cada caminho, tamanho de tela (`--viewports`, padrão `375x667,1280x800`) e tema (`--schemes`, padrão `light,dark`) ele
confere: status esperado (`caminho=status`), rolagem horizontal, `lang`, recursos de terceiros, erros de console e de
JavaScript, acessibilidade e contraste (axe-core, WCAG 2.x A e AA) e a ordem de foco por Tab com indicador visível.
Grava capturas e `report.json` em `var/browser-check/` (ignorado pelo Git). **Falhas** (saída 1): status inesperado,
rolagem horizontal, terceiros, erro de console, `axe` sério ou crítico. **Avisos** não falham: foco sem indicador
visível, `axe` moderado, `h1` fora de 1.

O que ele **não** faz: julgar o visual, testar fluxos (cliques de HTMX, formulários) nem medir Lighthouse. Isso
continua com uma pessoa, usando as capturas como apoio.
