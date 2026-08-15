#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/generated_integration_check.sh"

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
  mkdir -p "$service_dir"
fi
exit 0
FAKE
  cat >"$bin_dir/make" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
{
  printf 'make'
  for arg in "$@"; do
    printf ' %s' "$arg"
  done
  printf ' minio=%s object_store=%s\n' "${INCLUDE_MINIO:-}" "${INTEGRATION_OBJECT_STORE:-}"
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
  mkdir -p "$dir/repo/contrib" "$dir/repo/cmd/api-toolkit"
  FAKE_TOOL_CALLS="$dir/calls" GENERATED_INTEGRATION_REPO_ROOT="$dir/repo" GENERATED_INTEGRATION_RESULT_DIR=".ci-result/$name" PATH="$tmp/bin:$PATH" "$@" "$script"
}

make_fake_tools "$tmp/bin"

default_output="$(require_success default-generated-integration run_contract default env -u INCLUDE_MINIO -u FAIL_MAKE_TARGET)"
case "$default_output" in
  *"generated integration check passed; log=.ci-result/default/integration-check.log"*) ;;
  *) printf 'default output changed unexpectedly:\n%s\n' "$default_output" >&2; exit 1 ;;
esac
default_summary="$tmp/default/repo/.ci-result/default/summary.json"
for required in \
  '"status": "passed"' \
  '"profile": "saas-api-full"' \
  '"auth": "api-key"' \
  '"provider_workflows": ["stripe-billing","resend-email","clerk-webhooks","entitlements"]' \
  '"minio": false' \
  '"runtime_contract": {' \
  '"make build"' \
  '"make openapi-check"' \
  '"GET /docs/openapi.json"' \
  '"POST /widgets"' \
  '"Idempotency-Replayed"'; do
  if ! grep -Fq "$required" "$default_summary"; then
    printf 'default summary missing %q:\n%s\n' "$required" "$(cat "$default_summary")" >&2
    exit 1
  fi
done
for required in \
  "go run . new service" \
  "--profile saas-api-full" \
  "--auth api-key" \
  "--with stripe-billing" \
  "--with resend-email" \
  "--with clerk-webhooks" \
  "--with entitlements" \
  "go mod tidy" \
  "go test ./..." \
  "make build" \
  "make contracts-lint" \
  "make contracts-diff" \
  "make openapi-check" \
  "make client-check" \
  "make provider-check" \
  "make integration-check"; do
  if ! grep -Fq -- "$required" "$tmp/default/calls"; then
    printf 'default calls missing %q:\n%s\n' "$required" "$(cat "$tmp/default/calls")" >&2
    exit 1
  fi
done
if [ "$(cat "$tmp/default/repo/.ci-result/default/status")" != "passed" ]; then
  printf 'default status should be passed\n' >&2
  exit 1
fi

minio_output="$(require_success minio-generated-integration run_contract minio env INCLUDE_MINIO=true)"
case "$minio_output" in
  *"generated integration check passed"*) ;;
  *) printf 'minio output changed unexpectedly:\n%s\n' "$minio_output" >&2; exit 1 ;;
esac
if ! grep -Fq '"minio": true' "$tmp/minio/repo/.ci-result/minio/summary.json"; then
  printf 'minio summary did not record minio=true:\n%s\n' "$(cat "$tmp/minio/repo/.ci-result/minio/summary.json")" >&2
  exit 1
fi
if ! grep -Fq "make integration-check minio=true object_store=s3" "$tmp/minio/calls"; then
  printf 'minio calls missing S3 integration environment:\n%s\n' "$(cat "$tmp/minio/calls")" >&2
  exit 1
fi

failure_output="$(require_failure build-failure run_contract failure env FAIL_MAKE_TARGET=build)"
case "$failure_output" in
  *"generated integration check failed"*) ;;
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

unsupported_provider_output="$(require_failure unsupported-provider run_contract unsupported env GENERATED_INTEGRATION_PROVIDERS=unknown-provider)"
case "$unsupported_provider_output" in
  *"generated integration check failed"*) ;;
  *) printf 'unsupported provider output changed unexpectedly:\n%s\n' "$unsupported_provider_output" >&2; exit 1 ;;
esac
if ! grep -Fq 'GENERATED_INTEGRATION_PROVIDERS contains unsupported workflow unknown-provider' "$tmp/unsupported/repo/.ci-result/unsupported/integration-check.log"; then
  printf 'unsupported provider error was not recorded safely:\n%s\n' "$(cat "$tmp/unsupported/repo/.ci-result/unsupported/integration-check.log")" >&2
  exit 1
fi

path_output="$(require_failure unsafe-result-path env GENERATED_INTEGRATION_REPO_ROOT="$tmp/path/repo" GENERATED_INTEGRATION_RESULT_DIR="../escape" PATH="$tmp/bin:$PATH" "$script")"
case "$path_output" in
  *"GENERATED_INTEGRATION_RESULT_DIR must be a repo-relative path without .. components"*) ;;
  *) printf 'unsafe path output changed unexpectedly:\n%s\n' "$path_output" >&2; exit 1 ;;
esac

echo "generated integration contract tests passed"
