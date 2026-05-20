#!/usr/bin/env bash
set -euo pipefail

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required for integration-check" >&2
  exit 2
fi
if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required for integration-check" >&2
  exit 2
fi

read -r -a compose_cmd <<< "${COMPOSE:-docker compose}"

compose() {
  "${compose_cmd[@]}" "$@"
}

tmp_dir="$(mktemp -d)"
api_pid=""
worker_pid=""
receiver_pid=""

cleanup() {
  if [ -n "${api_pid}" ] && kill -0 "${api_pid}" 2>/dev/null; then
    kill "${api_pid}" 2>/dev/null || true
    wait "${api_pid}" 2>/dev/null || true
  fi
  if [ -n "${worker_pid}" ] && kill -0 "${worker_pid}" 2>/dev/null; then
    kill "${worker_pid}" 2>/dev/null || true
    wait "${worker_pid}" 2>/dev/null || true
  fi
  if [ -n "${receiver_pid}" ] && kill -0 "${receiver_pid}" 2>/dev/null; then
    kill "${receiver_pid}" 2>/dev/null || true
    wait "${receiver_pid}" 2>/dev/null || true
  fi
  # Default cleanup command: docker compose --profile objectstore down -v.
  compose --profile objectstore down -v
  rm -rf "${tmp_dir}"
}
trap cleanup EXIT

wait_for_postgres() {
  for _ in $(seq 1 60); do
    if compose exec -T postgres pg_isready -U api -d api >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "postgres did not become ready" >&2
  return 1
}

wait_for_redis() {
  for _ in $(seq 1 60); do
    if [ "$(compose exec -T redis redis-cli ping 2>/dev/null | tr -d '\r')" = "PONG" ]; then
      return 0
    fi
    sleep 1
  done
  echo "redis did not become ready" >&2
  return 1
}

wait_for_minio() {
  for _ in $(seq 1 60); do
    if curl -fsS "${S3_ENDPOINT}/minio/health/ready" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "minio did not become ready" >&2
  return 1
}

wait_for_http() {
  local url="$1"
  for _ in $(seq 1 90); do
    if curl -fsS "${url}/readyz" >/dev/null 2>&1; then
      return 0
    fi
    if [ -n "${api_pid}" ] && ! kill -0 "${api_pid}" 2>/dev/null; then
      echo "api process exited before readiness" >&2
      sed -n '1,120p' "${tmp_dir}/api.log" >&2 || true
      return 1
    fi
    sleep 1
  done
  echo "api did not become ready" >&2
  sed -n '1,120p' "${tmp_dir}/api.log" >&2 || true
  return 1
}

wait_for_receiver() {
  local url="$1"
  for _ in $(seq 1 30); do
    if curl -fsS "${url}/readyz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "webhook receiver did not become ready" >&2
  return 1
}

receiver_count() {
  local file="$1"
  if [ ! -f "${file}" ]; then
    printf '0\n'
    return 0
  fi
  wc -l <"${file}" | tr -d '[:space:]'
}

json_field() {
  local path="$1"
  local file="$2"
  python3 - "${path}" "${file}" <<'PY'
import json
import sys

path = sys.argv[1].split(".")
with open(sys.argv[2], "r", encoding="utf-8") as handle:
    data = json.load(handle)
for item in path:
    if isinstance(data, dict):
        data = data[item]
    elif isinstance(data, list):
        data = data[int(item)]
    else:
        raise KeyError(item)
if data is None:
    raise SystemExit(1)
print(data)
PY
}

header_value() {
  local name="$1"
  local file="$2"
  awk -v want="$(printf '%s' "${name}" | tr '[:upper:]' '[:lower:]')" '
    BEGIN { FS = ":" }
    {
      key = tolower($1)
      if (key == want) {
        sub(/^[^:]*:[ \t]*/, "", $0)
        sub(/\r$/, "", $0)
        print $0
        exit
      }
    }
  ' "${file}"
}

psql_exec() {
  local sql="$1"
  shift
  compose exec -T postgres psql -v ON_ERROR_STOP=1 -U api -d api "$@" <<<"${sql}"
}

