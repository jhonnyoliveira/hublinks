# Operação

Guia para quem mantém o Hub Links no ar: configuração, cuidados que não têm volta, rotinas diárias,
métricas e alertas, backup e carga.

## Configuração

As variáveis, os padrões e as exigências estão em
[`contracts/config.md`](../specs/001-encurtador-analytics/contracts/config.md); `deploy/.env.example`
traz todas com valores fictícios. Uma variável obrigatória ausente ou inválida faz o serviço sair na
partida com uma mensagem que a nomeia. O serviço aplica as migrações ao iniciar.

### Cuidados que não têm volta

- **`PEPPER`** é o segredo do identificador anônimo de visitante (HMAC do IP normalizado e do
  navegador). Trocá-lo muda todos os identificadores e quebra a unicidade permanente: cada visitante
  antigo voltaria a contar como "novo". Guarde-o em local seguro, **nunca o rotacione** e nunca o
  versione.
- **`REPORT_TZ`** define o que é um "dia" nas consolidações, nas séries e na retenção. É fixado na primeira
  consolidação; depois disso o serviço recusa subir com outro fuso.
- **`PEPPER` e `POSTGRES_PASSWORD`** do compose não podem ficar nos valores de exemplo em produção.

## Backup do banco

Todo o estado está no PostgreSQL (o serviço não grava arquivos). Faça backup lógico diário e teste a
restauração:

```bash
# backup (dentro do compose)
docker compose -f deploy/docker-compose.yml exec -T db pg_dump -U hublinks -Fc hublinks > hublinks-$(date +%F).dump

# restauração em um banco vazio
docker compose -f deploy/docker-compose.yml exec -T db pg_restore -U hublinks -d hublinks --clean --if-exists < hublinks-2026-10-01.dump
```

Guarde também o `.env` (principalmente `PEPPER`), separado do dump: sem o mesmo `PEPPER`, os visitantes
restaurados deixam de bater com os novos acessos. O volume `hublinks-postgres` guarda os dados entre
reinícios, mas não substitui o backup.

Eventos ainda na fila em memória (no máximo `EVENT_QUEUE_SIZE`, esvaziada em ~1 s) são descarregados no
desligamento normal; num `kill -9` ou queda da máquina, esses poucos segundos de cliques se perdem.

## Proxy reverso e limites

O limite de taxa (`PUBLIC_RATE_LIMIT`, por minuto) e o bloqueio de login usam o IP do visitante. Atrás de um
proxy, defina `TRUSTED_PROXIES` com os endereços do proxy; sem isso, todos os visitantes compartilham o IP
do proxy e um único usuário pode esgotar a cota de todos. O proxy também deve preservar o cabeçalho `Host`
(a verificação de origem do painel compara `Origin` com ele) e acrescentar o IP real a `X-Forwarded-For`.

Quem excede a cota recebe `429` com `Retry-After` (segundos). A janela é deslizante: depois de esgotar a cota
de uma vez, a espera pode chegar a 60 s. `/healthz`, `/privacidade` e `/static` não entram nesse limite.
Não publique a porta de métricas no proxy.

## Manutenção diária

Todo dia, em `MAINTENANCE_AT` (padrão `00:30`, no fuso `REPORT_TZ`), o serviço: (1) consolida os cliques de
cada dia passado ainda não consolidado, sem robôs, em `click_daily`; (2) remove eventos além da retenção
(`EVENTS_RETENTION_DAYS` para reais, `BOT_EVENTS_RETENTION_DAYS` para robôs), **somente de dias já
consolidados**; (3) torna definitiva a exclusão de itens na lixeira há mais de `TRASH_RETENTION_DAYS`,
apagando o registro de unicidade dos links. Os totais e os visitantes únicos não mudam com a limpeza. Só uma
instância executa por vez (advisory lock). Se o serviço ficou fora do ar no horário, a execução seguinte
consolida todos os dias pendentes.

Execução manual (por exemplo, para recuperar um atraso ou reconsolidar um dia):

