# mytonstorage-gateway

**[English version](README.md)**

Сервис предоставляет веб-интерфейс и API для просмотра содержимого TON Storage.

## Описание

Этот сервис обеспечивает публичный доступ к файлам, хранящимся в TON Storage:
- Получает информацию о багах из локального TON Storage
- Загружает баги с удаленных узлов TON Storage когда они недоступны локально
- Стримит файлы и директории
- Управляет модерацией контента через систему жалоб и банов
- Предоставляет веб-браузер файлов
- Предоставляет REST API endpoints для публичного и административного доступа
- Собирает метрики через **Prometheus**

## Локальная разработка (Docker)

**Нужно:** [Docker](https://docs.docker.com/), [Task](https://taskfile.dev), Go 1.24+ (для локальной сборки вне контейнера).

Стек в контейнерах: **PostgreSQL**, **tonutils-storage** (v1.5.1), **gateway**.

```bash
task deploy:up
task deploy:health
task deploy:logs
task deploy:down
task deploy:reset   # удалить volumes
```

API: `http://localhost:9093` (порт задаётся `GATEWAY_PORT` в `deploy/.env`).

Конфиг: `deploy/.env.example` → `deploy/.env` или `task deploy:init`.  
`DB_USER` должен быть **pguser** — так задано в `db/init.sql`.

## Деплой на VPS (Docker Hub)

```bash
# dev-машина
GATEWAY_IMAGE=rudolfkova/mytonstorage-gateway:latest \
TONUTILS_STORAGE_IMAGE=rudolfkova/mytonutils-storage:v1.5.1 \
task image:build:push

# VPS
task hub:init && nano deploy/.env.hub && task hub:up && task hub:health
```

Подробнее: [deploy/README.ru.md](deploy/README.ru.md).

## Разработка:
### Конфигурация VS Code
Создайте `.vscode/launch.json`:
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
            "buildFlags": "-tags=debug",    // для обработки OPTIONS запросов без nginx при разработке
            "env": {...}
        }
    ]
}
```

## Структура проекта

```
├── cmd/                          # Точка входа в приложение, конфиги, инициализация
├── pkg/                          # Пакеты приложения
│   ├── clients/                  # Клиенты TON Storage
│   │   ├── ton-storage/          # Клиент для локального TON Storage
│   │   └── remote-ton-storage/   # Клиент для общения с удаленными узлами TON Storage
│   ├── httpServer/               # Fiber
│   ├── iframewrap/               # Iframe обертка для HTML файлов
│   ├── models/                   # Модели данных БД и API
│   ├── repositories/             # Слой базы данных
│   ├── services/                 # Бизнес-логика. Просмотр файлов, баны, жалобы
│   ├── templates/                # Враппер для HTML файлов
├── bruno-collection/             # Коллекция Bruno для тестирования API
├── db/                           # Схема PostgreSQL (moderation)
├── deploy/                       # Docker Compose, .env.example
├── scripts/                      # Скрипты настройки и утилиты
├── templates/                    # Файлы HTML шаблонов
├── Dockerfile
└── Taskfile.yml
```

## API Endpoints

Сервер предоставляет REST API endpoints для доступа к файлам в TON Storage, модерации контента (жалобы и баны), проверки здоровья сервиса и метрик Prometheus. Защищённые endpoints требуют Bearer token аутентификацию с детальными разрешениями.

## Клиенты TON Storage

Сервис использует два клиента хранилища:
- **Локальный клиент** - взаимодействует с локальным демоном TON Storage через HTTP API
- **Удалённый клиент** - подключается к сети TON Storage через ADNL/DHT для поиска и получения багов с удалённых пиров

## Коллекция Bruno API

Проект включает коллекцию Bruno API для тестирования всех роутов. См. `bruno-collection/README.md` для инструкций по настройке.

## Лицензия

Apache-2.0



Этот проект был создан по заказу участника сообщества TON Foundation.
