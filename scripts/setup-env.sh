#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
if [ -e .env ]; then
  echo '.env already exists; leaving it unchanged.'
  exit 0
fi
command -v openssl >/dev/null 2>&1 || { echo 'Install openssl to generate credentials.' >&2; exit 1; }
task_jwt=$(openssl rand -hex 32)
task_password=$(openssl rand -hex 24)
# noclobber and restrictive permissions protect an existing file and its secrets.
umask 077
set -C
cat > .env <<EOF
APP_ENV=development
PORT=3000
AUTO_MIGRATE=true
JWT_SECRET=$task_jwt
POSTGRES_PASSWORD=$task_password
DATABASE_URL=postgres://noteflow:$task_password@127.0.0.1:5432/noteflow?sslmode=disable
REDIS_URL=redis://127.0.0.1:6379/0
TRUSTED_PROXIES=
EOF
echo 'Created .env with random local development credentials.'
