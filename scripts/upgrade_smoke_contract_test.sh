#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/upgrade_smoke_check.sh"

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

make_fake_go() {
  local bin_dir="$1"
  mkdir -p "$bin_dir"
  cat >"$bin_dir/go" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
printf 'go %s\n' "$*" >>"${FAKE_TOOL_CALLS:?}"
if [ "${1:-}" = "test" ] && [ -n "${FAIL_GO_TEST:-}" ]; then
  printf 'forced go test failure\n' >&2
  exit 1
fi
exit 0
FAKE
  chmod +x "$bin_dir/go"
}

run_contract() {
  local name="$1"
  shift
  local dir="$tmp/$name"
  mkdir -p "$dir/repo/internal/compatfixtures/rootcore"
  cat >"$dir/repo/internal/compatfixtures/rootcore/upgrade_smoke_test.go" <<'GO'
package upgradesmoke

import "testing"

func TestStableCoreUpgradeSmoke(t *testing.T) {}
GO
  FAKE_TOOL_CALLS="$dir/calls" UPGRADE_SMOKE_REPO_ROOT="$dir/repo" UPGRADE_SMOKE_RESULT_DIR=".ci-result/$name" PATH="$tmp/bin:$PATH" "$@" "$script"
}

make_fake_go "$tmp/bin"

default_output="$(require_success default-baseline run_contract default env -u UPGRADE_SMOKE_BASE_REF)"
case "$default_output" in
  *"upgrade smoke check passed for root-core from v3.1.2"* ) ;;
  *) printf 'default baseline output changed unexpectedly:\n%s\n' "$default_output" >&2; exit 1 ;;
esac
default_status="$tmp/default/repo/.ci-result/default/status.tsv"
if ! grep -Fqx $'root-core\tv3.1.2\tpassed\t.ci-result/default/root-core-v3.1.2.log' "$default_status"; then
  printf 'default status missing expected row:\n%s\n' "$(cat "$default_status")" >&2
  exit 1
fi
for required in \
  "go mod init example.com/api-toolkit-upgrade-smoke" \
  "go get github.com/aatuh/api-toolkit/v4@v3.1.2" \
  "go mod edit -replace=github.com/aatuh/api-toolkit/v4=$tmp/default/repo" \
  "go test -tags=downstreamcompat ./..."; do
  if ! grep -Fqx "$required" "$tmp/default/calls"; then
    printf 'fake go calls missing %q:\n%s\n' "$required" "$(cat "$tmp/default/calls")" >&2
    exit 1
  fi
done

explicit_output="$(require_success explicit-baseline run_contract explicit env UPGRADE_SMOKE_BASE_REF=v3.0.0)"
case "$explicit_output" in
  *"upgrade smoke check passed for root-core from v3.0.0"* ) ;;
  *) printf 'explicit baseline output changed unexpectedly:\n%s\n' "$explicit_output" >&2; exit 1 ;;
esac
if ! grep -Fqx $'root-core\tv3.0.0\tpassed\t.ci-result/explicit/root-core-v3.0.0.log' "$tmp/explicit/repo/.ci-result/explicit/status.tsv"; then
  printf 'explicit status missing expected row:\n%s\n' "$(cat "$tmp/explicit/repo/.ci-result/explicit/status.tsv")" >&2
  exit 1
fi

failure_output="$(require_failure go-test-failure run_contract failure env FAIL_GO_TEST=1)"
case "$failure_output" in
  *"upgrade smoke check failed for root-core from v3.1.2"* ) ;;
  *) printf 'failure output changed unexpectedly:\n%s\n' "$failure_output" >&2; exit 1 ;;
esac
if [ "$(cat "$tmp/failure/repo/.ci-result/failure/status")" != "failed" ]; then
  printf 'failure aggregate status should be failed\n' >&2
  exit 1
fi

echo "upgrade smoke contract tests passed"
