#!/usr/bin/env bash
set -Eeuo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
env_file="${1:-$root_dir/.env.production}"
backup_root="${2:-$root_dir/backups}"
compose_file="$root_dir/docker-compose.prod.yml"

if [[ ! -f "$env_file" ]]; then
  echo "Production environment file not found: $env_file" >&2
  exit 1
fi

mkdir -p "$backup_root"
backup_root="$(cd "$backup_root" && pwd)"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
temporary="$backup_root/.${timestamp}.tmp"
destination="$backup_root/$timestamp"
compose=(docker compose --env-file "$env_file" -f "$compose_file")

mkdir -p "$temporary"
trap 'rm -rf "$temporary"' EXIT

echo "Creating PostgreSQL backup..."
"${compose[@]}" exec -T postgres sh -c \
  'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner --no-privileges' \
  > "$temporary/postgres.dump"

echo "Creating MinIO backup..."
"${compose[@]}" --profile tools run --rm --no-deps \
  -v "$temporary:/backup" \
  --entrypoint /bin/sh storage-tool -c \
  'mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null && mc mirror --overwrite "local/$MINIO_BUCKET" /backup/minio'

printf 'created_at=%s\n' "$timestamp" > "$temporary/metadata.txt"
mv "$temporary" "$destination"
trap - EXIT

echo "Backup created: $destination"
