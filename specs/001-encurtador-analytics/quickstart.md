# Quickstart: validação da Fase 1

Guia para provar os critérios de aceite de ponta a ponta. Contratos em [contracts/](contracts/),
modelo em [data-model.md](data-model.md).

## Pré-requisitos

- Devcontainer do projeto (Go 1.27, `psql`), com o serviço `postgres` do `docker-compose.yml`
  da raiz, **ou** Docker com Compose v2 para o deploy.
- Copie `deploy/.env.example` para `deploy/.env` e preencha com valores fictícios
  (`PEPPER` com ≥ 32 caracteres, `ADMIN_EMAIL`, `ADMIN_PASSWORD`, `PRIVACY_CONTACT`).

## 1. Testes automatizados

```bash
export TEST_DATABASE_URL=postgresql://hublinks:hublinks@postgres:5432/postgres
gofmt -l .                 # esperado: nenhuma saída
go vet ./...
go test ./...
go test -race ./internal/events/... ./internal/redirect/...
```

Esperado: tudo passa e nenhum teste de integração é pulado (com `TEST_DATABASE_URL` definida).

## 2. Subir pelo compose (SC-010)

```bash
docker compose -f deploy/docker-compose.yml --env-file deploy/.env up -d --build
curl -s localhost:8080/healthz          # {"status":"ok"} em até 2 min
```

Sem `PEPPER`, a aplicação deve sair com um erro que cita `PEPPER`.

## 3. Cadastro pelo painel (SC-001, História 1)

1. Abra `http://localhost:8080/admin`, que redireciona ao login, e entre com o administrador do
   `.env`.
2. Crie o marketplace "Loja Exemplo" (`shorten`), o canal "WhatsApp" (`wapp`) e um link para
   `https://example.com/produto`.
3. No detalhe do link, copie a URL curta e a URL `/wapp/{code}`.

## 4. Redirecionamento e contagem (SC-002, SC-004, SC-007)

```bash
C=<código>
curl -si localhost:8080/$C      | grep -iE '^(HTTP|location|set-cookie)'   # 302, Location, sem Set-Cookie
curl -si localhost:8080/$C -A "UA-1" >/dev/null; curl -si localhost:8080/$C -A "UA-1" >/dev/null
curl -si localhost:8080/$C -A "UA-2" >/dev/null
curl -si localhost:8080/naoexis/$C | head -1                               # 404
```

Após cerca de 2 s, o detalhe do link mostra para o UA-1: +2 cliques e +1 único; o UA-2 conta
como mais um único.

## 5. Prévia de crawler (SC-005)

```bash
curl -s localhost:8080/$C -A "WhatsApp/2.23" | grep -E 'og:(title|image)|privacidade'
```

Esperado: `200`, metatags e link de privacidade; os contadores não mudam.

## 6. Política `direct` (SC-008)

Mude o marketplace para `direct`. Esperado:
- O detalhe e a lista mostram a URL original e "não rastreável".
- `curl` no código ainda responde `302` e os contadores não mudam.

## 7. Resiliência do worker (SC-002)

Os testes `internal/events` e `internal/redirect` cobrem fila cheia e worker parado. Em
execução manual, pare o banco (`docker compose ... stop postgres`) e acesse o código já em
cache. Esperado: `302`; a métrica `hublinks_events_dropped_total` aumenta
(`curl localhost:9091/metrics` dentro da rede do compose).

## 8. Agregação, limpeza e lixeira (SC-006)

```bash
docker compose -f deploy/docker-compose.yml exec app hublinks maintenance run --day 2026-10-01
```

O teste de integração `internal/maintenance` cobre o cenário completo:
1. semeia eventos antigos;
2. compara as contagens antes e depois de agregar e limpar (devem ser idênticas);
3. reexecuta a agregação (os valores não mudam);
4. verifica que dias não agregados são preservados;
5. verifica que a lixeira expira após 30 dias e apaga `visitor_seen`.

## 9. Interface (SC-009)

No DevTools, com a largura de 375 px, percorra login → novo link → detalhe → dashboard, nos dois
temas. Esperado:
- sem rolagem horizontal;
- foco visível navegando com Tab;
- tema lembrado após recarregar;
- estados vazios com orientação em uma instalação nova.

## 10. Privacidade

`curl -s localhost:8080/privacidade` mostra a identificação anônima, a finalidade, os prazos
(conforme o `.env`) e o contato.

Verifique também que não há IP puro em lugar nenhum:

```bash
docker compose -f deploy/docker-compose.yml logs app | grep -cE '([0-9]{1,3}\.){3}[0-9]{1,3}'
```

Esperado: `0`.