psql_scalar() {
  local sql="$1"
  shift
  compose exec -T postgres psql -v ON_ERROR_STOP=1 -tA -U api -d api "$@" <<<"${sql}" | tr -d '[:space:]'
}

export ENV=integration
export API_ADDR="${INTEGRATION_API_ADDR:-127.0.0.1:18080}"
export ADMIN_ADDR="${INTEGRATION_ADMIN_ADDR:-127.0.0.1:19090}"
default_db_user="${POSTGRES_USER:-api}"
default_db_password="${POSTGRES_PASSWORD:-api}"
export DATABASE_URL="${DATABASE_URL:-postgres://${default_db_user}:${default_db_password}@localhost:5432/api?sslmode=disable}"
export REDIS_ADDR="${REDIS_ADDR:-localhost:6379}"
export CACHE_STORE="${CACHE_STORE:-redis}"
export RATE_LIMIT_STORE="${RATE_LIMIT_STORE:-redis}"
export IDEMPOTENCY_STORE="${IDEMPOTENCY_STORE:-redis}"
export API_KEY="${API_KEY:-local-dev-key}"
export ADMIN_KEY="${ADMIN_KEY:-local-admin-key}"
export API_ACTOR_ID="${API_ACTOR_ID:-integration-actor}"
export API_KEY_PEPPER="${API_KEY_PEPPER:-integration-pepper-change-me}"
export WEBHOOK_SECRET_KEY="${WEBHOOK_SECRET_KEY:-local-webhook-secret-key-1234567}"
export OBJECT_STORE="${INTEGRATION_OBJECT_STORE:-${OBJECT_STORE:-memory}}"
export S3_ENDPOINT="${S3_ENDPOINT:-http://localhost:9000}"
export S3_REGION="${S3_REGION:-us-east-1}"
export S3_BUCKET="${S3_BUCKET:-api-objects}"
export S3_ACCESS_KEY_ID="${S3_ACCESS_KEY_ID:-minio}"
export S3_SECRET_ACCESS_KEY="${S3_SECRET_ACCESS_KEY:-minio123}"
export WEBHOOK_RECEIVER_ADDR="${WEBHOOK_RECEIVER_ADDR:-127.0.0.1:18081}"

api_url="http://${API_ADDR}"
admin_url="http://${ADMIN_ADDR}"
webhook_receiver_url="http://${WEBHOOK_RECEIVER_ADDR}"
webhook_receiver_log="${tmp_dir}/webhook-receiver.ndjson"

if [ ! -f .env ]; then
  cp .env.example .env
fi

if [ "${OBJECT_STORE}" = "s3" ]; then
  compose --profile objectstore up -d postgres redis minio
  wait_for_minio
  compose --profile objectstore run --rm minio-init
else
  compose up -d postgres redis
fi
wait_for_postgres
wait_for_redis

go mod tidy
go run ./cmd/migrate up
go run ./cmd/migrate check
go test ./...

python3 - "${WEBHOOK_RECEIVER_ADDR}" "${webhook_receiver_log}" <<'PY' >"${tmp_dir}/webhook-receiver.log" 2>&1 &
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

addr = sys.argv[1]
log_path = sys.argv[2]
host, port = addr.rsplit(":", 1)

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/readyz":
            self.send_response(204)
            self.end_headers()
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length).decode("utf-8", "replace")
        record = {
            "path": self.path,
            "headers": {key: value for key, value in self.headers.items()},
            "body": body,
        }
        with open(log_path, "a", encoding="utf-8") as handle:
            handle.write(json.dumps(record, sort_keys=True) + "\n")
        if self.path.startswith("/fail"):
            self.send_response(500)
            self.end_headers()
            self.wfile.write(b"retry")
            return
        self.send_response(204)
        self.end_headers()

    def log_message(self, *_):
        return

ThreadingHTTPServer((host, int(port)), Handler).serve_forever()
PY
receiver_pid="$!"
wait_for_receiver "${webhook_receiver_url}"

