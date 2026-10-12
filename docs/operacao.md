# Operação

Configure as variáveis conforme `specs/001-encurtador-analytics/contracts/config.md`. Guarde backup
do PostgreSQL e preserve `PEPPER` e `REPORT_TZ`: trocar qualquer um depois de dados existentes muda
contagens históricas ou impede a interpretação correta dos dias. O serviço aplica migrações ao iniciar.

As métricas devem alertar para eventos descartados, falhas de escrita e falhas dos jobs de manutenção.
Execute carga somente num ambiente isolado, após instalar `vegeta`.

## Métricas

`METRICS_ADDR` (padrão `:9091`; vazio desativa) expõe `GET /metrics` no formato Prometheus, em porta separada do site público: não publique essa porta no proxy reverso.

| Métrica | Significado |
|---|---|
| `hublinks_events_enqueued_total` | eventos aceitos na fila |
| `hublinks_events_dropped_total` | eventos descartados por fila cheia (alerte se crescer) |
| `hublinks_events_written_total` | eventos gravados no banco |
| `hublinks_events_write_errors_total` | lotes perdidos após 3 tentativas (alerte se crescer) |
| `hublinks_event_queue_length` | eventos aguardando gravação |

## Proxy reverso e limites

O limite de taxa (`PUBLIC_RATE_LIMIT`, por minuto) e o bloqueio de login usam o IP do visitante. Atrás de um proxy, defina `TRUSTED_PROXIES` com os endereços do proxy; sem isso, todos os visitantes compartilham o IP do proxy e um único usuário pode esgotar a cota de todos. O proxy também deve preservar o cabeçalho `Host` e acrescentar o IP real a `X-Forwarded-For`.

Quem excede a cota recebe `429` com `Retry-After` (segundos). A janela é deslizante: depois de esgotar a cota de uma vez, a espera pode chegar a 60 s. `/healthz`, `/privacidade` e `/static` não entram nesse limite.

