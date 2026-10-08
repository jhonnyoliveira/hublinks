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

Os testes Go cobrem o HTML que o servidor devolve, mas não o JavaScript (HTMX, Alpine, modal, tema). Para isso usamos
um Chromium headless, com o Playwright instalado **fora do repositório**:

```sh
mkdir -p /tmp/pw && cd /tmp/pw && npm init -y && npm i playwright && npx playwright install chromium
```

As bibliotecas de sistema do Chromium vêm de `Dockerfile.dev` (reconstrua o DevContainer). Suba o app contra um banco
descartável (`CREATE DATABASE`/`DROP DATABASE`), sem tocar no banco de desenvolvimento.