export ASYNC_WORKER_ENABLED=false
go run ./cmd/worker >"${tmp_dir}/worker.log" 2>&1 &
worker_pid="$!"
sleep 1
if ! kill -0 "${worker_pid}" 2>/dev/null; then
  echo "worker process exited before smoke checks" >&2
  sed -n '1,120p' "${tmp_dir}/worker.log" >&2 || true
  exit 1
fi

go run ./cmd/api >"${tmp_dir}/api.log" 2>&1 &
api_pid="$!"
wait_for_http "${api_url}"

curl -fsS "${api_url}/livez" >/dev/null
curl -fsS "${api_url}/docs/openapi.json" >/dev/null

auth_status="$(curl -sS -o "${tmp_dir}/auth.json" -w '%{http_code}' "${api_url}/organizations")"
if [ "${auth_status}" != "401" ]; then
  echo "expected unauthenticated organization request to return 401, got ${auth_status}" >&2
  sed -n '1,80p' "${tmp_dir}/auth.json" >&2 || true
  exit 1
fi

org_json="$(curl -fsS -X POST "${api_url}/organizations" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "Idempotency-Key: integration-create-organization" \
  --data '{"name":"Integration"}')"
printf '%s' "${org_json}" >"${tmp_dir}/organization.json"
org_id="$(json_field id "${tmp_dir}/organization.json")"
if [ -z "${org_id}" ]; then
  echo "create organization response did not include id" >&2
  exit 1
fi

poison_outbox_id="integration-poison-outbox"
psql_exec "insert into outbox_events (id, organization_id, event_type, payload, state, next_at, created_at) values (:'outbox_id', :'organization_id', 'integration.poison', '{}'::jsonb, 'pending', now(), now()) on conflict (id) do update set state='pending', lease_owner=null, lease_expires_at=null, retry_count=0, next_at=now();" \
  -v organization_id="${org_id}" \
  -v outbox_id="${poison_outbox_id}" >/dev/null
poison_deadletter_outbox_id="integration-poison-outbox-deadletter"
psql_exec "insert into outbox_events (id, organization_id, event_type, payload, state, retry_count, next_at, created_at) values (:'outbox_id', :'organization_id', 'integration.poison.deadletter', '{}'::jsonb, 'pending', 9, now(), now()) on conflict (id) do update set state='pending', lease_owner=null, lease_expires_at=null, retry_count=9, next_at=now();" \
  -v organization_id="${org_id}" \
  -v outbox_id="${poison_deadletter_outbox_id}" >/dev/null

curl -fsS "${api_url}/organizations/${org_id}/members" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "X-Tenant-ID: ${org_id}" >/dev/null

api_key_json="$(curl -fsS -X POST "${api_url}/organizations/${org_id}/api-keys" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "Idempotency-Key: integration-create-managed-api-key" \
  --data '{"name":"Integration","scopes":["widgets:read","widgets:write","operations:read"]}')"
printf '%s' "${api_key_json}" >"${tmp_dir}/api-key.json"
managed_api_key="$(json_field secret "${tmp_dir}/api-key.json")"
if [ -z "${managed_api_key}" ]; then
  echo "create API key response did not include secret" >&2
  exit 1
fi

webhook_endpoint_body="${tmp_dir}/webhook-endpoint.json"
curl -fsS -X POST "${api_url}/organizations/${org_id}/webhook-endpoints" \
  -o "${webhook_endpoint_body}" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "Idempotency-Key: integration-create-webhook-endpoint" \
  --data '{"url":"'"${webhook_receiver_url}"'/webhooks/widgets","events":["widget.created"]}'
webhook_secret="$(json_field secret "${webhook_endpoint_body}")"
if [ -z "${webhook_secret}" ]; then
  echo "create webhook endpoint response did not include signing secret" >&2
  exit 1
fi

curl -fsS "${api_url}/organizations/${org_id}/webhook-endpoints" \
  -o "${tmp_dir}/webhook-endpoints.json" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "X-Tenant-ID: ${org_id}" >/dev/null
if grep -F -q -- "${webhook_secret}" "${tmp_dir}/webhook-endpoints.json"; then
  echo "webhook endpoint list leaked signing secret" >&2
  exit 1
fi

