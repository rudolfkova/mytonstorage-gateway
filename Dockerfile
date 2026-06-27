FROM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
COPY . .

ARG BUILD_TAGS=
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -tags="${BUILD_TAGS}" -ldflags="-s -w" -o /out/mtpo-gateway ./cmd

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/mtpo-gateway /usr/local/bin/mtpo-gateway
COPY templates /templates

ENV TEMPLATES_PATH=/templates

EXPOSE 9093

ENTRYPOINT ["/usr/local/bin/mtpo-gateway"]
