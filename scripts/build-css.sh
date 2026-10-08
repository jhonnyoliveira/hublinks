#!/usr/bin/env sh
set -eu
mkdir -p bin web/static/css
version="${TAILWIND_VERSION:-4.1.17}"
binary="bin/tailwindcss"
if [ ! -x "$binary" ]; then
  curl -fsSL "https://github.com/tailwindlabs/tailwindcss/releases/download/v${version}/tailwindcss-linux-x64" -o "$binary"
  chmod +x "$binary"
fi
tmp="web/static/css/app.css"
"$binary" -i web/assets/app.css -o "$tmp" --minify
hash=$(sha256sum "$tmp" | cut -c1-8)
output="web/static/css/app.${hash}.css"
mv "$tmp" "$output"
printf '{"app.css":"css/app.%s.css"}\n' "$hash" > web/static/manifest.json
