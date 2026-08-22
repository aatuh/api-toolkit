#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
test_enable="API_TOOLKIT_TEST_POSTGRES"
test_dsn="API_TOOLKIT_TEST_POSTGRES_DSN"
test_user="api_toolkit_test"
test_password="api_toolkit_test"
container="api-toolkit-test-postgres-$$"

run_tests() {
  (
    cd "$repo_root/contrib"
    env \
      GOWORK=off \
      "${test_enable}=1" \
      GOTOOLCHAIN="${GOTOOLCHAIN:-local}" \
      go test ./internal/testpostgres -count=1
  )
}

if [ -n "${!test_dsn:-}" ]; then
  if [ "${!test_enable:-}" != "1" ]; then
    echo "${test_enable}=1 is required when ${test_dsn} is set" >&2
    exit 2
  fi
  run_tests
  exit 0
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required for make test-postgres when ${test_dsn} is not set" >&2
  exit 2
fi

cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

if ! docker run --rm -d \
  --name "$container" \
  --label "com.api-toolkit.test-postgres=true" \
  -e "POSTGRES_USER=$test_user" \
  -e "POSTGRES_PASSWORD=$test_password" \
  -e "POSTGRES_DB=postgres" \
  -p 127.0.0.1::5432 \
  postgres:18-alpine >/dev/null; then
  echo "start isolated PostgreSQL test container" >&2
  exit 1
fi

ready=false
for _ in $(seq 1 60); do
  if docker exec "$container" pg_isready -U "$test_user" -d postgres >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 0.2
done
if [ "$ready" != "true" ]; then
  echo "isolated PostgreSQL test container did not become ready" >&2
  exit 1
fi

binding="$(docker port "$container" 5432/tcp | head -n 1)"
port="${binding##*:}"
case "$port" in
  ""|*[!0-9]*)
    echo "isolated PostgreSQL test container did not publish a numeric loopback port" >&2
    exit 1
    ;;
esac

export "${test_enable}=1"
export "${test_dsn}=postgres://${test_user}:${test_password}@127.0.0.1:${port}/postgres?sslmode=disable"
run_tests
