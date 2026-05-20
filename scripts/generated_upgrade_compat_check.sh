#!/usr/bin/env bash
set -euo pipefail

repo_root="${GENERATED_UPGRADE_COMPAT_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
result_dir="${GENERATED_UPGRADE_COMPAT_RESULT_DIR:-.ci-result/generated-upgrade-compat}"
status_path="$result_dir/status"
module_path="${GENERATED_UPGRADE_COMPAT_MODULE:-example.com/upgrade-api}"
profile="${GENERATED_UPGRADE_COMPAT_PROFILE:-saas-api-full}"
auth_mode="${GENERATED_UPGRADE_COMPAT_AUTH:-api-key}"
if [ -n "${GENERATED_UPGRADE_COMPAT_REFS:-}" ]; then
  generator_refs="$GENERATED_UPGRADE_COMPAT_REFS"
elif [ -n "${GENERATOR_REF:-}" ]; then
  generator_refs="$GENERATOR_REF"
else
  generator_refs="v3.0.0 v3.1.0"
fi

mkdir -p "$repo_root/$result_dir"
status="failed"
status_tsv="$repo_root/$result_dir/status.tsv"
: >"$status_tsv"

safe_ref_name() {
  printf '%s' "$1" | tr -c 'A-Za-z0-9._-' '-'
}

run_ref() {
  local generator_ref="$1"
  local safe_ref
  local tmpdir
  local generator_dir=""
  local service_dir
  local log_path

  safe_ref="$(safe_ref_name "$generator_ref")"
  log_path="$result_dir/upgrade-compat-$safe_ref.log"
  tmpdir="$(mktemp -d)"
  service_dir="$tmpdir/upgrade-api"
  cleanup_ref() {
    if [ -n "$generator_dir" ]; then
      git -C "$repo_root" worktree remove --force "$generator_dir" >/dev/null 2>&1 || true
      git -C "$repo_root" worktree prune >/dev/null 2>&1 || true
    fi
    rm -rf "$tmpdir"
  }
  trap cleanup_ref RETURN

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
    printf '%s\tpassed\t%s\n' "$generator_ref" "$log_path" >>"$status_tsv"
    printf 'generated upgrade compatibility check passed for %s; log=%s\n' "$generator_ref" "$log_path"
    return 0
  fi

  printf '%s\tfailed\t%s\n' "$generator_ref" "$log_path" >>"$status_tsv"
  printf 'generated upgrade compatibility check failed for %s; log=%s\n' "$generator_ref" "$log_path" >&2
  return 1
}

status="passed"
for generator_ref in $generator_refs; do
  if ! run_ref "$generator_ref"; then
    status="failed"
  fi
done

printf '%s\n' "$status" >"$repo_root/$status_path"
printf 'generated upgrade compatibility matrix %s; status=%s\n' "$status" "$result_dir/status.tsv"
if [ "$status" != "passed" ]; then
  exit 1
fi