webhook_failure_endpoint_body="${tmp_dir}/webhook-failure-endpoint.json"
curl -fsS -X POST "${api_url}/organizations/${org_id}/webhook-endpoints" \
  -o "${webhook_failure_endpoint_body}" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "Idempotency-Key: integration-create-failing-webhook-endpoint" \
  --data '{"url":"'"${webhook_receiver_url}"'/fail/widgets","events":["widget.updated"]}'
failure_endpoint_id="$(json_field endpoint.id "${webhook_failure_endpoint_body}")"
if [ -z "${failure_endpoint_id}" ]; then
  echo "create failing webhook endpoint response did not include id" >&2
  exit 1
fi

widget_headers="${tmp_dir}/widget-create.headers"
widget_body="${tmp_dir}/widget-create.json"
curl -fsS -X POST "${api_url}/widgets" \
  -D "${widget_headers}" \
  -o "${widget_body}" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${managed_api_key}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "Idempotency-Key: integration-managed-key-widget" \
  --data '{"name":"managed-key-widget"}'
widget_id="$(json_field id "${widget_body}")"
widget_etag="$(header_value ETag "${widget_headers}")"
if [ -z "${widget_id}" ] || [ -z "${widget_etag}" ]; then
  echo "create widget response did not include id and ETag" >&2
  exit 1
fi

replay_headers="${tmp_dir}/widget-replay.headers"
replay_body="${tmp_dir}/widget-replay.json"
replay_status="$(curl -sS -D "${replay_headers}" -o "${replay_body}" -w '%{http_code}' -X POST "${api_url}/widgets" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${managed_api_key}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "Idempotency-Key: integration-managed-key-widget" \
  --data '{"name":"managed-key-widget"}')"
replay_id="$(json_field id "${replay_body}")"
replay_header="$(header_value Idempotency-Replayed "${replay_headers}")"
if [ "${replay_status}" != "201" ] || [ "${replay_id}" != "${widget_id}" ] || [ "${replay_header}" != "true" ]; then
  echo "expected idempotent widget replay, got status ${replay_status}, id ${replay_id}, replay header ${replay_header}" >&2
  exit 1
fi

delivery_id=""
delivery_state=""
for _ in $(seq 1 30); do
  curl -fsS "${api_url}/organizations/${org_id}/webhook-deliveries" \
    -o "${tmp_dir}/webhook-deliveries.json" \
    -H "X-API-Key: ${API_KEY}" \
    -H "X-Actor-ID: ${API_ACTOR_ID}" \
    -H "X-Tenant-ID: ${org_id}" >/dev/null
  delivery_id="$(json_field items.0.id "${tmp_dir}/webhook-deliveries.json" 2>/dev/null || true)"
  delivery_state="$(psql_scalar "select state from webhook_deliveries where organization_id = :'organization_id' and id = :'delivery_id' order by created_at desc limit 1;" \
    -v organization_id="${org_id}" \
    -v delivery_id="${delivery_id}")"
  if [ -n "${delivery_id}" ] && [ "${delivery_state}" = "succeeded" ] && [ "$(receiver_count "${webhook_receiver_log}")" -ge 1 ]; then
    break
  fi
  sleep 1
done
if [ -z "${delivery_id}" ]; then
  echo "widget create did not enqueue webhook delivery" >&2
  exit 1
fi
if [ "${delivery_state}" != "succeeded" ]; then
  echo "webhook delivery did not succeed, last state ${delivery_state}" >&2
  sed -n '1,80p' "${tmp_dir}/webhook-deliveries.json" >&2 || true
  sed -n '1,80p' "${tmp_dir}/worker.log" >&2 || true
  exit 1
fi
if grep -F -q -- "${webhook_secret}" "${tmp_dir}/webhook-deliveries.json"; then
  echo "webhook delivery list leaked signing secret" >&2
  exit 1
fi
if grep -F -q -- "${webhook_secret}" "${webhook_receiver_log}"; then
  echo "webhook receiver log leaked signing secret" >&2
  exit 1
fi

curl -fsS -X POST "${api_url}/organizations/${org_id}/webhook-deliveries/${delivery_id}/replay" \
  -o "${tmp_dir}/webhook-replay.json" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "Idempotency-Key: integration-replay-webhook-delivery" \
  --data '{}'
