#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
test_enable="API_TOOLKIT_TEST_REDIS"
test_url="API_TOOLKIT_TEST_REDIS_URL"
container="api-toolkit-test-redis-$$"

run_tests() {
  (
    cd "$repo_root/contrib"
    env -u REDIS_URL -u REDIS_ADDR \
      GOWORK=off \
      "${test_enable}=1" \
      GOTOOLCHAIN="${GOTOOLCHAIN:-local}" \
      go test ./testredis -count=1
  )
  (
    cd "$repo_root/examples/reference-saas-api"
    env -u REDIS_URL -u REDIS_ADDR \
      GOWORK=off \
      "${test_enable}=1" \
      GOTOOLCHAIN="${GOTOOLCHAIN:-local}" \
      go test ./internal/adapters/redis -count=1 -run '^TestRealRedis'
  )
}

if [ -n "${!test_url:-}" ]; then
  if [ "${!test_enable:-}" != "1" ]; then
    echo "${test_enable}=1 is required when ${test_url} is set" >&2
    exit 2
  fi
  run_tests
  exit 0
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required for make test-redis when ${test_url} is not set" >&2
  exit 2
fi

cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

if ! docker run --rm -d \
  --name "$container" \
  --label "com.api-toolkit.test-redis=true" \
  -p 127.0.0.1::6379 \
  redis:7-alpine >/dev/null; then
  echo "start isolated Redis test container" >&2
  exit 1
fi

ready=false
for _ in $(seq 1 60); do
  if docker exec "$container" redis-cli ping 2>/dev/null | grep -qx PONG; then
    ready=true
    break
  fi
  sleep 0.2
done
if [ "$ready" != "true" ]; then
  echo "isolated Redis test container did not become ready" >&2
  exit 1
fi

binding="$(docker port "$container" 6379/tcp | head -n 1)"
port="${binding##*:}"
case "$port" in
  ""|*[!0-9]*)
    echo "isolated Redis test container did not publish a numeric loopback port" >&2
    exit 1
    ;;
esac

export "${test_enable}=1"
export "${test_url}=redis://127.0.0.1:${port}/15"
run_tests
