# IDP Platform

Веб-приложение для управления индивидуальными планами развития сотрудников.

## Структура

- `backend` — Go API по Clean Architecture.
- `frontend` — React + TypeScript + Vite SPA.
- `backend/migrations` — SQL-миграции PostgreSQL для goose.
- `docker-compose.yml` — локальный контур приложения и инфраструктуры.

## Локальный запуск

```bash
cp .env.example .env
docker compose up --build
```

После запуска:

- API health: `http://localhost:8080/health`
- API ready: `http://localhost:8080/ready`
- Frontend через контейнер: `http://localhost:3000`
- Mailpit для просмотра development-писем: `http://localhost:8025`

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

## Демо-данные

Для наполнения локальной базы наглядными данными:

```bash
docker compose --profile demo run --rm demo-seed
```

Команду можно запускать повторно. Демо-аккаунты используют пароль `Demo12345`:

- Руководитель: `manager.demo@idp.local`
- Сотрудник: `alexey.demo@idp.local`
- Сотрудник: `maria.demo@idp.local`

## Email в development

API ставит письма в Redis, а отдельный `email-worker` отправляет их через SMTP. Mailpit перехватывает письма локально и не отправляет их реальным получателям. Параметры SMTP и имя очереди задаются в `.env`.
