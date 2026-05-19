#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
result_dir="${GENERATED_INTEGRATION_RESULT_DIR:-.ci-result/generated-integration}"
log_path="$result_dir/integration-check.log"
status_path="$result_dir/status"
include_minio="${INCLUDE_MINIO:-false}"

mkdir -p "$repo_root/$result_dir"
tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

service_dir="$tmpdir/full-api"
status="failed"
if {
  cd "$repo_root/contrib"
  go run ./cmd/api-toolkit new service \
    --module example.com/full-api \
    --profile saas-api-full \
    --auth api-key \
    --dir "$service_dir" \
    --core-replace "$repo_root" \
    --contrib-replace "$repo_root/contrib"

  cd "$service_dir"
  go mod tidy
  go test ./...
  make contracts-lint
  make contracts-diff
  make openapi-check
  make client-check
  if [ "$include_minio" = "true" ]; then
    COMPOSE_PROFILES="${COMPOSE_PROFILES:-minio}" \
      ENABLE_MINIO_INTEGRATION="${ENABLE_MINIO_INTEGRATION:-1}" \
      INTEGRATION_OBJECT_STORE="${INTEGRATION_OBJECT_STORE:-s3}" \
      make integration-check
  else
    make integration-check
  fi
} >"$repo_root/$log_path" 2>&1; then
  status="passed"
fi

printf '%s\n' "$status" >"$repo_root/$status_path"
printf 'generated integration check %s; log=%s\n' "$status" "$log_path"
if [ "$status" != "passed" ]; then
  exit 1
fi
