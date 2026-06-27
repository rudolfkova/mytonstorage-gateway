FROM golang:1.25-bookworm AS builder

ARG TONUTILS_STORAGE_VERSION=v1.5.1

RUN apt-get update && \
    apt-get install -y --no-install-recommends git ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /src
RUN git clone --depth 1 --branch "${TONUTILS_STORAGE_VERSION}" \
    https://github.com/xssnick/tonutils-storage.git .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/tonutils-storage ./cli

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/tonutils-storage /usr/local/bin/tonutils-storage
COPY tonutils-storage-entrypoint.sh /usr/local/bin/tonutils-storage-entrypoint.sh

RUN chmod +x /usr/local/bin/tonutils-storage-entrypoint.sh && \
    mkdir -p /data/db /bags

EXPOSE 13474 47431/udp

ENTRYPOINT ["/usr/local/bin/tonutils-storage-entrypoint.sh"]
CMD ["--daemon", "--db", "/data/db", "--api", "0.0.0.0:13474"]
