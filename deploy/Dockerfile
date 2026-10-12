# Imagem de produção do Hub Links: CSS (Tailwind standalone) → binário Go estático → distroless.
# Contexto de build: a raiz do repositório (veja deploy/docker-compose.yml).

# 1) CSS. O Tailwind standalone é um binário glibc, então usamos Debian (não Alpine/musl).
#    A entrada web/assets/app.css importa web/static/css/base.css e varre web/templates
#    atrás de classes: os três precisam estar presentes neste estágio.
FROM debian:12-slim AS css
ARG TAILWIND_VERSION=4.1.17
RUN apt-get update \
 && apt-get install -y --no-install-recommends curl ca-certificates \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY scripts/build-css.sh scripts/build-css.sh
COPY web/assets web/assets
COPY web/static/css/base.css web/static/css/base.css
COPY web/templates web/templates
RUN TAILWIND_VERSION=$TAILWIND_VERSION sh scripts/build-css.sh

# 2) Binário. CGO desligado: o resultado roda no distroless sem bibliotecas do sistema.
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CSS minificado com hash no nome e o manifest que o mapeia (embutidos no binário).
COPY --from=css /src/web/static/css web/static/css
COPY --from=css /src/web/static/manifest.json web/static/manifest.json
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /hublinks ./cmd/hublinks

# 3) Imagem final: sem shell, usuário não root.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /hublinks /hublinks
EXPOSE 8080
ENTRYPOINT ["/hublinks"]
CMD ["serve"]
