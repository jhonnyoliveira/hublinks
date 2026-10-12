#!/usr/bin/env bash
# Sobe a aplicação num banco descartável para validação manual (quickstart, passo 2).
#
# Uso: scripts/manual-run.sh [--reset] [--drop]
#   --reset  recria o banco antes de subir (apaga os dados anteriores)
#   --drop   remove o banco ao encerrar (Ctrl+C)
#
# Variáveis opcionais (valores padrão entre parênteses):
#   MANUAL_DB (hl_manual)  PGHOST (postgres)  PGUSER (hublinks)  PGPASSWORD (hublinks)
#   HTTP_ADDR (:8080)  METRICS_ADDR (:9091)  ADMIN_EMAIL  ADMIN_PASSWORD
# Qualquer outra variável da aplicação já definida no ambiente é respeitada.
set -euo pipefail

reset=0 drop=0
for arg in "$@"; do
  case "$arg" in
    --reset) reset=1 ;;
    --drop) drop=1 ;;
    -h | --help) sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "argumento desconhecido: $arg (use --help)" >&2; exit 2 ;;
  esac
done

cd "$(dirname "$0")/.."

db="${MANUAL_DB:-hl_manual}"
export PGHOST="${PGHOST:-postgres}" PGUSER="${PGUSER:-hublinks}" PGPASSWORD="${PGPASSWORD:-hublinks}"
if ! [[ "$db" =~ ^[a-z_][a-z0-9_]*$ ]]; then echo "MANUAL_DB inválido: $db" >&2; exit 2; fi
command -v psql >/dev/null || { echo "psql não encontrado" >&2; exit 1; }

admin() { psql -X -q -d postgres -v ON_ERROR_STOP=1 "$@" 2>&1 | grep -v -E '^(perl: warning|\s+(LC_|LANG|are supported))' || true; }

if [ "$reset" = 1 ]; then
  admin -c "DROP DATABASE IF EXISTS $db WITH (FORCE)"
fi
if [ "$(psql -X -At -d postgres -c "SELECT 1 FROM pg_database WHERE datname='$db'" 2>/dev/null)" != "1" ]; then
  admin -c "CREATE DATABASE $db"
  echo "Banco $db criado."
else
  echo "Banco $db já existe (use --reset para recriar)."
fi

if [ "$drop" = 1 ]; then
  trap 'echo; echo "Removendo o banco $db..."; admin -c "DROP DATABASE IF EXISTS $db WITH (FORCE)"' EXIT
fi

export DATABASE_URL="postgresql://$PGUSER:$PGPASSWORD@$PGHOST:5432/$db"
export PEPPER="${PEPPER:-0123456789012345678901234567890123456789}"
export HTTP_ADDR="${HTTP_ADDR:-:8080}"
export METRICS_ADDR="${METRICS_ADDR-:9091}"
export BASE_URL="${BASE_URL:-http://localhost:${HTTP_ADDR##*:}}"
export PRIVACY_CONTACT="${PRIVACY_CONTACT:-voce@example.com}"
export ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
export ADMIN_PASSWORD="${ADMIN_PASSWORD:-senha-de-teste-123}"
export APP_ENV="${APP_ENV:-development}"
export EVENT_FLUSH_INTERVAL="${EVENT_FLUSH_INTERVAL:-300ms}"

echo "Compilando o CSS..."
make css >/dev/null

cat <<MSG

Painel:    $BASE_URL/admin   ($ADMIN_EMAIL / $ADMIN_PASSWORD)
Métricas:  ${METRICS_ADDR:-desativadas}
Encerrar:  Ctrl+C

MSG
go run ./cmd/hublinks serve
