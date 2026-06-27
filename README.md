# mytonstorage-gateway

**[Русская версия](README.ru.md)**

Gateway service for accessing TON Storage bags - provides web interface and API for browsing TON Storage content.

## Description

This gateway service provides public access to files stored on TON Storage network:
- Retrieves bag information from local TON Storage daemon
- Fetches bags from remote TON Storage network via ADNL/DHT when not available locally
- Streams files and directories from TON Storage bags
- Manages content moderation via reports and bans system
- Provides web-based file browser with HTML templates
- Exposes REST API endpoints for public and administrative access
- Collects metrics via **Prometheus**

## Local development (Docker)

**Requires:** [Docker](https://docs.docker.com/), [Task](https://taskfile.dev), Go 1.24+ (for local builds outside containers).

Stack in containers: **PostgreSQL**, **tonutils-storage** (v1.5.1), **gateway**.

```bash
task deploy:up
task deploy:health
task deploy:logs
task deploy:down
task deploy:reset   # remove volumes
```

API: `http://localhost:9093` (port set via `GATEWAY_PORT` in `deploy/.env`).

Config: copy `deploy/.env.example` → `deploy/.env` or run `task deploy:init`.  
`DB_USER` must be **pguser** — required by `db/init.sql`.

## Deploy on VPS (Docker Hub)

```bash
# dev machine
GATEWAY_IMAGE=rudolfkova/mytonstorage-gateway:latest \
TONUTILS_STORAGE_IMAGE=rudolfkova/mytonutils-storage:v1.5.1 \
task image:build:push

# VPS
task hub:init && nano deploy/.env.hub && task hub:up && task hub:health
```

See [deploy/README.md](deploy/README.md) for details.

## Dev:
### VS Code Configuration
Create `.vscode/launch.json`:
```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Launch Package",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/cmd",
            "buildFlags": "-tags=debug",    // to handle OPTIONS queries without nginx when dev
            "env": {...}
        }
    ]
}
```

## Project Structure

```
├── cmd/                          # Application entry point, configs, inits
├── pkg/                          # Application packages
│   ├── clients/                  # TON Storage clients
│   │   ├── ton-storage/          # Local TON Storage daemon client
│   │   └── remote-ton-storage/   # Remote TON Storage network client
│   ├── httpServer/               # Fiber server handlers and routes
│   ├── iframewrap/               # iframe wrapper for secure html display
│   ├── models/                   # DB and API data models
│   ├── repositories/             # Database
│   ├── services/                 # Business logic. File browsing, streaming and content moderation (reports, bans)
│   ├── templates/                # HTML templates wrapper
├── bruno-collection/             # Bruno API testing collection
├── db/                           # PostgreSQL schema (moderation)
├── deploy/                       # Docker Compose, .env.example
├── scripts/                      # Setup and utility scripts
├── templates/                    # HTML template files
├── Dockerfile
└── Taskfile.yml
```

## API Endpoints

The server provides REST API endpoints for gateway access, content moderation (reports and bans), health checks, and Prometheus metrics. Protected endpoints require Bearer token authentication with granular permissions.

## TON Storage Clients

The service uses two storage clients:
- **Local Client** - communicates with local TON Storage daemon via HTTP API
- **Remote Client** - connects to TON Storage network via ADNL/DHT for retrieving bags from remote peers. Implemented based on [tonutils-proxy](https://github.com/xssnick/Tonutils-Proxy)

## Bruno API Collection

The project includes a Bruno API collection for testing all endpoints. See `bruno-collection/README.md` for setup instructions.

## License

Apache-2.0



This project was created by order of a TON Foundation community member.
