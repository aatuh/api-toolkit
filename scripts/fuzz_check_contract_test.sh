#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

fake_go="$tmp/go"
fuzz_dir="$tmp/fuzzpkg"
mkdir -p "$fuzz_dir"
cat >"$fuzz_dir/fuzz_test.go" <<'GO'
package fuzzpkg

import "testing"

func FuzzContractTarget(f *testing.F) {
	f.Fuzz(func(t *testing.T, _ string) {})
}
GO

cat >"$fake_go" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail

state="${FAKE_FUZZ_STATE:?}"
fuzz_dir="${FAKE_FUZZ_DIR:?}"
mode="${FAKE_FUZZ_MODE:-deadline-race}"

if [ "$1" = "list" ] && [ "$2" = "./..." ]; then
  printf 'example.test/fuzzpkg\n'
  exit 0
fi
if [ "$1" = "list" ] && [ "$2" = "-f" ]; then
  printf '%s\n' "$fuzz_dir"
  exit 0
fi
if [ "$1" = "test" ]; then
  count=0
  if [ -f "$state" ]; then
    count="$(cat "$state")"
  fi
  count=$((count + 1))
  printf '%s\n' "$count" >"$state"

  if [ "$mode" = "deadline-race" ] && [ "$count" -eq 1 ]; then
    cat <<'OUTPUT'
--- FAIL: FuzzContractTarget (10.00s)
    context deadline exceeded
FAIL
exit status 1
FAIL	example.test/fuzzpkg	10.00s
OUTPUT
    exit 1
  fi
  if [ "$mode" = "real-failure" ]; then
    cat <<'OUTPUT'
--- FAIL: FuzzContractTarget (0.01s)
    --- FAIL: FuzzContractTarget/seed#0 (0.00s)
        contract failure
Failing input written to testdata/fuzz/FuzzContractTarget/example
FAIL
exit status 1
FAIL	example.test/fuzzpkg	0.01s
OUTPUT
    exit 1
  fi
  printf 'ok\texample.test/fuzzpkg\t0.01s\n'
  exit 0
fi

printf 'unexpected fake go invocation: %s\n' "$*" >&2
exit 2
FAKE
chmod +x "$fake_go"

state="$tmp/deadline-race-count"
deadline_output="$(FAKE_FUZZ_STATE="$state" FAKE_FUZZ_DIR="$fuzz_dir" GO="$fake_go" FUZZTIME=1s FUZZ_DEADLINE_RETRIES=1 "$repo_root/scripts/fuzz_check.sh" . 2>&1)"
if [ "$(cat "$state")" != "2" ]; then
  printf 'deadline-race retry count = %s, want 2\noutput:\n%s\n' "$(cat "$state")" "$deadline_output" >&2
  exit 1
fi
case "$deadline_output" in
  *"known Go timed-fuzz deadline race"*) ;;
  *) printf 'deadline-race retry message missing:\n%s\n' "$deadline_output" >&2; exit 1 ;;
esac

state="$tmp/real-failure-count"
set +e
real_failure_output="$(FAKE_FUZZ_STATE="$state" FAKE_FUZZ_DIR="$fuzz_dir" FAKE_FUZZ_MODE=real-failure GO="$fake_go" FUZZTIME=1s FUZZ_DEADLINE_RETRIES=1 "$repo_root/scripts/fuzz_check.sh" . 2>&1)"
real_failure_status=$?
set -e
if [ "$real_failure_status" -eq 0 ]; then
  printf 'real fuzz failure unexpectedly passed:\n%s\n' "$real_failure_output" >&2
  exit 1
fi
if [ "$(cat "$state")" != "1" ]; then
  printf 'real fuzz failure was retried; count = %s\noutput:\n%s\n' "$(cat "$state")" "$real_failure_output" >&2
  exit 1
fi
case "$real_failure_output" in
  *"Failing input written to"*) ;;
  *) printf 'real fuzz failure output missing corpus evidence:\n%s\n' "$real_failure_output" >&2; exit 1 ;;
esac
