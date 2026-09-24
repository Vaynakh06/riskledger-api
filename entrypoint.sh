#!/bin/sh
set -eu

if [ "${DEMO_MODE:-false}" = "true" ]; then
  echo "DEMO_MODE is enabled; skipping database migrations."
  exec /app/app
fi

if [ -n "${DATABASE_URL:-}" ]; then
  echo "Running database migrations..."
  for sql in /app/migrations/*.sql; do
    echo "Applying $(basename "$sql")"
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$sql"
  done
else
  echo "DATABASE_URL is not set; skipping migrations."
fi

exec /app/app
