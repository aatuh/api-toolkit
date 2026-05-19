#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
result_dir="${GENERATED_UPGRADE_COMPAT_RESULT_DIR:-.ci-result/generated-upgrade-compat}"
log_path="$result_dir/upgrade-compat.log"
status_path="$result_dir/status"
generator_ref="${GENERATOR_REF:-v3.0.0}"
module_path="${GENERATED_UPGRADE_COMPAT_MODULE:-example.com/upgrade-api}"
profile="${GENERATED_UPGRADE_COMPAT_PROFILE:-saas-api-full}"
auth_mode="${GENERATED_UPGRADE_COMPAT_AUTH:-api-key}"

mkdir -p "$repo_root/$result_dir"
tmpdir="$(mktemp -d)"
generator_dir=""
cleanup() {
  if [ -n "$generator_dir" ]; then
    git -C "$repo_root" worktree remove --force "$generator_dir" >/dev/null 2>&1 || true
    git -C "$repo_root" worktree prune >/dev/null 2>&1 || true
  fi
  rm -rf "$tmpdir"
}
trap cleanup EXIT

service_dir="$tmpdir/upgrade-api"
status="failed"
if {
  generator_dir="$tmpdir/generator"
  git -C "$repo_root" worktree add --detach "$generator_dir" "$generator_ref"

  cd "$generator_dir/contrib"
  GOWORK=off GOTOOLCHAIN="${GOTOOLCHAIN:-local}" go run ./cmd/api-toolkit new service \
    --module "$module_path" \
    --profile "$profile" \
    --auth "$auth_mode" \
    --dir "$service_dir"

  cd "$service_dir"
  go mod edit -replace=github.com/aatuh/api-toolkit/v3="$repo_root"
  go mod edit -replace=github.com/aatuh/api-toolkit/contrib/v3="$repo_root/contrib"
  go mod tidy
  go test ./...
  make openapi-check
  make client-check
  make contracts-lint
  make contracts-diff
} >"$repo_root/$log_path" 2>&1; then
  status="passed"
fi

printf '%s\n' "$status" >"$repo_root/$status_path"
printf 'generated upgrade compatibility check %s; log=%s\n' "$status" "$log_path"
if [ "$status" != "passed" ]; then
  exit 1
fi
