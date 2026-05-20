#!/usr/bin/env bash
set -euo pipefail

repo_root="${REFERENCE_SERVICE_EVIDENCE_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
result_dir="${REFERENCE_SERVICE_EVIDENCE_RESULT_DIR:-.ci-result/reference-service}"
status_path="$result_dir/status"
summary_path="$result_dir/summary.json"
local_log="$result_dir/reference-service-check.log"
docker_log="$result_dir/integration-check.log"
docker_requested="${REFERENCE_SERVICE_DOCKER:-0}"
minio_requested="${REFERENCE_SERVICE_MINIO:-0}"

if [ "$minio_requested" = "1" ]; then
  docker_requested="1"
fi

mkdir -p "$repo_root/$result_dir"

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

run_logged() {
  local log="$1"
  shift
  if "$@" >"$repo_root/$log" 2>&1; then
    return 0
  fi
  return 1
}

commit="$(git -C "$repo_root" rev-parse HEAD 2>/dev/null || printf 'unknown')"
timestamp="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
toolchain="$(go version 2>/dev/null || printf 'unknown')"

overall_status="passed"
local_status="passed"
if ! run_logged "$local_log" env GOTOOLCHAIN="${GOTOOLCHAIN:-local}" GOWORK="${GOWORK:-off}" make -C "$repo_root" reference-service-check; then
  local_status="failed"
  overall_status="failed"
fi

docker_status="skipped"
if [ "$docker_requested" = "1" ]; then
  docker_status="passed"
  if [ "$minio_requested" = "1" ]; then
    docker_cmd=(env
      GOTOOLCHAIN="${GOTOOLCHAIN:-local}"
      GOWORK="${GOWORK:-off}"
      COMPOSE_PROFILES="${COMPOSE_PROFILES:-minio}"
      ENABLE_MINIO_INTEGRATION="${ENABLE_MINIO_INTEGRATION:-1}"
      INTEGRATION_OBJECT_STORE="${INTEGRATION_OBJECT_STORE:-s3}"
      make -C "$repo_root/examples/reference-saas-api" integration-check)
  else
    docker_cmd=(env
      GOTOOLCHAIN="${GOTOOLCHAIN:-local}"
      GOWORK="${GOWORK:-off}"
      make -C "$repo_root/examples/reference-saas-api" integration-check)
  fi
  if ! run_logged "$docker_log" "${docker_cmd[@]}"; then
    docker_status="failed"
    overall_status="failed"
  fi
fi

printf '%s\n' "$overall_status" >"$repo_root/$status_path"
cat >"$repo_root/$summary_path" <<JSON
{
  "status": "$(json_escape "$overall_status")",
  "commit": "$(json_escape "$commit")",
  "timestamp": "$(json_escape "$timestamp")",
  "toolchain": "$(json_escape "$toolchain")",
  "reference_service_check": {
    "status": "$(json_escape "$local_status")",
    "log_path": "$(json_escape "$local_log")"
  },
  "docker_integration": {
    "requested": $(json_bool "$docker_requested"),
    "minio": $(json_bool "$minio_requested"),
    "status": "$(json_escape "$docker_status")",
    "log_path": "$(json_escape "$docker_log")"
  }
}
JSON

printf 'reference service evidence %s; summary=%s\n' "$overall_status" "$summary_path"
if [ "$overall_status" != "passed" ]; then
  exit 1
fi
