# Hub Links

Serviço Go e PostgreSQL para links curtos de afiliado com analytics por canal.

No devcontainer, defina `TEST_DATABASE_URL`, execute `make css` e `make run`. Para os testes:
`GOCACHE=/tmp/hublinks-go-build GOPATH=/tmp/hublinks-go make test`.

Para o deploy, copie `deploy/.env.example` para `deploy/.env`, substitua os valores fictícios e
execute `docker compose -f deploy/docker-compose.yml --env-file deploy/.env up --build`.

As variáveis, seus padrões e exigências estão documentados em
[`contracts/config.md`](specs/001-encurtador-analytics/contracts/config.md). `PEPPER` é um segredo
fixo e não deve ser rotacionado após a entrada em produção.
