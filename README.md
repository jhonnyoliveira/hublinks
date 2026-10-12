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

Para o deploy, copie `deploy/.env.example` para `deploy/.env`, substitua os valores fictícios e
execute `docker compose -f deploy/docker-compose.yml --env-file deploy/.env up --build`.

As variáveis, seus padrões e exigências estão documentados em
[`contracts/config.md`](specs/001-encurtador-analytics/contracts/config.md). `PEPPER` é um segredo
fixo e não deve ser rotacionado após a entrada em produção.