```bash
hublinks maintenance run                      # consolida os dias pendentes, limpa e expira a lixeira
hublinks maintenance run --day 2026-10-01     # reconsolida esse dia (YYYY-MM-DD, em REPORT_TZ)
```

`--day` recusa o dia corrente e dias já consolidados que estão fora da retenção de eventos: refazer a
consolidação a partir de eventos já removidos zeraria o histórico.

## Métricas e alertas

`METRICS_ADDR` (padrão `:9091`; vazio desativa) expõe `GET /metrics` no formato Prometheus, em porta separada
do site público.

| Métrica | Significado | Alerta sugerido |
|---|---|---|
| `hublinks_events_enqueued_total` | eventos aceitos na fila | — |
| `hublinks_events_dropped_total` | eventos descartados por fila cheia | `rate(...[5m]) > 0` |
| `hublinks_events_written_total` | eventos gravados no banco | — |
| `hublinks_events_write_errors_total` | lotes perdidos após 3 tentativas | `increase(...[10m]) > 0` |
| `hublinks_event_queue_length` | eventos aguardando gravação | próximo de `EVENT_QUEUE_SIZE` por mais de 1 min |
| `hublinks_http_requests_total{route,status}` | requisições por rota (padrão do mux) e status | taxa de `status=~"5.."` |
| `hublinks_redirect_duration_seconds` | latência do redirecionamento (histograma) | p95 > 50 ms |
| `hublinks_maintenance_duration_seconds` | duração da última manutenção | — |
| `hublinks_maintenance_failures_total` | manutenções que falharam | `increase(...[1d]) > 0` |
| `hublinks_maintenance_last_success_timestamp_seconds` | instante do último sucesso | `time() - ... > 26*3600` |

A rota nas métricas é o padrão do roteador, nunca o caminho real: códigos curtos e ids não viram rótulos.
O redirecionamento público é `route="GET /"` (todo `/{code}` e `/{segment}/{code}`). Logs são JSON em
`stdout` e não contêm IP, user-agent nem referer.

## Teste de carga

`scripts/loadtest.sh <código> [taxa] [duração]` dispara `vegeta` contra `GET /{code}` e imprime o p95 do
cliente e do servidor e a taxa de descarte de eventos, com o veredicto contra as metas (p95 < 50 ms, SC-003;
descarte < 0,1 %, SC-011). Instale o `vegeta` com
`GOBIN=$PWD/bin go install github.com/tsenart/vegeta/v12@latest`.

Suba o servidor com `PUBLIC_RATE_LIMIT` alto (por exemplo `1000000`): a carga sai de um único IP e o padrão
de 300 requisições por minuto responderia `429` a quase tudo. Execute somente em ambiente isolado.

Resultado medido (16 vCPUs, PostgreSQL local, servidor e gerador na mesma máquina, código em cache,
500 req/s por 60 s, 30 000 requisições):

| Medida | Resultado | Meta |
|---|---|---|
| Respostas | 30 000 × `302`, 100 % de sucesso | — |
| p50 / p95 / p99 no cliente | 0,54 ms / 0,88 ms / 1,24 ms | p95 < 50 ms |
| p95 no servidor (histograma) | ≤ 0,5 ms | < 50 ms |
| Eventos enfileirados / gravados / descartados | 30 000 / 30 000 / 0 | descarte < 0,1 % |
| Erros de gravação | 0 | 0 |

A medição local exclui a rede e uma máquina menor; repita no ambiente de produção antes de confiar nos
números absolutos.

## Privacidade

Nenhum IP é gravado ou logado: ele existe só em memória, para o limite de taxa, o bloqueio de login e o
identificador anônimo (HMAC). Nenhuma resposta pública emite cookie; o único cookie é o da sessão do painel.
O teste `internal/audit` falha se `RemoteAddr`, `X-Forwarded-For` ou cookies passarem a ser usados fora de
`httpx/clientip.go` e `auth/session.go`, se uma migração criar coluna de IP ou se um log receber IP,
user-agent ou referer. A página pública `/privacidade` mostra os prazos configurados e o contato.