replayed_delivery_id="$(json_field id "${tmp_dir}/webhook-replay.json")"
if [ "${replayed_delivery_id}" != "${delivery_id}" ]; then
  echo "replay webhook delivery returned ${replayed_delivery_id}, want ${delivery_id}" >&2
  exit 1
fi
for _ in $(seq 1 30); do
  if [ "$(receiver_count "${webhook_receiver_log}")" -ge 2 ]; then
    break
  fi
  sleep 1
done
if [ "$(receiver_count "${webhook_receiver_log}")" -lt 2 ]; then
  echo "replay webhook delivery did not reach local receiver" >&2
  sed -n '1,80p' "${webhook_receiver_log}" >&2 || true
  exit 1
fi

precondition_status="$(curl -sS -o "${tmp_dir}/widget-stale.json" -w '%{http_code}' -X PATCH "${api_url}/widgets/${widget_id}" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${managed_api_key}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "If-Match: stale-etag" \
  -H "Idempotency-Key: integration-widget-stale-etag" \
  --data '{"name":"stale-update"}')"
if [ "${precondition_status}" != "412" ]; then
  echo "expected stale widget update to return 412, got ${precondition_status}" >&2
  sed -n '1,80p' "${tmp_dir}/widget-stale.json" >&2 || true
  exit 1
fi

curl -fsS -X PATCH "${api_url}/widgets/${widget_id}" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${managed_api_key}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "If-Match: ${widget_etag}" \
  -H "Idempotency-Key: integration-widget-update" \
  --data '{"name":"managed-key-widget-updated"}' >/dev/null

failure_delivery_state=""
failure_retry_count=""
for _ in $(seq 1 30); do
  failure_delivery_state="$(psql_scalar "select state from webhook_deliveries where organization_id = :'organization_id' and endpoint_id = :'endpoint_id' order by created_at desc limit 1;" \
    -v organization_id="${org_id}" \
    -v endpoint_id="${failure_endpoint_id}")"
  failure_retry_count="$(psql_scalar "select retry_count from outbox_events where organization_id = :'organization_id' and event_type = 'webhook.delivery' and payload->>'endpoint_id' = :'endpoint_id' order by created_at desc limit 1;" \
    -v organization_id="${org_id}" \
    -v endpoint_id="${failure_endpoint_id}")"
  if [ "${failure_delivery_state}" = "failed" ] && [ -n "${failure_retry_count}" ] && [ "${failure_retry_count}" -ge 1 ]; then
    break
  fi
  sleep 1
done
if [ "${failure_delivery_state}" != "failed" ] || [ -z "${failure_retry_count}" ] || [ "${failure_retry_count}" -lt 1 ]; then
  echo "failing webhook did not record retryable failure; state=${failure_delivery_state} retry_count=${failure_retry_count}" >&2
  sed -n '1,120p' "${tmp_dir}/worker.log" >&2 || true
  exit 1
fi

import_body="${tmp_dir}/widget-import.json"
curl -fsS -X POST "${api_url}/widgets/imports" \
  -o "${import_body}" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${managed_api_key}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "Idempotency-Key: integration-widget-import" \
  --data '{"items":[{"name":"imported-widget"}]}'
operation_id="$(json_field id "${import_body}")"
if [ -z "${operation_id}" ]; then
  echo "widget import response did not include operation id" >&2
  exit 1
fi
operation_state=""
for _ in $(seq 1 30); do
  curl -fsS "${api_url}/operations/${operation_id}" \
    -o "${tmp_dir}/operation.json" \
    -H "X-API-Key: ${managed_api_key}" \
    -H "X-Tenant-ID: ${org_id}"
  operation_state="$(json_field state "${tmp_dir}/operation.json")"
  if [ "${operation_state}" = "succeeded" ]; then
    break
  fi
  sleep 1
done
if [ "${operation_state}" != "succeeded" ]; then
  echo "operation did not succeed, last state ${operation_state}" >&2
  sed -n '1,80p' "${tmp_dir}/operation.json" >&2 || true
  exit 1
