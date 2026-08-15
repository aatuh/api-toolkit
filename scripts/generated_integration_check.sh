#!/usr/bin/env bash
set -euo pipefail

repo_root="${GENERATED_INTEGRATION_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
result_dir="${GENERATED_INTEGRATION_RESULT_DIR:-.ci-result/generated-integration}"
case "$result_dir" in
  ""|/*|..|../*|*/..|*/../*)
    echo "GENERATED_INTEGRATION_RESULT_DIR must be a repo-relative path without .. components" >&2
    exit 2
    ;;
esac
log_path="$result_dir/integration-check.log"
status_path="$result_dir/status"
summary_path="$result_dir/summary.json"
include_minio="${INCLUDE_MINIO:-false}"
postgres_host_port="${GENERATED_INTEGRATION_POSTGRES_PORT:-55432}"
provider_workflows="${GENERATED_INTEGRATION_PROVIDERS:-stripe-billing,resend-email,clerk-webhooks,entitlements}"
provider_args=()
provider_names=()

mkdir -p "$repo_root/$result_dir"
tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

service_dir="$tmpdir/full-api"
status="failed"

json_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '%s' "$value"
}

json_bool() {
  case "$1" in
    1|true|TRUE|yes|YES) printf 'true' ;;
    *) printf 'false' ;;
  esac
}

json_string() {
  printf '"%s"' "$(json_escape "$1")"
}

json_string_array() {
  local first=true
  printf '['
  for value in "$@"; do
    if [ "$first" = true ]; then
      first=false
    else
      printf ','
    fi
    json_string "$value"
  done
  printf ']'
}

write_summary() {
  local commit timestamp toolchain
  commit="$(git -C "$repo_root" rev-parse HEAD 2>/dev/null || printf 'unknown')"
  timestamp="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  toolchain="$(go version 2>/dev/null || printf 'unknown')"

  cat >"$repo_root/$summary_path" <<JSON
{
  "status": "$(json_escape "$status")",
  "commit": "$(json_escape "$commit")",
  "timestamp": "$(json_escape "$timestamp")",
  "toolchain": "$(json_escape "$toolchain")",
  "profile": "saas-api-full",
  "auth": "api-key",
  "provider_workflows": $(json_string_array "${provider_names[@]}"),
  "minio": $(json_bool "$include_minio"),
  "log_path": "$(json_escape "$log_path")",
  "runtime_contract": {
    "generated_checks": $(json_string_array "go mod tidy" "go test ./..." "make build" "make contracts-lint" "make contracts-diff" "make openapi-check" "make client-check"),
    "docker_runtime_checks": $(json_string_array "make integration-check" "GET /livez" "GET /readyz" "GET /docs/openapi.json" "POST /organizations" "POST /widgets" "Idempotency-Replayed")
  }
}
JSON
}

run_check() {
  provider_args=()
  if [ -n "$provider_workflows" ]; then
    local workflow
    local old_ifs="$IFS"
    IFS=','
    for workflow in $provider_workflows; do
      workflow="$(printf '%s' "$workflow" | tr -d '[:space:]')"
      case "$workflow" in
        stripe-billing|resend-email|clerk-webhooks|entitlements)
          provider_args+=("--with" "$workflow")
          provider_names+=("$workflow")
          ;;
        *)
          IFS="$old_ifs"
          echo "GENERATED_INTEGRATION_PROVIDERS contains unsupported workflow $workflow" >&2
          return 2
          ;;
      esac
    done
    IFS="$old_ifs"
  fi
  local cli_binary="$tmpdir/api-toolkit"
  (cd "$repo_root/cmd/api-toolkit" && GOWORK=off go build -o "$cli_binary" .) || return
  export API_TOOLKIT="$cli_binary"
  cd "$repo_root/cmd/api-toolkit"
  go run . new service \
    --module example.com/full-api \
    --profile saas-api-full \
    --auth api-key \
    "${provider_args[@]}" \
    --dir "$service_dir" \
    --core-replace "$repo_root" \
    --contrib-replace "$repo_root/contrib" || return

  cd "$service_dir"
  go mod tidy || return
  go test ./... || return
  make build || return
  make contracts-lint || return
  make contracts-diff || return
  make openapi-check || return
  make client-check || return
  make provider-check || return
  if [ "$include_minio" = "true" ]; then
    COMPOSE_PROFILES="${COMPOSE_PROFILES:-minio}" \
      ENABLE_MINIO_INTEGRATION="${ENABLE_MINIO_INTEGRATION:-1}" \
      INTEGRATION_OBJECT_STORE="${INTEGRATION_OBJECT_STORE:-s3}" \
      POSTGRES_HOST_PORT="$postgres_host_port" DATABASE_URL="postgres://api:api@localhost:${postgres_host_port}/api?sslmode=disable" make integration-check || return
  else
    POSTGRES_HOST_PORT="$postgres_host_port" DATABASE_URL="postgres://api:api@localhost:${postgres_host_port}/api?sslmode=disable" make integration-check || return
  fi
}

if run_check >"$repo_root/$log_path" 2>&1; then
  status="passed"
fi

printf '%s\n' "$status" >"$repo_root/$status_path"
write_summary
printf 'generated integration check %s; log=%s\n' "$status" "$log_path"
if [ "$status" != "passed" ]; then
  exit 1
fi
