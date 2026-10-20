#!/bin/sh
# Disposable databases only: the normal Compose database is never touched.
set -eu
cd "$(dirname "$0")/.."
task_pg="noteflow-test-pg-$$"
task_redis="noteflow-test-redis-$$"
cleanup() {
  docker rm -f "$task_pg" "$task_redis" >/dev/null 2>&1 || true
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
docker run -d --rm --name "$task_pg" -e POSTGRES_USER=noteflow \
  -e POSTGRES_PASSWORD=test-only-password -e POSTGRES_DB=noteflow_test \
  -p 127.0.0.1::5432 postgres:17-alpine >/dev/null
docker run -d --rm --name "$task_redis" -p 127.0.0.1::6379 redis:7-alpine >/dev/null
task_attempt=0
until docker exec "$task_pg" pg_isready -U noteflow -d noteflow_test >/dev/null 2>&1 && \
  docker exec "$task_redis" redis-cli ping >/dev/null 2>&1; do
  task_attempt=$((task_attempt + 1))
  if [ "$task_attempt" -ge 30 ]; then echo 'Test dependencies failed to start.' >&2; exit 1; fi
  sleep 1
done
task_pg_port=$(docker port "$task_pg" 5432/tcp | cut -d: -f2)
task_redis_port=$(docker port "$task_redis" 6379/tcp | cut -d: -f2)
export TEST_DATABASE_URL="postgres://noteflow:test-only-password@127.0.0.1:$task_pg_port/noteflow_test?sslmode=disable"
export TEST_REDIS_URL="redis://127.0.0.1:$task_redis_port/0"
export GIN_MODE=release
go test -race -count=1 -coverprofile=coverage.out ./...
