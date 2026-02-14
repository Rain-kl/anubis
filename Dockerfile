FROM golang:1.24.2-alpine AS build

WORKDIR /src

RUN apk add --no-cache \
    nodejs \
    npm \
    brotli \
    zstd \
    bash

COPY go.mod go.sum package.json package-lock.json ./
RUN go mod download

COPY . .

# Ensure we use container-built node_modules, not host ones (e.g. macOS binaries).
RUN rm -rf node_modules && npm ci

ENV PATH="/src/node_modules/.bin:${PATH}"

RUN go generate ./...
RUN ./web/build.sh
RUN ./xess/build.sh

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=devel
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X 'github.com/TecharoHQ/anubis.Version=${VERSION}'" -o /out/anubis ./cmd/anubis

FROM alpine:3.20

COPY --from=build /out/anubis /usr/local/bin/anubis

RUN addgroup -S anubis && adduser -S anubis -G anubis
USER anubis:anubis
EXPOSE 8923 9090

ENTRYPOINT ["/usr/local/bin/anubis"]
