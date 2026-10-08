#!/usr/bin/env sh
set -eu
code="${1:?informe o código curto}"
base="${BASE_URL:-http://localhost:8080}"
printf 'GET %s/%s\n' "$base" "$code" | vegeta attack -rate=500 -duration=60s | vegeta report
