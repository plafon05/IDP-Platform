#!/usr/bin/env bash
set -Eeuo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
backup_dir="${1:-}"
env_file="${2:-$root_dir/.env.production}"
compose_file="$root_dir/docker-compose.prod.yml"

if [[ -z "$backup_dir" || ! -f "$backup_dir/postgres.dump" || ! -d "$backup_dir/minio" ]]; then
  echo "Usage: $0 <backup-directory> [env-file]" >&2
  exit 1
fi
if [[ ! -f "$env_file" ]]; then
  echo "Production environment file not found: $env_file" >&2
  exit 1
fi
if [[ "${RESTORE_CONFIRM:-}" != "yes" ]]; then
  echo "Restore replaces current production data. Run with RESTORE_CONFIRM=yes." >&2
  exit 1
fi

backup_dir="$(cd "$backup_dir" && pwd)"
compose=(docker compose --env-file "$env_file" -f "$compose_file")
trap 'echo "Restore failed; backend, worker and gateway remain stopped." >&2' ERR

echo "Stopping application services..."
"${compose[@]}" stop gateway backend email-worker

echo "Restoring PostgreSQL..."
"${compose[@]}" exec -T postgres sh -c \
  'exec pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists --no-owner --no-privileges --exit-on-error' \
  < "$backup_dir/postgres.dump"

echo "Applying migrations..."
"${compose[@]}" --profile tools run --rm migrate

echo "Restoring MinIO..."
"${compose[@]}" --profile tools run --rm --no-deps \
  -v "$backup_dir:/backup:ro" \
  --entrypoint /bin/sh storage-tool -c \
  'mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null && mc mb --ignore-existing "local/$MINIO_BUCKET" && mc mirror --overwrite --remove /backup/minio "local/$MINIO_BUCKET" && mc anonymous set download "local/$MINIO_BUCKET"'

trap - ERR
echo "Starting application services..."
"${compose[@]}" up -d backend email-worker frontend gateway
echo "Restore completed. Check /health and application logs."
