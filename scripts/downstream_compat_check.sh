#!/usr/bin/env bash
set -euo pipefail

# Verify published module consumers before testing the same consumers against
# this checkout. Fixtures are copied to a fresh temporary module so neither
# go.work nor a developer's local replace directive can make the release phase
# pass accidentally.

repo_root="$(cd "$(git rev-parse --show-toplevel 2>/dev/null || pwd)" && pwd -P)"
result_dir="${DOWNSTREAM_COMPAT_RESULT_DIR:-.ci-result/downstream-compat}"
base_refs_csv="${DOWNSTREAM_COMPAT_BASE_REFS:-}"
module_path="example.com/api-toolkit-downstream-compat"
gotoolchain="${GOTOOLCHAIN:-local}"

case "$result_dir" in
  ""|/*|*"/../"*|../*|*/..)
    printf 'DOWNSTREAM_COMPAT_RESULT_DIR must be a non-empty relative path without parent traversal\n' >&2
    exit 2
    ;;
esac

mkdir -p "$repo_root/$result_dir"
result_root="$(cd "$repo_root/$result_dir" && pwd -P)"
case "$result_root/" in
  "$repo_root/"*) ;;
  *)
    printf 'DOWNSTREAM_COMPAT_RESULT_DIR resolved outside the repository\n' >&2
    exit 2
    ;;
esac

status_path="$result_root/status"
status_tsv="$result_root/status.tsv"
: >"$status_tsv"

safe_ref_name() {
  printf '%s' "$1" | tr -c 'A-Za-z0-9._-' '-'
}

validate_ref() {
  [[ "$1" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-.][A-Za-z0-9._-]+)?$ ]]
}

split_refs() {
  local raw_ref
  IFS=',' read -r -a base_refs <<<"$base_refs_csv"
  if [ -z "$base_refs_csv" ] || [ "${#base_refs[@]}" -eq 0 ]; then
    printf 'DOWNSTREAM_COMPAT_BASE_REFS must name at least one verified released version tag\n' >&2
    exit 2
  fi
  for raw_ref in "${base_refs[@]}"; do
    if ! validate_ref "$raw_ref"; then
      printf 'invalid released version tag in DOWNSTREAM_COMPAT_BASE_REFS: %q\n' "$raw_ref" >&2
      exit 2
    fi
  done
}

fixture_dir() {
  printf '%s/internal/compatfixtures/%s' "$repo_root" "$1"
}

