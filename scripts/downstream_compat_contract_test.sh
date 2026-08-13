#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/downstream_compat_check.sh"
tmp="$(mktemp -d)"
cleanup() {
  rm -rf -- "$tmp"
}
trap cleanup EXIT

require_success() {
  local name="$1"
  shift
  local output status
  set +e
  output="$("$@" 2>&1)"
  status=$?
  set -e
  if [ "$status" -ne 0 ]; then
    printf 'expected %s to pass, but it failed with %s\noutput:\n%s\n' "$name" "$status" "$output" >&2
    exit 1
  fi
  printf '%s' "$output"
}

require_failure() {
  local name="$1"
  shift
  local output status
  set +e
  output="$("$@" 2>&1)"
  status=$?
  set -e
  if [ "$status" -eq 0 ]; then
    printf 'expected %s to fail, but it passed\noutput:\n%s\n' "$name" "$output" >&2
    exit 1
  fi
  printf '%s' "$output"
}

make_fake_go() {
  local bin_dir="$1"
  mkdir -p "$bin_dir"
  cat >"$bin_dir/go" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
printf 'go %s\n' "$*" >>"${FAKE_TOOL_CALLS:?}"
if [ "${1:-}" = "mod" ] && [ "${2:-}" = "init" ]; then
  printf 'module %s\n' "${3:-example.invalid}" >go.mod
fi
if [ "${1:-}" = "run" ]; then
  previous=""
  for argument in "$@"; do
    if [ "$previous" = "--dir" ]; then
      mkdir -p "$argument"
      printf 'module example.com/generated-downstream\n' >"$argument/go.mod"
      break
    fi
    previous="$argument"
  done
fi
if [ "${1:-}" = "test" ] && [ -n "${FAIL_GO_TEST:-}" ]; then
  printf 'forced go test failure\n' >&2
  exit 1
fi
FAKE
  chmod +x "$bin_dir/go"
}

make_fixture_repo() {
  local dir="$1"
  local fixture
  mkdir -p "$dir/repo/contrib/cmd/api-toolkit" "$dir/tmp"
  for fixture in rootcore nethttp chi idempotency adapters; do
    mkdir -p "$dir/repo/internal/compatfixtures/$fixture"
    cat >"$dir/repo/internal/compatfixtures/$fixture/upgrade_smoke_test.go" <<'GO'
//go:build downstreamcompat

package upgradesmoke

import "testing"

func TestDownstreamFixture(t *testing.T) {}
GO
  done
}

make_fake_go "$tmp/bin"
make_fixture_repo "$tmp/default"

missing_baseline_output="$(require_failure missing-baseline env \
  DOWNSTREAM_COMPAT_RESULT_DIR=.ci-result/missing \
  PATH="$tmp/bin:$PATH" \
  bash -c "cd '$tmp/default/repo' && '$script'")"
case "$missing_baseline_output" in
  *"must name at least one verified released version tag"*) ;;
  *) printf 'missing-baseline output changed unexpectedly:\n%s\n' "$missing_baseline_output" >&2; exit 1 ;;
esac

default_output="$(require_success default-matrix env \
  FAKE_TOOL_CALLS="$tmp/default/calls" \
  DOWNSTREAM_COMPAT_BASE_REFS=v4.0.0 \
  DOWNSTREAM_COMPAT_RESULT_DIR=.ci-result/default \
  TMPDIR="$tmp/default/tmp" \
  PATH="$tmp/bin:$PATH" \
  bash -c "cd '$tmp/default/repo' && '$script'")"
case "$default_output" in
  *"downstream compatibility matrix"*"status=passed"*) ;;
  *) printf 'default output changed unexpectedly:\n%s\n' "$default_output" >&2; exit 1 ;;
esac
default_status="$tmp/default/repo/.ci-result/default/status.tsv"
if [ "$(wc -l <"$default_status")" -ne 12 ]; then
  printf 'default matrix should record 12 fixture phases:\n%s\n' "$(cat "$default_status")" >&2
  exit 1
fi
for required in \
  $'rootcore\tv4.0.0\treleased\tpassed' \
  $'rootcore\tv4.0.0\tcandidate\tpassed' \
  $'idempotency\tv4.0.0\treleased\tpassed' \
  $'adapters\tv4.0.0\tcandidate\tpassed' \
  $'cli\tv4.0.0\treleased\tpassed' \
  $'cli\tv4.0.0\tcandidate\tpassed'; do
  if ! grep -Fq "$required" "$default_status"; then
    printf 'default matrix missing %q:\n%s\n' "$required" "$(cat "$default_status")" >&2
    exit 1
  fi
done
for required in \
  "go get github.com/aatuh/api-toolkit/v4@v4.0.0" \
  "go get github.com/aatuh/api-toolkit/contrib/v4@v4.0.0" \
  "go mod edit -replace=github.com/aatuh/api-toolkit/v4=$tmp/default/repo" \
  "go mod edit -replace=github.com/aatuh/api-toolkit/contrib/v4=$tmp/default/repo/contrib" \
  "go test -tags=downstreamcompat ./..." \
  "go run github.com/aatuh/api-toolkit/contrib/v4/cmd/api-toolkit@v4.0.0 new service" \
  "go run ./cmd/api-toolkit new service"; do
  if ! grep -Fq "$required" "$tmp/default/calls"; then
    printf 'fake go calls missing %q:\n%s\n' "$required" "$(cat "$tmp/default/calls")" >&2
    exit 1
  fi
done

invalid_output="$(require_failure invalid-ref env \
  DOWNSTREAM_COMPAT_BASE_REFS='v4.0.0;touch' \
  DOWNSTREAM_COMPAT_RESULT_DIR=.ci-result/invalid \
  PATH="$tmp/bin:$PATH" \
  bash -c "cd '$tmp/default/repo' && '$script'")"
case "$invalid_output" in
  *"invalid released version tag"*) ;;
  *) printf 'invalid-ref output changed unexpectedly:\n%s\n' "$invalid_output" >&2; exit 1 ;;
esac

traversal_output="$(require_failure traversal-result-dir env \
  DOWNSTREAM_COMPAT_RESULT_DIR='../outside' \
  PATH="$tmp/bin:$PATH" \
  bash -c "cd '$tmp/default/repo' && '$script'")"
case "$traversal_output" in
  *"must be a non-empty relative path without parent traversal"*) ;;
  *) printf 'traversal output changed unexpectedly:\n%s\n' "$traversal_output" >&2; exit 1 ;;
esac

make_fixture_repo "$tmp/failure"
failure_output="$(require_failure failed-test env \
  FAKE_TOOL_CALLS="$tmp/failure/calls" \
  FAIL_GO_TEST=1 \
  DOWNSTREAM_COMPAT_BASE_REFS=v4.0.0 \
  DOWNSTREAM_COMPAT_RESULT_DIR=.ci-result/failure \
  TMPDIR="$tmp/failure/tmp" \
  PATH="$tmp/bin:$PATH" \
  bash -c "cd '$tmp/failure/repo' && '$script'")"
case "$failure_output" in
  *"status=failed"*) ;;
  *) printf 'failure output changed unexpectedly:\n%s\n' "$failure_output" >&2; exit 1 ;;
esac
if [ "$(cat "$tmp/failure/repo/.ci-result/failure/status")" != "failed" ]; then
  printf 'failed matrix should write aggregate failure status\n' >&2
  exit 1
fi

echo "downstream compatibility contract tests passed"
