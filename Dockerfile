FROM golang:1.27 AS build

WORKDIR /src

# Node drives the JS asset pipeline, brotli/zstd are used by web/build.sh, and
# rustup provides the wasm32 target used by the WebAssembly PoW modules.
RUN apt-get update && apt-get install -y --no-install-recommends \
    nodejs \
    npm \
    brotli \
    zstd \
    ca-certificates \
    curl \
    git \
    && rm -rf /var/lib/apt/lists/*

ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    PATH=/usr/local/cargo/bin:$PATH
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \
    | sh -s -- -y --profile minimal --default-toolchain stable \
    && rustup target add wasm32-unknown-unknown

COPY go.mod go.sum package.json package-lock.json ./
RUN go mod download

COPY . .

# Ensure we use container-built node_modules, not host ones (e.g. macOS binaries).
RUN rm -rf node_modules && npm ci

ENV PATH="/src/node_modules/.bin:${PATH}"

RUN npm run assets

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=devel
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -buildvcs=false \
    -ldflags="-s -w -X 'github.com/TecharoHQ/anubis.Version=${VERSION}'" \
    -o /out/anubis ./cmd/anubis


FROM alpine:3.22

COPY --from=build /out/anubis /usr/local/bin/anubis

RUN addgroup -S anubis && adduser -S anubis -G anubis
USER anubis:anubis
EXPOSE 8923 9090

ENTRYPOINT ["/usr/local/bin/anubis"]
