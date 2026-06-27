# Деплой mytonstorage-gateway на VPS

**[English version](README.md)**

Стек (hub compose):
- `gateway` (образ из registry)
- `tonutils-storage` (образ из registry)
- `postgres` (официальный образ)

## Docker Hub (рекомендуется для VPS)

| Шаг | Где | Команда |
|-----|-----|---------|
| 1 | Dev-машина | `GATEWAY_IMAGE=... TONUTILS_STORAGE_IMAGE=... task image:build:push` |
| 2 | VPS | Клон репо |
| 3 | VPS | `task hub:init` → правка `deploy/.env.hub` |
| 4 | VPS | `task hub:up` |

Hub compose ([docker-compose.hub.yml](docker-compose.hub.yml)) — `pull_policy: always`, без локальной сборки.

### Dev-машина: build + push

```bash
GATEWAY_IMAGE=rudolfkova/mytonstorage-gateway:latest \
TONUTILS_STORAGE_IMAGE=rudolfkova/mytonutils-storage:v1.5.1 \
task image:build:push
```

Release gateway (без debug CORS): не задавай `BUILD_TAGS` или `BUILD_TAGS=`.

### VPS: pull + up

```bash
task hub:init
nano deploy/.env.hub
task hub:up
task hub:health
```

Обязательно в `.env.hub`:
- `GATEWAY_IMAGE`, `TONUTILS_STORAGE_IMAGE`
- `DB_PASSWORD` — сильный пароль Postgres
- `SYSTEM_ACCESS_TOKENS` — Bearer-токены для metrics/reports/bans (формат см. `cmd/config.go`)
- `TON_STORAGE_LOGIN`, `TON_STORAGE_PASSWORD` — credentials tonutils-storage API
- `TONUTILS_STORAGE_EXTERNAL_IP` — публичный IP/DNS хоста (для overlay DHT)
- `POSTGRES_PORT`, `GATEWAY_PORT`, `TONUTILS_STORAGE_API_PORT`, `TONUTILS_STORAGE_UDP_PORT` — не конфликтовать с другими стеками на той же VPS (дефолты: `5434`, `9093`, `13475`, `47432`)

Остановка: `task hub:down`.

## Локальная разработка

[docker-compose.yml](docker-compose.yml) + `task deploy:up` — сборка на месте. См. [README.ru.md](../README.ru.md).

## Nginx (прокси с mytonstorage.org)

Фронтенд ссылается на `${API_BASE}/api/v1/gateway/...`. Gateway слушает `:9093`, backend — `:9092`. Пример:

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

Остальной `/api/v1/*` — на backend `:9092`.

## Co-location с mytonstorage-backend

Оба стека независимы (`name: mytonstorage-gateway` vs `name: mytonstorage`). На одной VPS используй смещённые порты из `.env.hub.example` — backend обычно занимает `5433`, `9092`, `13474`, `47431`.
