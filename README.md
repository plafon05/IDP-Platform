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

## Production: HTTPS и миграции

Production reverse proxy настраивается файлом `nginx/idp-platform.prod.conf`. Сертификат и приватный ключ монтируются только для чтения:

- `/etc/nginx/tls/fullchain.pem`
- `/etc/nginx/tls/privkey.pem`

Конфигурация перенаправляет HTTP на HTTPS, разрешает только TLS 1.2/1.3 и заменяет входящий `X-Forwarded-For` адресом непосредственного клиента.

В production API не применяет миграции при старте. Перед обновлением backend запустите одноразовый migration job из того же Docker image:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml run --rm migrate
```

Deployment должен продолжаться только после успешного завершения этой команды. Затем запускаются или обновляются `backend` и `email-worker`.

Production-контур описан отдельным `docker-compose.prod.yml`. Он не содержит development-сервисов и не публикует PostgreSQL, Redis, MinIO, backend или frontend наружу. Реальные значения создаются оператором:

```bash
cp .env.production.example .env.production
```

После заполнения обязательных переменных порядок запуска следующий:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml run --rm migrate
docker compose --env-file .env.production -f docker-compose.prod.yml up -d
```

Compose завершится ошибкой до запуска контейнеров, если обязательные адреса, images или секреты не заданы. Mailpit, demo seed и внешние порты инфраструктуры в production-конфигурацию не входят.

## CI/CD

Workflow `CI` запускает Go vet/tests, frontend build, integration-тесты с PostgreSQL и Redis, сборку Docker images и проверку production Compose/nginx.

При публикации Git tag вида `v1.2.3` workflow `Publish production images` собирает backend и frontend, формирует SBOM/provenance и публикует immutable images в GHCR:

- `ghcr.io/<owner>/<repository>-backend:v1.2.3`
- `ghcr.io/<owner>/<repository>-frontend:v1.2.3`

Полученные адреса указываются оператором в `BACKEND_IMAGE` и `FRONTEND_IMAGE`. Автоматический deploy на сервер не выполняется, пока не определена целевая инфраструктура.

## Backup и восстановление

Backup PostgreSQL и MinIO создаётся одной командой:

```bash
./scripts/backup-production.sh
```

Результат сохраняется в `backups/<UTC-дата>/`. Папка `backups` не отслеживается Git и должна копироваться оператором за пределы production-сервера.

Восстановление заменяет текущие данные и требует явного подтверждения:

```bash
RESTORE_CONFIRM=yes ./scripts/restore-production.sh backups/<UTC-дата>
```

Во время восстановления gateway, backend и email-worker останавливаются. После PostgreSQL автоматически применяются недостающие миграции, затем восстанавливается MinIO и приложение запускается снова. При ошибке сервисы остаются остановленными, чтобы не запустить приложение на частично восстановленных данных.

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