copy_fixture() {
  local name="$1"
  local destination="$2"
  local source
  source="$(fixture_dir "$name")"
  if [ ! -d "$source" ] || [ ! -f "$source/upgrade_smoke_test.go" ]; then
    printf 'missing downstream fixture: %s\n' "$source" >&2
    return 2
  fi
  mkdir -p "$destination"
  cp "$source"/*.go "$destination/"
  printf '%s\n' 'package upgradesmoke' >"$destination/consumer.go"
}

run_go() {
  GOWORK=off GOTOOLCHAIN="$gotoolchain" go "$@"
}

run_fixture_phase() {
  local fixture="$1"
  local ref="$2"
  local phase="$3"
  local modules="$4"
  local extra_modules="$5"
  local tmpdir="$6"
  local consumer_dir="$tmpdir/$fixture-$phase"

  copy_fixture "$fixture" "$consumer_dir"
  (
    cd "$consumer_dir"
    run_go mod init "$module_path/$fixture" || return $?
    if [ "$phase" = "released" ]; then
      run_go get "github.com/aatuh/api-toolkit/v4@$ref" || return $?
      if [[ "$modules" == *contrib* ]]; then
        run_go get "github.com/aatuh/api-toolkit/contrib/v4@$ref" || return $?
      fi
    else
      GOWORK=off go mod edit -require="github.com/aatuh/api-toolkit/v4@$ref" || return $?
      if [[ "$modules" == *contrib* ]]; then
        GOWORK=off go mod edit -require="github.com/aatuh/api-toolkit/contrib/v4@$ref" || return $?
      fi
    fi
    if [ -n "$extra_modules" ]; then
      local dependency
      IFS=' ' read -r -a dependencies <<<"$extra_modules"
      for dependency in "${dependencies[@]}"; do
        run_go get "$dependency" || return $?
      done
    fi
    if [ "$phase" = "candidate" ]; then
      GOWORK=off go mod edit -replace=github.com/aatuh/api-toolkit/v4="$repo_root" || return $?
      if [[ "$modules" == *contrib* ]]; then
        GOWORK=off go mod edit -replace=github.com/aatuh/api-toolkit/contrib/v4="$repo_root/contrib" || return $?
      fi
    elif grep -Eq '^[[:space:]]*replace[[:space:]]+' go.mod; then
      printf 'release verification unexpectedly contains a replace directive\n' >&2
      return 1
    fi
    run_go mod tidy || return $?
    run_go test -tags=downstreamcompat ./... || return $?
  )
}

run_standard_fixture() {
  local fixture="$1"
  local ref="$2"
  local modules="$3"
  local extra_modules="$4"
  local safe_ref tmpdir log_path phase fixture_status
  safe_ref="$(safe_ref_name "$ref")"
  tmpdir="$(mktemp -d)"
  fixture_status=0

  for phase in released candidate; do
    log_path="$result_root/$fixture-$safe_ref-$phase.log"
    if run_fixture_phase "$fixture" "$ref" "$phase" "$modules" "$extra_modules" "$tmpdir" >"$log_path" 2>&1; then
      printf '%s\t%s\t%s\tpassed\t%s\n' "$fixture" "$ref" "$phase" "$log_path" >>"$status_tsv"
    else
      printf '%s\t%s\t%s\tfailed\t%s\n' "$fixture" "$ref" "$phase" "$log_path" >>"$status_tsv"
      printf 'downstream fixture failed: fixture=%s ref=%s phase=%s log=%s\n' "$fixture" "$ref" "$phase" "$log_path" >&2
      fixture_status=1
    fi
  done
  rm -rf -- "$tmpdir"
  return "$fixture_status"
}

run_cli_phase() {
  local ref="$1"
  local phase="$2"
  local tmpdir="$3"
  local generated_dir="$tmpdir/cli-$phase/service"

  mkdir -p "$(dirname "$generated_dir")"
  if [ "$phase" = "released" ]; then
    (
      cd "$tmpdir/cli-$phase"
      run_go run "github.com/aatuh/api-toolkit/contrib/v4/cmd/api-toolkit@$ref" new service \
        --module example.com/generated-downstream --dir "$generated_dir" --profile saas-api --auth api-key
    ) || return $?
  else
    (
      cd "$repo_root/contrib"
      GOWORK=off GOTOOLCHAIN="$gotoolchain" go run ./cmd/api-toolkit new service \
        --module example.com/generated-downstream --dir "$generated_dir" --profile saas-api --auth api-key \
        --core-replace "$repo_root" --contrib-replace "$repo_root/contrib"
    ) || return $?
  fi
  (
    cd "$generated_dir"
    run_go mod tidy || return $?
    run_go test ./... || return $?
  )
}

run_cli_fixture() {
  local ref="$1"
  local safe_ref tmpdir log_path phase fixture_status
  safe_ref="$(safe_ref_name "$ref")"
  tmpdir="$(mktemp -d)"
  fixture_status=0

  for phase in released candidate; do
    log_path="$result_root/cli-$safe_ref-$phase.log"
    if run_cli_phase "$ref" "$phase" "$tmpdir" >"$log_path" 2>&1; then
      printf 'cli\t%s\t%s\tpassed\t%s\n' "$ref" "$phase" "$log_path" >>"$status_tsv"
    else
      printf 'cli\t%s\t%s\tfailed\t%s\n' "$ref" "$phase" "$log_path" >>"$status_tsv"
      printf 'downstream fixture failed: fixture=cli ref=%s phase=%s log=%s\n' "$ref" "$phase" "$log_path" >&2
      fixture_status=1
    fi
  done
  rm -rf -- "$tmpdir"
  return "$fixture_status"
}

split_refs
status="passed"
for ref in "${base_refs[@]}"; do
  run_standard_fixture rootcore "$ref" root "" || status="failed"
  run_standard_fixture nethttp "$ref" root "" || status="failed"
  run_standard_fixture chi "$ref" root "github.com/go-chi/chi/v5@v5.2.5" || status="failed"
  run_standard_fixture idempotency "$ref" root,contrib "" || status="failed"
  run_standard_fixture adapters "$ref" root,contrib "" || status="failed"
  run_cli_fixture "$ref" || status="failed"
done

printf '%s\n' "$status" >"$status_path"
printf 'downstream compatibility matrix %s; status=%s\n' "$status_tsv" "$status"
if [ "$status" != "passed" ]; then
  exit 1
fi
