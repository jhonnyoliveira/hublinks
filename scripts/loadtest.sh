#!/usr/bin/env bash
# Teste de carga do redirecionamento público (SC-003 e SC-011).
#
# Uso: scripts/loadtest.sh <codigo-curto> [taxa=500] [duracao=60s]
#
# Dispara `vegeta` contra GET /<codigo> (resolução em cache), lê as métricas do
# servidor antes e depois e imprime:
#   - o p95 medido pelo cliente (vegeta) e o medido no servidor
#     (histograma hublinks_redirect_duration_seconds); meta: < 50 ms;
#   - a taxa de eventos descartados (hublinks_events_dropped_total sobre
#     hublinks_events_enqueued_total); meta: < 0,1 %.
#
# Variáveis: BASE_URL (http://localhost:8080), METRICS_URL (http://localhost:9091/metrics),
# VEGETA (bin/vegeta ou o do PATH; instale com
#   GOBIN=$PWD/bin go install github.com/tsenart/vegeta/v12@latest).
#
# ATENÇÃO: suba o servidor com PUBLIC_RATE_LIMIT alto (por exemplo 1000000). O padrão,
# 300 requisições por minuto por IP, responderia 429 à maior parte da carga, que sai
# de um único IP.
set -euo pipefail

code="${1:?informe o código curto}"
rate="${2:-500}"
duration="${3:-60s}"
base="${BASE_URL:-http://localhost:8080}"
metrics="${METRICS_URL:-http://localhost:9091/metrics}"
cd "$(dirname "$0")/.."
vegeta="${VEGETA:-}"
if [ -z "$vegeta" ]; then
  if [ -x bin/vegeta ]; then vegeta=bin/vegeta; elif command -v vegeta >/dev/null; then vegeta=vegeta; fi
fi
[ -n "$vegeta" ] || { echo "vegeta não encontrado (veja o cabeçalho deste script)" >&2; exit 1; }

scrape() { curl -sf "$metrics" || { echo "não consegui ler $metrics (METRICS_ADDR ligado?)" >&2; exit 1; }; }
value() { awk -v m="$2" '$1 == m { print $2 }' <<<"$1" | head -1; }

before="$(scrape)"
results="$(mktemp)"
trap 'rm -f "$results"' EXIT
printf 'GET %s/%s\n' "$base" "$code" | "$vegeta" attack -rate="$rate" -duration="$duration" -redirects=-1 >"$results" # -1: não segue o 302 (o destino é um site de terceiros)
sleep 2 # deixa o worker esvaziar a fila
after="$(scrape)"

echo "== vegeta =="
"$vegeta" report <"$results"
client_p95="$("$vegeta" report -type=json <"$results" | python3 -c 'import sys,json; print(json.load(sys.stdin)["latencies"]["95th"]/1e6)')"
echo
echo "== servidor =="
enq=$(awk -v a="$(value "$after" hublinks_events_enqueued_total)" -v b="$(value "$before" hublinks_events_enqueued_total)" 'BEGIN{print a-b}')
drop=$(awk -v a="$(value "$after" hublinks_events_dropped_total)" -v b="$(value "$before" hublinks_events_dropped_total)" 'BEGIN{print a-b}')
written=$(awk -v a="$(value "$after" hublinks_events_written_total)" -v b="$(value "$before" hublinks_events_written_total)" 'BEGIN{print a-b}')
werr=$(awk -v a="$(value "$after" hublinks_events_write_errors_total)" -v b="$(value "$before" hublinks_events_write_errors_total)" 'BEGIN{print a-b}')
# p95 do servidor: menor limite `le` do histograma cujo acumulado (delta) cobre 95 %.
server_p95="$(awk -v before="$before" -v after="$after" '
  function load(text, arr,   n, lines, i, le) {
    n = split(text, lines, "\n")
    for (i = 1; i <= n; i++) if (match(lines[i], /^hublinks_redirect_duration_seconds_bucket\{le="[^"]+"\} /)) {
      le = lines[i]; sub(/^[^"]*"/, "", le); sub(/".*$/, "", le); arr[le] = lines[i]; sub(/^.*\} /, "", arr[le])
    }
  }
  BEGIN {
    load(before, b); load(after, a)
    total = a["+Inf"] - b["+Inf"]
    if (total <= 0) { print "nan"; exit }
    best = ""; bestle = 1e18
    for (le in a) if (le != "+Inf") { d = a[le] - b[le]; v = le + 0; if (d >= 0.95 * total && v < bestle) { bestle = v; best = le } }
    print (best == "" ? "+Inf" : bestle * 1000)
  }')"
printf 'eventos enfileirados: %s\neventos descartados:  %s\neventos gravados:     %s\nerros de gravação:    %s\n' "$enq" "$drop" "$written" "$werr"
rate_pct=$(awk -v d="$drop" -v e="$enq" -v x="$drop" 'BEGIN{ t = e + x; if (t == 0) print "0"; else printf "%.4f", 100 * d / t }')
printf 'taxa de descarte:     %s %%  (meta < 0,1 %%)\n' "$rate_pct"
printf 'p95 no cliente:       %.2f ms\n' "$client_p95"
printf 'p95 no servidor:      %s ms  (limite superior do balde; meta < 50 ms)\n' "$server_p95"
ok=1
only302="$("$vegeta" report -type=json <"$results" | python3 -c 'import sys,json; d=json.load(sys.stdin)["status_codes"]; print(1 if list(d)==["302"] else 0)')"
[ "$only302" = 1 ] || { echo "ATENÇÃO: houve respostas diferentes de 302"; ok=0; }
awk -v v="$client_p95" 'BEGIN{exit !(v < 50)}' || ok=0
awk -v v="$rate_pct" 'BEGIN{exit !(v < 0.1)}' || ok=0
if [ "$ok" = 1 ]; then echo "RESULTADO: dentro das metas (SC-003, SC-011)"; else echo "RESULTADO: FORA das metas"; exit 3; fi
