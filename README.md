# bitrix_deals

Сервис для загрузки сделок из Bitrix24 в PostgreSQL и выдачи данных по HTTP для интеграции с Google Sheets.

## Что делает сервис

- Загружает сделки из Bitrix24 (полный и дельта-синк).
- Сохраняет сделки в таблицу `bitrix_deals`.
- Хранит watermark синхронизации в `sync_state`.
- Отдает данные по HTTP:
  - `GET /deals/sheets`

## Требования

- Docker + Docker Compose
- (для локального запуска без Docker) Go `1.24.5` и PostgreSQL

## Переменные окружения

Минимально нужны:

- `BITRIX_WEBHOOK_BASE_URL` — базовый URL вебхука Bitrix24
- `DATABASE_URL` — строка подключения к PostgreSQL

Пример в файле `.env.example`.

Для Docker-окружения можно использовать `.env.docker`.

## Быстрый старт (Docker)

1. Заполните `.env.docker`:

```env
BITRIX_WEBHOOK_BASE_URL=https://<portal>.bitrix24.../rest/<user>/<webhook>/
DATABASE_URL=postgres://postgres:postgres@postgres:5432/bitrix?sslmode=disable&options=-c%20TimeZone%3DUTC
NGROK_AUTHTOKEN=<your_ngrok_token>
```

2. Запустите контейнеры:

```bash
docker compose --env-file .env.docker up --build -d
```

3. Выполните первый полный синк (обязательно один раз, чтобы поставить watermark):

```bash
docker compose --env-file .env.docker run --rm api full
```

4. После этого API доступно на `http://localhost:8080`.

## Локальный запуск (без Docker)

1. Поднимите PostgreSQL.
2. Установите переменные окружения (`BITRIX_WEBHOOK_BASE_URL`, `DATABASE_URL`).
3. Запустите полный синк:

```bash
go run ./cmd full
```

4. Запустите сервер:

```bash
go run ./cmd serve
```

Или одной командой (дельта-синк + сервер):

```bash
go run ./cmd serve-delta
```

## Режимы запуска

- `full` — полный импорт сделок с `>=DATE_CREATE: 2024-01-01`.
- `delta` — обновление по `>=DATE_MODIFY` от watermark с overlap 10 минут.
- `serve` — только HTTP сервер.
- `serve-delta` — сначала `delta`, затем HTTP сервер и фоновый `delta` каждые `10 минут` (режим по умолчанию в Dockerfile).

## HTTP API

### `GET /deals/sheets`

Возвращает структуру для табличной интеграции:

- `headers`: массив заголовков
- `rows`: массив строк

Пример:

```bash
curl http://localhost:8080/deals/sheets
```

### `GET /health/sync`

Показывает состояние синхронизации:

- `watermark`
- `last_deal_modify`
- возраст watermark/последней сделки в секундах
- `status` (`ok` / `stale` / `no_watermark`)

Пример:

```bash
curl http://localhost:8080/health/sync
```

## Схема БД

DDL находится в `0001_create_db.sql`.
При старте приложение также выполняет миграцию программно (`Migrate()`), создавая:

- `bitrix_deals`
- `sync_state`
- индекс `bitrix_deals_date_modify_idx`

## Полезные команды

```bash
# логи API
docker compose logs -f api

# логи ngrok
docker compose logs -f ngrok

# остановить окружение
docker compose down
```

## Автообновление (macOS)

Настроен автоматический запуск `delta`-синхронизации:

- каждый день в `14:00` (локальное время Mac, `Asia/Almaty`),
- при входе в сессию macOS (`RunAtLoad`),
- при пробуждении из сна через `sleepwatcher`.

Фактический запуск ограничен cooldown в `2 часа` (`scripts/update_delta.sh`), чтобы избежать частых повторов.
