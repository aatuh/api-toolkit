#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/generated_soak_check.sh"

tmp="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

require_failure() {
  local name="$1"
  shift
  local output
  set +e
  output="$("$@" 2>&1)"
  local status=$?
  set -e
  if [ "$status" -eq 0 ]; then
    printf 'expected %s to fail, but it passed\noutput:\n%s\n' "$name" "$output" >&2
    exit 1
  fi
  printf '%s' "$output"
}

require_success() {
  local name="$1"
  shift
  local output
  set +e
  output="$("$@" 2>&1)"
  local status=$?
  set -e
  if [ "$status" -ne 0 ]; then
    printf 'expected %s to pass, but it failed with %s\noutput:\n%s\n' "$name" "$status" "$output" >&2
    exit 1
  fi
  printf '%s' "$output"
}

make_fake_tools() {
  local bin_dir="$1"
  mkdir -p "$bin_dir"
  cat >"$bin_dir/git" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" = "-C" ]; then
  shift 2
fi
if [ "${1:-}" = "rev-parse" ] && [ "${2:-}" = "HEAD" ]; then
  printf 'abc123\n'
  exit 0
fi
printf 'unexpected fake git call: %s\n' "$*" >&2
exit 2
FAKE
  cat >"$bin_dir/go" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
{
  printf 'go'
  for arg in "$@"; do
    printf ' %s' "$arg"
  done
  printf ' duration=%s workers=%s\n' "${GENERATED_SOAK_TEST_DURATION:-}" "${GENERATED_SOAK_RACE_WORKERS:-}"
} >>"${FAKE_TOOL_CALLS:?}"
if [ "${1:-}" = "version" ]; then
  printf 'go version go1.99.0 fake/linux\n'
  exit 0
fi
if [ "${1:-}" = "run" ]; then
  service_dir=""
  for ((i = 1; i <= $#; i++)); do
    if [ "${!i}" = "--dir" ]; then
      next=$((i + 1))
      service_dir="${!next}"
    fi
  done
  if [ -z "$service_dir" ]; then
    printf 'fake go run missing --dir\n' >&2
    exit 2
  fi
  mkdir -p "$service_dir/internal/httpapi"
  exit 0
fi
if [ "${1:-}" = "mod" ] && [ "${2:-}" = "tidy" ]; then
  exit 0
fi
if [ "${1:-}" = "test" ]; then
  if [ ! -f "internal/httpapi/generated_soak_test.go" ]; then
    printf 'generated soak test was not written\n' >&2
    exit 2
  fi
  for required in "TestGeneratedFullProfileSoakNoGoroutineGrowth" "runtime.NumGoroutine" "GENERATED_SOAK_TEST_DURATION"; do
    if ! grep -Fq "$required" internal/httpapi/generated_soak_test.go; then
      printf 'generated soak test missing %s\n' "$required" >&2
      exit 2
    fi
  done
  exit 0
fi
printf 'unexpected fake go call: %s\n' "$*" >&2
exit 2
FAKE
  cat >"$bin_dir/make" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
{
  printf 'make'
  for arg in "$@"; do
    printf ' %s' "$arg"
  done
  printf '\n'
} >>"${FAKE_TOOL_CALLS:?}"
target="${*: -1}"
if [ -n "${FAIL_MAKE_TARGET:-}" ] && [ "$target" = "$FAIL_MAKE_TARGET" ]; then
  printf 'forced failure for %s\n' "$FAIL_MAKE_TARGET" >&2
  exit 1
fi
exit 0
FAKE
  chmod +x "$bin_dir/git" "$bin_dir/go" "$bin_dir/make"
}

run_contract() {
  local name="$1"
  shift
  local dir="$tmp/$name"
  mkdir -p "$dir/repo/contrib"
  FAKE_TOOL_CALLS="$dir/calls" \
    GENERATED_SOAK_REPO_ROOT="$dir/repo" \
    GENERATED_SOAK_RESULT_DIR=".ci-result/$name" \
    GENERATED_SOAK_DURATION_SECONDS=5 \
    GENERATED_SOAK_INTEGRATION_CYCLES=2 \
    GENERATED_SOAK_RACE_WORKERS=3 \
    PATH="$tmp/bin:$PATH" \
    "$@" "$script"
}

make_fake_tools "$tmp/bin"

default_output="$(require_success default-generated-soak run_contract default env -u FAIL_MAKE_TARGET)"
case "$default_output" in
  *"generated soak check passed; summary=.ci-result/default/summary.json"*) ;;
  *) printf 'default output changed unexpectedly:\n%s\n' "$default_output" >&2; exit 1 ;;
