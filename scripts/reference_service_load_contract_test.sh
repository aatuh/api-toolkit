#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/reference_service_load.sh"

tmp="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

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

make_fake_go() {
  local bin_dir="$1"
  mkdir -p "$bin_dir"
  cat >"$bin_dir/go" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
{
  printf 'go'
  for arg in "$@"; do
    printf ' %s' "$arg"
  done
  printf '\n'
} >>"${FAKE_TOOL_CALLS:?}"
if [ "${1:-}" = "run" ] && [ "${2:-}" = "./cmd/loadsmoke" ]; then
  shift 2
  requests=""
  concurrency=""
  out=""
  while [ "$#" -gt 0 ]; do
    case "$1" in
      -requests) requests="$2"; shift 2 ;;
      -concurrency) concurrency="$2"; shift 2 ;;
      -out) out="$2"; shift 2 ;;
      *) printf 'unexpected load smoke flag: %s\n' "$1" >&2; exit 2 ;;
    esac
  done
  mkdir -p "$out"
  printf 'passed\n' >"$out/status"
  cat >"$out/summary.json" <<JSON
{
  "schema": "reference-service-load-smoke.v1",
  "status": "passed",
  "requests": $requests,
  "concurrency": $concurrency,
  "throughput_rps": 100,
  "latency_ms": {"p95": 1.25},
  "memory": {"total_alloc_delta_bytes": 2048},
  "allocations": {"allocs_per_request": 10},
  "failure_behavior": {"expected_status": 401, "unexpected_status_count": 0}
}
JSON
  cat >"$out/summary.md" <<'MD'
# Reference Service Load Smoke

auth_failure
MD
  printf 'reference-service load smoke passed; summary=%s/summary.json\n' "$out"
  exit 0
fi
printf 'unexpected fake go call: %s\n' "$*" >&2
exit 2
FAKE
  chmod +x "$bin_dir/go"
}

make_fake_go "$tmp/bin"

repo="$tmp/repo"
mkdir -p "$repo/examples/reference-saas-api"

output="$(FAKE_TOOL_CALLS="$tmp/calls" \
  REFERENCE_SERVICE_LOAD_REPO_ROOT="$repo" \
  REFERENCE_SERVICE_LOAD_RESULT_DIR=".ci-result/load-contract" \
  REFERENCE_SERVICE_LOAD_REQUESTS=17 \
  REFERENCE_SERVICE_LOAD_CONCURRENCY=3 \
  PATH="$tmp/bin:$PATH" \
  require_success load-contract "$script")"

case "$output" in
  *"reference-service load smoke passed"*) ;;
  *) printf 'load contract output missing pass status:\n%s\n' "$output" >&2; exit 1 ;;
esac

result="$repo/.ci-result/load-contract"
for file in status summary.json summary.md load-smoke.log; do
  if [ ! -f "$result/$file" ]; then
    printf 'load contract missing %s\n' "$result/$file" >&2
    exit 1
  fi
done
for required in '"schema": "reference-service-load-smoke.v1"' '"requests": 17' '"concurrency": 3' '"expected_status": 401'; do
  if ! grep -Fq "$required" "$result/summary.json"; then
    printf 'load summary missing %q:\n%s\n' "$required" "$(cat "$result/summary.json")" >&2
    exit 1
  fi
done
if ! grep -Fqx "passed" "$result/status"; then
  printf 'load status file is wrong:\n%s\n' "$(cat "$result/status")" >&2
  exit 1
fi
for required in "go run ./cmd/loadsmoke" "-requests 17" "-concurrency 3" "-out $result"; do
  if ! grep -Fq -- "$required" "$tmp/calls"; then
    printf 'fake go call missing %q:\n%s\n' "$required" "$(cat "$tmp/calls")" >&2
    exit 1
  fi
done

echo "reference service load contract tests passed"
