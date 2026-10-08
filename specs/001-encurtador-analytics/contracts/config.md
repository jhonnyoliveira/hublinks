# Contrato: configuração por ambiente

Uma variável obrigatória ausente ou um valor inválido fazem a aplicação abortar na partida, com
uma mensagem que nomeia a variável. Segredos nunca aparecem em logs.

| Variável | Obrigatória | Padrão | Descrição |
|---|---|---|---|
| `DATABASE_URL` | sim | | Conexão PostgreSQL |
| `PEPPER` | sim | | Segredo do HMAC do visitante, com ≥ 32 bytes. **Nunca rotacionar** |
| `BASE_URL` | sim | | Ex.: `http://localhost:8080`; usado nas URLs geradas e no Open Graph |
| `HTTP_ADDR` | não | `:8080` | Endereço HTTP |
| `METRICS_ADDR` | não | `:9091` | Endereço das métricas Prometheus (vazio desativa) |
| `APP_ENV` | não | `production` | `development` habilita `/admin/ui` |
| `TRUSTED_PROXIES` | não | vazio | CIDRs separados por vírgula |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | na 1ª execução | | Criam o administrador se não houver usuários; senha com ≥ 12 caracteres |
| `ORG_NAME` | não | `Minha organização` | Nome da org padrão criada na partida |
| `REPORT_TZ` | não | `America/Sao_Paulo` | Fuso do "dia"; imutável após a 1ª agregação |
| `EVENTS_RETENTION_DAYS` | não | `395` | Retenção de eventos reais |
| `BOT_EVENTS_RETENTION_DAYS` | não | `30` | Retenção de eventos de robô |
| `TRASH_RETENTION_DAYS` | não | `30` | Prazo da lixeira |
| `BOT_DISTINCT_CODES` | não | `5` | Códigos distintos que marcam automação |
| `BOT_WINDOW` | não | `10s` | Janela da detecção |
| `PUBLIC_RATE_LIMIT` | não | `300` | Requisições por minuto por IP nas rotas públicas |
| `LOGIN_MAX_FAILURES` | não | `5` | Falhas de login permitidas |
| `LOGIN_WINDOW`, `LOGIN_LOCKOUT` | não | `15m`, `15m` | Janela e duração do bloqueio |
| `EVENT_QUEUE_SIZE` | não | `10000` | Capacidade da fila de eventos |
| `EVENT_BATCH_SIZE`, `EVENT_FLUSH_INTERVAL` | não | `500`, `1s` | Lote do worker |
| `MAINTENANCE_AT` | não | `00:30` | Horário diário dos jobs, em `REPORT_TZ` |
| `PRIVACY_CONTACT` | sim | | E-mail exibido em `/privacidade` para pedidos de exclusão |
| `LOG_LEVEL` | não | `info` | `debug`, `info`, `warn` ou `error` |

## Subcomandos do binário `hublinks`

- `hublinks serve` (padrão): aplica as migrações, faz o bootstrap (org e admin) e sobe o HTTP,
  o worker e o scheduler.
- `hublinks migrate [up|status]`.
- `hublinks maintenance run [--day YYYY-MM-DD]`: agrega, limpa e expira a lixeira uma vez.
