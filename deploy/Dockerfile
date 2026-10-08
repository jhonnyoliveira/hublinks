FROM alpine:3.21 AS css
ARG TAILWIND_VERSION=4.1.17
RUN apk add --no-cache curl bash
WORKDIR /src
COPY web/assets web/assets
COPY scripts/build-css.sh scripts/build-css.sh
RUN TAILWIND_VERSION=$TAILWIND_VERSION scripts/build-css.sh

FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=css /src/web/static/css web/static/css
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /hublinks ./cmd/hublinks

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /hublinks /hublinks
EXPOSE 8080
ENTRYPOINT ["/hublinks"]
CMD ["serve"]