esac
default_summary="$tmp/default/repo/.ci-result/default/summary.json"
for required in \
  '"status": "passed"' \
  '"profile": "saas-api-full"' \
  '"duration_seconds": 5' \
  '"integration_cycles": 2' \
  '"race_workers": 3' \
  '"soak_contract": {' \
  '"go test -race ./internal/httpapi -run TestGeneratedFullProfileSoakNoGoroutineGrowth"' \
  '"runtime.NumGoroutine before/after threshold"' \
  '"repeated make integration-check"'; do
  if ! grep -Fq "$required" "$default_summary"; then
    printf 'default summary missing %q:\n%s\n' "$required" "$(cat "$default_summary")" >&2
    exit 1
  fi
done
for file in generate.log race-soak.log build-and-contracts.log integration-cycle-1.log integration-cycle-2.log; do
  if [ ! -f "$tmp/default/repo/.ci-result/default/$file" ]; then
    printf 'default generated soak missing log %s\n' "$file" >&2
    exit 1
  fi
done
for required in \
  "go run ./cmd/api-toolkit new service" \
  "--profile saas-api-full" \
  "--auth api-key" \
  "go mod tidy" \
  "go test -race ./internal/httpapi -run TestGeneratedFullProfileSoakNoGoroutineGrowth -count=1 duration=5s workers=3" \
  "make build" \
  "make contracts-lint" \
  "make contracts-diff" \
  "make openapi-check" \
  "make client-check"; do
  if ! grep -Fq -- "$required" "$tmp/default/calls"; then
    printf 'default calls missing %q:\n%s\n' "$required" "$(cat "$tmp/default/calls")" >&2
    exit 1
  fi
done
if [ "$(grep -Fc "make integration-check" "$tmp/default/calls")" -ne 2 ]; then
  printf 'expected two integration cycles:\n%s\n' "$(cat "$tmp/default/calls")" >&2
  exit 1
fi
if [ "$(cat "$tmp/default/repo/.ci-result/default/status")" != "passed" ]; then
  printf 'default status should be passed\n' >&2
  exit 1
fi

failure_output="$(require_failure integration-failure run_contract failure env FAIL_MAKE_TARGET=integration-check)"
case "$failure_output" in
  *"generated soak check failed"*) ;;
  *) printf 'failure output changed unexpectedly:\n%s\n' "$failure_output" >&2; exit 1 ;;
esac
if [ "$(cat "$tmp/failure/repo/.ci-result/failure/status")" != "failed" ]; then
  printf 'failure status should be failed\n' >&2
  exit 1
fi
if ! grep -Fq '"status": "failed"' "$tmp/failure/repo/.ci-result/failure/summary.json"; then
  printf 'failure summary should record failed status:\n%s\n' "$(cat "$tmp/failure/repo/.ci-result/failure/summary.json")" >&2
  exit 1
fi

path_output="$(require_failure unsafe-result-path env GENERATED_SOAK_REPO_ROOT="$tmp/path/repo" GENERATED_SOAK_RESULT_DIR="../escape" PATH="$tmp/bin:$PATH" "$script")"
case "$path_output" in
  *"GENERATED_SOAK_RESULT_DIR must be a repo-relative path without .. components"*) ;;
  *) printf 'unsafe path output changed unexpectedly:\n%s\n' "$path_output" >&2; exit 1 ;;
esac

duration_output="$(require_failure invalid-duration env GENERATED_SOAK_REPO_ROOT="$tmp/duration/repo" GENERATED_SOAK_DURATION_SECONDS=0 PATH="$tmp/bin:$PATH" "$script")"
case "$duration_output" in
  *"GENERATED_SOAK_DURATION_SECONDS must be an integer between 1 and 7200"*) ;;
  *) printf 'invalid duration output changed unexpectedly:\n%s\n' "$duration_output" >&2; exit 1 ;;
esac

cycles_output="$(require_failure invalid-cycles env GENERATED_SOAK_REPO_ROOT="$tmp/cycles/repo" GENERATED_SOAK_INTEGRATION_CYCLES=0 PATH="$tmp/bin:$PATH" "$script")"
case "$cycles_output" in
  *"GENERATED_SOAK_INTEGRATION_CYCLES must be an integer between 1 and 50"*) ;;
  *) printf 'invalid cycles output changed unexpectedly:\n%s\n' "$cycles_output" >&2; exit 1 ;;
esac

echo "generated soak contract tests passed"
