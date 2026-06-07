#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/generated_failure_check.sh"

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
  printf '\n'
} >>"${FAKE_TOOL_CALLS:?}"
if [ "${1:-}" = "version" ]; then
  printf 'go version go1.99.0 fake/linux\n'
  exit 0
fi
if [ "${1:-}" = "run" ]; then
  service_dir=""
  auth_mode=""
  for ((i = 1; i <= $#; i++)); do
    if [ "${!i}" = "--dir" ]; then
      next=$((i + 1))
      service_dir="${!next}"
    fi
    if [ "${!i}" = "--auth" ]; then
      next=$((i + 1))
      auth_mode="${!next}"
    fi
  done
  if [ -z "$service_dir" ] || [ -z "$auth_mode" ]; then
    printf 'fake go run missing --dir or --auth\n' >&2
    exit 2
  fi
  mkdir -p "$service_dir/internal/httpapi" "$service_dir/cmd/api"
  exit 0
fi
if [ "${1:-}" = "mod" ] && [ "${2:-}" = "tidy" ]; then
  exit 0
fi
if [ "${1:-}" = "test" ]; then
  package="${2:-}"
  if [ -n "${FAIL_GO_TEST_PACKAGE:-}" ] && [ "$package" = "$FAIL_GO_TEST_PACKAGE" ]; then
    printf 'forced failure for %s\n' "$FAIL_GO_TEST_PACKAGE" >&2
    exit 1
  fi
  case "$package" in
    ./internal/httpapi)
      file="internal/httpapi/generated_failure_test.go"
      if [ ! -f "$file" ]; then
        printf 'generated API-key failure test was not written\n' >&2
        exit 2
      fi
      for required in \
        "TestGeneratedFailureReadinessFailsClosedForPostgresAndRedis" \
        "postgres_down" \
        "redis_down" \
        "TestGeneratedFailureExpiredManagedAPIKeyIsRejected" \
        "TestGeneratedFailureSlowDownstreamHardTimeoutIsBounded" \
        "timeoutmw.NewHard" \
        "application/problem+json" \
        "service is not ready"; do
        if ! grep -Fq "$required" "$file"; then
          printf 'generated API-key failure test missing %s\n' "$required" >&2
          exit 2
        fi
      done
      ;;
    ./cmd/api)
      file="cmd/api/generated_failure_jwt_test.go"
      if [ ! -f "$file" ]; then
        printf 'generated JWT failure test was not written\n' >&2
        exit 2
      fi
      for required in \
        "TestGeneratedFailureBadJWKSURLFailsClosed" \
        "127.0.0.1:1/jwks" \
        "JWT_JWKS_URL" \
        "newJWTMiddleware" \
        "bad JWKS endpoint"; do
        if ! grep -Fq "$required" "$file"; then
          printf 'generated JWT failure test missing %s\n' "$required" >&2
          exit 2
        fi
      done
      ;;
    *)
      printf 'unexpected fake go test package: %s\n' "$package" >&2
      exit 2
      ;;
  esac
  exit 0
fi
printf 'unexpected fake go call: %s\n' "$*" >&2
exit 2
FAKE
  chmod +x "$bin_dir/git" "$bin_dir/go"
}

run_contract() {
  local name="$1"
  shift
  local dir="$tmp/$name"
  mkdir -p "$dir/repo/contrib"
  FAKE_TOOL_CALLS="$dir/calls" \
    GENERATED_FAILURE_REPO_ROOT="$dir/repo" \
    GENERATED_FAILURE_RESULT_DIR=".ci-result/$name" \
    PATH="$tmp/bin:$PATH" \
    "$@" "$script"
}

make_fake_tools "$tmp/bin"

default_output="$(require_success default-generated-failure run_contract default env -u FAIL_GO_TEST_PACKAGE)"
case "$default_output" in
  *"generated failure check passed; summary=.ci-result/default/summary.json"*) ;;
  *) printf 'default output changed unexpectedly:\n%s\n' "$default_output" >&2; exit 1 ;;
esac
default_summary="$tmp/default/repo/.ci-result/default/summary.json"
for required in \
  '"status": "passed"' \
  '"profile": "saas-api-full"' \
  '"api-key"' \
  '"jwt"' \
  '"failure_contract": {' \
  '"redis_down"' \
  '"postgres_down"' \
  '"expired_api_key"' \
  '"bad_jwks_endpoint"' \
  '"slow_downstream_timeout"' \
  '"go test ./internal/httpapi -run TestGeneratedFailure -count=1"' \
  '"go test ./cmd/api -run TestGeneratedFailureBadJWKSURLFailsClosed -count=1"'; do
  if ! grep -Fq "$required" "$default_summary"; then
    printf 'default summary missing %q:\n%s\n' "$required" "$(cat "$default_summary")" >&2
    exit 1
  fi
done
for file in api-key-generate.log api-key-failure-tests.log jwt-generate.log jwt-failure-tests.log; do
  if [ ! -f "$tmp/default/repo/.ci-result/default/$file" ]; then
    printf 'default generated failure missing log %s\n' "$file" >&2
    exit 1
  fi
done
for required in \
  "go run ./cmd/api-toolkit new service" \
  "--profile saas-api-full" \
  "--auth api-key" \
  "--auth jwt" \
  "go mod tidy" \
  "go test ./internal/httpapi -run TestGeneratedFailure -count=1" \
  "go test ./cmd/api -run TestGeneratedFailureBadJWKSURLFailsClosed -count=1"; do
  if ! grep -Fq -- "$required" "$tmp/default/calls"; then
    printf 'default calls missing %q:\n%s\n' "$required" "$(cat "$tmp/default/calls")" >&2
    exit 1
  fi
done
if [ "$(cat "$tmp/default/repo/.ci-result/default/status")" != "passed" ]; then
  printf 'default status should be passed\n' >&2
  exit 1
fi

failure_output="$(require_failure jwt-failure run_contract failure env FAIL_GO_TEST_PACKAGE=./cmd/api)"
case "$failure_output" in
  *"generated failure check failed"*) ;;
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

path_output="$(require_failure unsafe-result-path env GENERATED_FAILURE_REPO_ROOT="$tmp/path/repo" GENERATED_FAILURE_RESULT_DIR="../escape" PATH="$tmp/bin:$PATH" "$script")"
case "$path_output" in
  *"GENERATED_FAILURE_RESULT_DIR must be a repo-relative path without .. components"*) ;;
  *) printf 'unsafe path output changed unexpectedly:\n%s\n' "$path_output" >&2; exit 1 ;;
esac

echo "generated failure contract tests passed"
