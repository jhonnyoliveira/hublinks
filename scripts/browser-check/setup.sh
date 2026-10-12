#!/bin/sh
# Instala, de forma idempotente, o Playwright, o axe-core e o Chromium usados por
# scripts/browser-check/check.mjs. Fica fora do repositório (e fora do build da aplicação):
#   BROWSER_TOOLS_DIR (padrão: ~/.cache/hublinks-browser)
# Uso: sh scripts/browser-check/setup.sh        # também roda no build do Dockerfile.dev
set -eu

SRC="$(cd "$(dirname "$0")" && pwd)"
DIR="${BROWSER_TOOLS_DIR:-$HOME/.cache/hublinks-browser}"
STAMP="$(cksum "$SRC/package.json" "$SRC/package-lock.json" | tr '\n' ' ')"

if [ -f "$DIR/.installed" ] && [ "$(cat "$DIR/.installed")" = "$STAMP" ] && [ -d "$DIR/browsers" ]; then
  echo "browser-check: já instalado em $DIR"
  exit 0
fi

mkdir -p "$DIR"
cp "$SRC/package.json" "$SRC/package-lock.json" "$DIR/"
cd "$DIR"
npm ci --no-audit --no-fund
PLAYWRIGHT_BROWSERS_PATH="$DIR/browsers" npx playwright install chromium
printf '%s' "$STAMP" > "$DIR/.installed"
echo "browser-check: instalado em $DIR"