fi

operation_outbox_state=""
for _ in $(seq 1 30); do
  operation_outbox_state="$(psql_scalar "select state from outbox_events where organization_id = :'organization_id' and event_type = 'widgets.import' and payload->>'operation_id' = :'operation_id' order by created_at desc limit 1;" \
    -v organization_id="${org_id}" \
    -v operation_id="${operation_id}")"
  if [ "${operation_outbox_state}" = "succeeded" ]; then
    break
  fi
  sleep 1
done
if [ "${operation_outbox_state}" != "succeeded" ]; then
  echo "operation outbox did not complete, last state ${operation_outbox_state}" >&2
  exit 1
fi

poison_retry_count=""
for _ in $(seq 1 30); do
  poison_retry_count="$(psql_scalar "select retry_count from outbox_events where organization_id = :'organization_id' and id = :'outbox_id' and retry_count >= 1 order by retry_count desc limit 1;" \
    -v organization_id="${org_id}" \
    -v outbox_id="${poison_outbox_id}")"
  if [ -n "${poison_retry_count}" ]; then
    break
  fi
  sleep 1
done
case "${poison_retry_count}" in
  ""|*[!0-9]*)
    echo "outbox retry was not recorded" >&2
    exit 1
    ;;
esac

poison_deadletter_state=""
for _ in $(seq 1 30); do
  poison_deadletter_state="$(psql_scalar "select state from outbox_events where organization_id = :'organization_id' and id = :'outbox_id' order by created_at desc limit 1;" \
    -v organization_id="${org_id}" \
    -v outbox_id="${poison_deadletter_outbox_id}")"
  if [ "${poison_deadletter_state}" = "dead_letter" ]; then
    break
  fi
  sleep 1
done
if [ "${poison_deadletter_state}" != "dead_letter" ]; then
  echo "outbox dead-letter was not recorded, last state ${poison_deadletter_state}" >&2
  exit 1
fi

curl -fsS -X POST "${api_url}/organizations/${org_id}/objects" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "Idempotency-Key: integration-put-object" \
  --data '{"key":"integration.txt","content_type":"text/plain","content_base64":"aGVsbG8="}' >/dev/null

curl -fsS "${api_url}/organizations/${org_id}/objects/integration.txt" \
  -o "${tmp_dir}/object-get.json" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "X-Tenant-ID: ${org_id}" >/dev/null
object_content="$(json_field content_base64 "${tmp_dir}/object-get.json")"
if [ "${object_content}" != "aGVsbG8=" ]; then
  echo "object get did not return stored content" >&2
  exit 1
fi

audit_count="$(psql_scalar "select count(*) from audit_events where organization_id = :'organization_id';" \
  -v organization_id="${org_id}")"
case "${audit_count}" in
  ""|*[!0-9]*)
    echo "audit event count query returned ${audit_count}" >&2
    exit 1
    ;;
esac
if [ "${audit_count}" -lt 1 ]; then
  echo "audit events were not recorded" >&2
  exit 1
fi

curl -fsS "${admin_url}/health/detailed" \
  -H "X-Admin-Key: ${ADMIN_KEY}" >/dev/null

curl -fsS "${admin_url}/metrics" \
  -H "X-Admin-Key: ${ADMIN_KEY}" | grep -q "http_requests_total"

curl -fsS "${admin_url}/debug/pprof/" \
  -H "X-Admin-Key: ${ADMIN_KEY}" | grep -q "Types of profiles available"

metrics_status="$(curl -sS -o "${tmp_dir}/metrics-unauthorized.txt" -w '%{http_code}' "${admin_url}/metrics")"
if [ "${metrics_status}" != "401" ]; then
  echo "expected unauthenticated admin metrics request to return 401, got ${metrics_status}" >&2
  exit 1
fi

public_pprof_status="$(curl -sS -o "${tmp_dir}/public-pprof.html" -w '%{http_code}' "${api_url}/debug/pprof/")"
if [ "${public_pprof_status}" != "404" ]; then
  echo "public pprof endpoint should be isolated; got ${public_pprof_status}" >&2
  exit 1
fi

echo "integration-check passed"
