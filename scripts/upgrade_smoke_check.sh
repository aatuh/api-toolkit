#!/usr/bin/env bash
set -euo pipefail

repo_root="${UPGRADE_SMOKE_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
result_dir="${UPGRADE_SMOKE_RESULT_DIR:-.ci-result/upgrade-smoke}"
base_ref="${UPGRADE_SMOKE_BASE_REF:-v3.1.2}"
module_path="${UPGRADE_SMOKE_MODULE:-example.com/api-toolkit-upgrade-smoke}"
gotoolchain="${GOTOOLCHAIN:-local}"
root_core_fixture="$repo_root/internal/compatfixtures/rootcore/upgrade_smoke_test.go"
status_path="$repo_root/$result_dir/status"
status_tsv="$repo_root/$result_dir/status.tsv"

mkdir -p "$repo_root/$result_dir"
: >"$status_tsv"

safe_ref_name() {
  printf '%s' "$1" | tr -c 'A-Za-z0-9._-' '-'
}

write_root_core_fixture() {
  local path="$1"

  if [ ! -f "$root_core_fixture" ]; then
    printf 'missing root core compatibility fixture: %s\n' "$root_core_fixture" >&2
    return 2
  fi
  cp "$root_core_fixture" "$path/upgrade_smoke_test.go"
}

run_root_core_fixture() {
  local safe_ref
  local tmpdir
  local fixture_dir
  local log_path

  safe_ref="$(safe_ref_name "$base_ref")"
  tmpdir="$(mktemp -d)"
  fixture_dir="$tmpdir/root-core"
  log_path="$result_dir/root-core-$safe_ref.log"
  cleanup() {
    rm -rf "$tmpdir"
  }
  trap cleanup RETURN

  mkdir -p "$fixture_dir"
  write_root_core_fixture "$fixture_dir"

  if {
    cd "$fixture_dir"
    GOWORK=off GOTOOLCHAIN="$gotoolchain" go mod init "$module_path"
    GOWORK=off GOTOOLCHAIN="$gotoolchain" go get "github.com/aatuh/api-toolkit/v3@$base_ref"
    GOWORK=off GOTOOLCHAIN="$gotoolchain" go mod tidy
    GOWORK=off GOTOOLCHAIN="$gotoolchain" go test ./...
    go mod edit -replace=github.com/aatuh/api-toolkit/v3="$repo_root"
    GOWORK=off GOTOOLCHAIN="$gotoolchain" go mod tidy
    GOWORK=off GOTOOLCHAIN="$gotoolchain" go test ./...
  } >"$repo_root/$log_path" 2>&1; then
    printf 'root-core\t%s\tpassed\t%s\n' "$base_ref" "$log_path" >>"$status_tsv"
    printf 'upgrade smoke check passed for root-core from %s; log=%s\n' "$base_ref" "$log_path"
    return 0
  fi

  printf 'root-core\t%s\tfailed\t%s\n' "$base_ref" "$log_path" >>"$status_tsv"
  printf 'upgrade smoke check failed for root-core from %s; log=%s\n' "$base_ref" "$log_path" >&2
  return 1
}

status="passed"
if ! run_root_core_fixture; then
  status="failed"
fi

printf '%s\n' "$status" >"$status_path"
printf 'upgrade smoke matrix %s; status=%s\n' "$status" "$status_tsv"
if [ "$status" != "passed" ]; then
  exit 1
fi
