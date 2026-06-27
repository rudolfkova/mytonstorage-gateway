# Deploy mytonstorage-gateway on VPS

**[Русская версия](README.ru.md)**

Stack (hub compose):
- `gateway` (image from registry)
- `tonutils-storage` (image from registry)
- `postgres` (official image)

## Docker Hub (recommended for VPS)

| Step | Where | Command |
|------|-------|---------|
| 1 | Dev machine | `GATEWAY_IMAGE=... TONUTILS_STORAGE_IMAGE=... task image:build:push` |
| 2 | VPS | Clone repo |
| 3 | VPS | `task hub:init` → edit `deploy/.env.hub` |
| 4 | VPS | `task hub:up` |

Hub compose ([docker-compose.hub.yml](docker-compose.hub.yml)) uses `pull_policy: always`, no local build.

### Dev machine: build + push

```bash
GATEWAY_IMAGE=rudolfkova/mytonstorage-gateway:latest \
TONUTILS_STORAGE_IMAGE=rudolfkova/mytonutils-storage:v1.5.1 \
task image:build:push
```

Release gateway (no debug CORS): leave `BUILD_TAGS` unset or empty.

### VPS: pull + up

```bash
task hub:init
nano deploy/.env.hub
task hub:up
task hub:health
```

Required in `.env.hub`:
- `GATEWAY_IMAGE`, `TONUTILS_STORAGE_IMAGE`
- `DB_PASSWORD` — strong Postgres password
- `SYSTEM_ACCESS_TOKENS` — Bearer tokens for metrics/reports/bans (see `cmd/config.go`)
- `TON_STORAGE_LOGIN`, `TON_STORAGE_PASSWORD` — tonutils-storage API credentials
- `TONUTILS_STORAGE_EXTERNAL_IP` — public IP/DNS of the host
- Ports — avoid conflicts with other stacks on the same VPS (defaults: `5434`, `9093`, `13475`, `47432`)

Stop: `task hub:down`.

## Local development

[docker-compose.yml](docker-compose.yml) + `task deploy:up`. See [README.md](../README.md).

## Nginx (proxy from mytonstorage.org)

Frontend links to `${API_BASE}/api/v1/gateway/...`. Gateway listens on `:9093`, backend on `:9092`. Example:

```nginx
location /api/v1/gateway/ {
    proxy_pass http://127.0.0.1:9093;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

All other `/api/v1/*` routes go to backend `:9092`.

## Co-location with mytonstorage-backend

Both stacks are independent. On one VPS use offset ports from `.env.hub.example` — backend typically uses `5433`, `9092`, `13474`, `47431`.
