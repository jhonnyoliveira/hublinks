# Quickstart: validação da Fase 2

Guia para provar os critérios de aceite de ponta a ponta. Contratos em [contracts/](contracts/),
modelo em [data-model.md](data-model.md). Os passos 1 a 3 valem para o desenvolvimento; o 9 para o
deploy.

## Pré-requisitos

- Devcontainer com Go 1.27 e `psql`, ou Docker com Compose v2.
- Fase 1 aceita e funcionando (`specs/001-encurtador-analytics/quickstart.md`, passos 1 e 2).
- Duas imagens de teste: uma JPEG ou PNG de pelo menos 1600 px de largura e um logotipo PNG com
  transparência (use arquivos fictícios).
- Variáveis novas (opcionais em desenvolvimento): `MEDIA_DIR`, `MEDIA_MAX_BYTES`, `PAGE_CACHE_MB`.

## 1. Testes automatizados

```bash
export TEST_DATABASE_URL=postgresql://hublinks:hublinks@postgres:5432/postgres
gofmt -l .                 # esperado: nenhuma saída
go vet ./...
go test ./...
make test-ci               # falha se algum teste de integração for pulado
go test -race ./internal/events/... ./internal/redirect/... ./internal/web/...
```

Esperado: tudo passa; os testes de regressão da fase 1 (totais de links sem eventos de produto)
continuam verdes.

## 2. Subir o ambiente

```bash
scripts/manual-run.sh --reset      # banco descartável; MEDIA_DIR em var/media
```

Painel em `http://localhost:8080/admin` (credenciais do `manual-run.sh`). Cadastre antes um
marketplace, dois canais (`wapp`, `ig`) e dois links de afiliado, um `shorten` e outro `direct`
(marketplaces diferentes).

## 3. Publicar um produto (História 1, SC-001)

1. *(Só depois da História 6; antes disso, pule este item e a categoria do item 2.)* Em **Categorias**, crie "Fones". Em **Marketplaces**, envie um logotipo a um deles.
2. Em **Produtos → Novo**: título "Fone Bluetooth X1", aceite o slug sugerido (`fone-bluetooth-x1`),
   categoria, descrição curta e longa; envie a imagem de 1600 px; vincule os dois links.
3. Abra a **pré-visualização** (deve mostrar "Rascunho") e publique.
4. Sem sessão (`curl -si`), confira:
   ```bash
   curl -si localhost:8080/p/fone-bluetooth-x1 | head -20      # 200, ETag, Cache-Control: no-cache, sem Set-Cookie
   curl -si localhost:8080/p/inexistente-x | head -1           # 404
   ```
5. Despublique e repita: `404`. Publique de novo. Cronometre o fluxo (meta: < 5 min, SC-001).

## 4. Página pública (História 2, SC-003, SC-010)

Abra `/p/fone-bluetooth-x1` num celular ou no DevTools a 375 px, nos dois temas e só com o teclado:
a ação "Ver onde comprar" aparece sem rolar; a barra fixa surge ao rolar; "Onde comprar" mostra
os botões por marketplace com logotipo e sem preço; o link `direct` abre a URL original; o link
`shorten` passa pelo redirecionamento (`302`). Desative o JavaScript: botões e acordeão funcionam.

## 5. Ficha técnica e relacionados (História 3)

No formulário, crie três grupos com itens, reordene com os botões e relacione dois outros
produtos publicados. Na página: só o primeiro grupo aberto, "Mostrar todos" abre todos,
`/p/fone-bluetooth-x1#tela` abre o grupo "Tela". Despublique um relacionado: ele some do carrossel.
Tente relacionar o produto a ele mesmo: recusado.

## 6. Visitas e cliques (História 4, SC-005, SC-006, SC-009)

```bash
P=$(PGHOST=postgres PGUSER=hublinks PGPASSWORD=hublinks psql -d hl_manual -Atc "select code from short_codes where target_type='product' limit 1")
curl -si localhost:8080/$P         -A "UA-1" | head -8   # 200 mesma página, sem Location, sem Set-Cookie
curl -si localhost:8080/wapp/$P    -A "UA-1" >/dev/null
curl -si localhost:8080/p/fone-bluetooth-x1 -A "WhatsApp/2.23" >/dev/null   # crawler: sem evento
curl -s -o /dev/null -w '%{http_code}\n' -X POST localhost:8080/api/v1/beacon/view \
  -H 'Origin: http://localhost:8080' -H 'Content-Type: text/plain' -d "{\"k\":\"$P\",\"c\":\"wapp\"}"   # 204
```

Aguarde ~1 s (lote). No painel, o detalhe do produto mostra visitas e cliques **separados**; o
mesmo user-agent repetido não cria novo único; trocar o user-agent cria. O link `direct` continua
"não rastreável", sem contadores. No navegador real, abrir a página gera exatamente 1 visita
(confira no banco: `select event_type, count(*) from click_events group by 1`).

## 7. Compartilhamento e SEO (História 5)

Cole `…/p/fone-bluetooth-x1` e `…/{código}` num validador de prévia (ou inspecione o HTML): título,
imagem (a maior variante gerada), descrição e `og:url` canônico igual ao endereço amigável. O JSON-LD tem
`@type: Product` e **não** contém `offers`. Há um único `<h1>`.

## 8. Edição, cache e lixeira (SC-007)

*(Vale a partir da História 2.)*

```bash
E=$(curl -si localhost:8080/p/fone-bluetooth-x1 | grep -i '^etag' | cut -d' ' -f2)
curl -si -H "If-None-Match: $E" localhost:8080/p/fone-bluetooth-x1 | head -1    # 304
```

Edite o título no painel e repita o comando: agora `200` com `ETag` novo, já na primeira abertura.
Exclua o produto: o endereço e o código passam a `404`; restaure dentro de 30 dias e voltam.
Inative o link vinculado: some de "Onde comprar" na abertura seguinte.

## 9. Mídia, backup e deploy

```bash
docker compose -f deploy/docker-compose.yml --env-file deploy/.env up -d --build
docker compose -f deploy/docker-compose.yml exec app /hublinks healthcheck   # saudável
```

Envie imagens pelo painel, recrie o contêiner (`up -d --force-recreate app`) e confirme que as
imagens continuam servidas (`/media/{id}/640.webp` → `200`). Siga o roteiro de backup do volume
`hublinks-media` em `docs/operacao.md`. Um `MEDIA_DIR` sem permissão de escrita deve impedir a
partida citando a variável.

## 10. Privacidade e desempenho (SC-002, SC-004, SC-008, SC-011)

- `curl -si` em página, mídia e beacon: nenhum `Set-Cookie`; o HTML não referencia domínio externo.
- Auditoria automatizada (`internal/audit`) continua verde: sem IP em log ou banco.
- Carga: `scripts/loadtest.sh` adaptado à página (200 req/s por 60 s, catálogo de 1.000 produtos
  publicados semeados por script): sem erros, p95 < 100 ms no servidor, descarte < 0,1 %.
- Lighthouse/Web Vitals no celular emulado: LCP < 2,5 s, CLS < 0,1, INP < 200 ms.
