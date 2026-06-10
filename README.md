# IDP Platform

Веб-приложение для управления индивидуальными планами развития сотрудников.

## Структура

- `backend` — Go API по Clean Architecture.
- `frontend` — React + TypeScript + Vite SPA.
- `backend/migrations` — SQL-миграции PostgreSQL для goose.
- `docker-compose.yml` — локальный контур PostgreSQL, Redis, MinIO, backend, frontend.

## Локальный запуск

```bash
cp .env.example .env
docker compose up --build
```

После запуска:

- API health: `http://localhost:8080/health`
- API ready: `http://localhost:8080/ready`
- Frontend через контейнер: `http://localhost:3000`

Для разработки фронтенда с HMR:

```bash
cd frontend
npm install
npm run dev
```

Для разработки API:

```bash
cd backend
go mod download
go run ./cmd/server
```

## Текущий статус

Первая итерация создаёт фундамент проекта:

- базовый Go HTTP-сервер с graceful shutdown;
- endpoints `/health`, `/ready`, `/api/v1/health`, `/api/v1/ready`;
- единый JSON-формат ошибок;
- CORS для локального фронтенда;
- начальная PostgreSQL-схема по ТЗ;
- первый экран React-приложения с дашбордом;
- Docker Compose для локальной инфраструктуры.
