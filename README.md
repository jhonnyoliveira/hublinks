# Hub Links

Serviço Go e PostgreSQL para links curtos de afiliado com analytics por canal.

No devcontainer, defina `TEST_DATABASE_URL`, execute `make css` e `make run`. Para os testes:
`GOCACHE=/tmp/hublinks-go-build GOPATH=/tmp/hublinks-go make test`.

## Validação manual

`scripts/manual-run.sh` sobe a aplicação num banco descartável (`hl_manual`), com variáveis de teste,
`APP_ENV=development` e o CSS compilado. O roteiro completo está em
[`quickstart.md`](specs/001-encurtador-analytics/quickstart.md).

```bash
scripts/manual-run.sh                  # cria o banco se não existir e sobe o servidor
scripts/manual-run.sh --reset          # recria o banco antes (apaga os dados anteriores)
scripts/manual-run.sh --drop           # remove o banco ao encerrar (Ctrl+C)
scripts/manual-run.sh --reset --drop   # sessão totalmente descartável
```

O painel fica em `http://localhost:8080/admin` (`admin@example.com` / `senha-de-teste-123`) e as
métricas em `http://localhost:9091/metrics`, em porta separada do site. Qualquer variável já definida no
ambiente prevalece (`MANUAL_DB`, `PGHOST`, `PGUSER`, `PGPASSWORD`, `HTTP_ADDR`, `METRICS_ADDR`, `BASE_URL`,
`ADMIN_EMAIL`, `ADMIN_PASSWORD`). Use `scripts/manual-run.sh --help` para o resumo.

## Deploy

Requisitos: Docker com o plugin Compose. Copie `deploy/.env.example` para `deploy/.env`, substitua os
valores fictícios (principalmente `PEPPER`, `POSTGRES_PASSWORD`, `ADMIN_EMAIL` e `ADMIN_PASSWORD`) e execute:

```bash
docker compose -f deploy/docker-compose.yml --env-file deploy/.env up -d --build
curl -s localhost:8080/healthz        # {"status":"ok"}
```

O compose sobe a aplicação (imagem multi-stage: CSS com Tailwind standalone → binário Go estático →
distroless, usuário não root) e o PostgreSQL 16 com volume. As migrações rodam na partida e o healthcheck
da aplicação usa o próprio binário (`hublinks healthcheck`). A porta de métricas (`9091`) não é publicada;
atrás de um proxy reverso, veja [`docs/operacao.md`](docs/operacao.md) (`TRUSTED_PROXIES`, `Host`).
Sem `PEPPER`, a aplicação sai com um erro que cita a variável.

As variáveis, seus padrões e exigências estão documentados em
[`contracts/config.md`](specs/001-encurtador-analytics/contracts/config.md). `PEPPER` é um segredo
fixo e não deve ser rotacionado após a entrada em produção.
