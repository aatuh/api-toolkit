#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/reference_service_evidence.sh"

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
if [ "${1:-}" = "version" ]; then
  printf 'go version go1.99.0 fake/linux\n'
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
  printf ' docker=%s minio=%s object_store=%s\n' "${REFERENCE_SERVICE_DOCKER:-}" "${REFERENCE_SERVICE_MINIO:-}" "${INTEGRATION_OBJECT_STORE:-}"
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
  mkdir -p "$dir/repo"
  FAKE_TOOL_CALLS="$dir/calls" REFERENCE_SERVICE_EVIDENCE_REPO_ROOT="$dir/repo" REFERENCE_SERVICE_EVIDENCE_RESULT_DIR=".ci-result/$name" PATH="$tmp/bin:$PATH" "$@" "$script"
}

make_fake_tools "$tmp/bin"

skipped_output="$(require_success skipped-docker run_contract skipped env -u REFERENCE_SERVICE_DOCKER -u REFERENCE_SERVICE_MINIO)"
case "$skipped_output" in
  *"reference service evidence passed"*) ;;
  *) printf 'skipped-docker output missing pass status:\n%s\n' "$skipped_output" >&2; exit 1 ;;
esac
skipped_summary="$tmp/skipped/repo/.ci-result/skipped/summary.json"
for required in '"status": "passed"' '"commit": "abc123"' '"toolchain": "go version go1.99.0 fake/linux"' '"status": "skipped"' '"requested": false' '.ci-result/skipped/reference-service-check.log'; do
  if ! grep -Fq "$required" "$skipped_summary"; then
    printf 'skipped summary missing %q:\n%s\n' "$required" "$(cat "$skipped_summary")" >&2
    exit 1
  fi
done
if grep -Fq "integration-check" "$tmp/skipped/calls"; then
  printf 'skipped-docker should not call integration-check:\n%s\n' "$(cat "$tmp/skipped/calls")" >&2
  exit 1
fi

docker_output="$(require_success docker-minio run_contract docker env REFERENCE_SERVICE_DOCKER=1 REFERENCE_SERVICE_MINIO=1)"
case "$docker_output" in
  *"reference service evidence passed"*) ;;
  *) printf 'docker-minio output missing pass status:\n%s\n' "$docker_output" >&2; exit 1 ;;
esac
docker_summary="$tmp/docker/repo/.ci-result/docker/summary.json"
for required in '"requested": true' '"minio": true' '"status": "passed"' '.ci-result/docker/integration-check.log'; do
  if ! grep -Fq "$required" "$docker_summary"; then
    printf 'docker summary missing %q:\n%s\n' "$required" "$(cat "$docker_summary")" >&2
    exit 1
  fi
done
if ! grep -Fq "integration-check" "$tmp/docker/calls" || ! grep -Fq "object_store=s3" "$tmp/docker/calls"; then
  printf 'docker-minio calls missing integration or MinIO env:\n%s\n' "$(cat "$tmp/docker/calls")" >&2
  exit 1
fi

failure_output="$(require_failure failed-status run_contract failed env FAIL_MAKE_TARGET=reference-service-check)"
case "$failure_output" in
  *"reference service evidence failed"*) ;;
  *) printf 'failure output missing failed status:\n%s\n' "$failure_output" >&2; exit 1 ;;
esac
failed_status="$tmp/failed/repo/.ci-result/failed/status"
failed_summary="$tmp/failed/repo/.ci-result/failed/summary.json"
if ! grep -Fqx "failed" "$failed_status"; then
  printf 'failed status file is wrong:\n%s\n' "$(cat "$failed_status")" >&2
  exit 1
fi
for required in '"status": "failed"' '"reference_service_check": {' '.ci-result/failed/reference-service-check.log'; do
  if ! grep -Fq "$required" "$failed_summary"; then
    printf 'failed summary missing %q:\n%s\n' "$required" "$(cat "$failed_summary")" >&2
    exit 1
  fi
done

echo "reference service evidence contract tests passed"
