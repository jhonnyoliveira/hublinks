#!/usr/bin/env sh
# Compila web/assets/app.css com o Tailwind v4 standalone (sem Node/npm), grava
# web/static/css/app.<sha256-8>.css minificado e web/static/manifest.json.
# Uso: make css   (TAILWIND_VERSION fixa a versão do binário)
set -eu

version="${TAILWIND_VERSION:-4.1.17}"
mkdir -p bin web/static/css

case "$(uname -s)-$(uname -m)" in
  Linux-x86_64) asset="tailwindcss-linux-x64" ;;
  Linux-aarch64 | Linux-arm64) asset="tailwindcss-linux-arm64" ;;
  Darwin-x86_64) asset="tailwindcss-macos-x64" ;;
  Darwin-arm64) asset="tailwindcss-macos-arm64" ;;
  *) echo "plataforma sem binário do Tailwind: $(uname -s)-$(uname -m)" >&2; exit 1 ;;
esac

binary="bin/tailwindcss-${version}"
if [ ! -x "$binary" ]; then
  base="https://github.com/tailwindlabs/tailwindcss/releases/download/v${version}"
  tmp_bin="$(mktemp)"
  trap 'rm -f "$tmp_bin"' EXIT
  curl -fsSL "${base}/${asset}" -o "$tmp_bin"
  # confere o binário contra o sha256sums.txt publicado na mesma release
  expected="$(curl -fsSL "${base}/sha256sums.txt" | awk -v f="./${asset}" '$2 == f { print $1 }')"
  if [ -z "$expected" ]; then echo "checksum de ${asset} não encontrado" >&2; exit 1; fi
  if command -v sha256sum >/dev/null 2>&1; then actual="$(sha256sum "$tmp_bin" | cut -d' ' -f1)"; else actual="$(shasum -a 256 "$tmp_bin" | cut -d' ' -f1)"; fi
  if [ "$actual" != "$expected" ]; then echo "checksum divergente para ${asset}" >&2; exit 1; fi
  chmod +x "$tmp_bin"
  mv "$tmp_bin" "$binary"
  trap - EXIT
fi

tmp="web/static/css/app.css"
"$binary" -i web/assets/app.css -o "$tmp" --minify
if command -v sha256sum >/dev/null 2>&1; then hash="$(sha256sum "$tmp" | cut -c1-8)"; else hash="$(shasum -a 256 "$tmp" | cut -c1-8)"; fi
# remove CSS de builds anteriores (somente nomes com hash de 8 hex) e grava o novo
find web/static/css -maxdepth 1 -name 'app.[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f].css' -delete
output="web/static/css/app.${hash}.css"
mv "$tmp" "$output"
printf '{"app.css":"css/app.%s.css"}\n' "$hash" > web/static/manifest.json
echo "CSS gerado: $output"
