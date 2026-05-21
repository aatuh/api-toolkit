#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/generated_upgrade_compat_check.sh"

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
if [ "${1:-}" = "worktree" ] && [ "${2:-}" = "add" ]; then
  generator_dir="${4:?}"
  generator_ref="${5:?}"
  mkdir -p "$generator_dir/contrib/cmd/api-toolkit"
  printf '%s\n' "$generator_ref" >"$generator_dir/.generator-ref"
  exit 0
fi
if [ "${1:-}" = "worktree" ]; then
  exit 0
fi
printf 'unexpected fake git call: %s\n' "$*" >&2
exit 2
FAKE
  cat >"$bin_dir/go" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
printf 'go %s\n' "$*" >>"${FAKE_TOOL_CALLS:?}"
if [ "${1:-}" = "run" ]; then
  service_dir=""
  for ((i=1; i <= $#; i++)); do
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
  cp ../.generator-ref "$service_dir/.generator-ref"
fi
exit 0
FAKE
  cat >"$bin_dir/make" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
printf 'make %s\n' "$*" >>"${FAKE_TOOL_CALLS:?}"
if [ -f .generator-ref ] && [ -n "${FAIL_REF:-}" ] && [ "$(cat .generator-ref)" = "$FAIL_REF" ]; then
  printf 'forced failure for %s\n' "$FAIL_REF" >&2
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
  mkdir -p "$dir"
  FAKE_TOOL_CALLS="$dir/calls" GENERATED_UPGRADE_COMPAT_REPO_ROOT="$dir/repo" GENERATED_UPGRADE_COMPAT_RESULT_DIR=".ci-result/$name" PATH="$tmp/bin:$PATH" "$@" "$script"
}

make_fake_tools "$tmp/bin"

default_output="$(require_success default-refs run_contract default env -u GENERATED_UPGRADE_COMPAT_REFS -u GENERATOR_REF)"
for required in "generated upgrade compatibility check passed for v3.0.0" "generated upgrade compatibility check passed for v3.1.2"; do
  case "$default_output" in
    *"$required"*) ;;
    *) printf 'default matrix output missing %q:\n%s\n' "$required" "$default_output" >&2; exit 1 ;;
  esac
done
default_status="$tmp/default/repo/.ci-result/default/status.tsv"
for required in $'v3.0.0\tpassed\t.ci-result/default/upgrade-compat-v3.0.0.log' $'v3.1.2\tpassed\t.ci-result/default/upgrade-compat-v3.1.2.log'; do
  if ! grep -Fqx "$required" "$default_status"; then
    printf 'default status missing %q:\n%s\n' "$required" "$(cat "$default_status")" >&2
    exit 1
  fi
done

explicit_output="$(require_success explicit-refs run_contract explicit env GENERATED_UPGRADE_COMPAT_REFS="v3.1.2")"
case "$explicit_output" in
  *"v3.1.2"* ) ;;
  *) printf 'explicit refs output changed unexpectedly:\n%s\n' "$explicit_output" >&2; exit 1 ;;
esac
if grep -q "v3.0.0" "$tmp/explicit/repo/.ci-result/explicit/status.tsv"; then
  printf 'explicit refs should not include default refs:\n%s\n' "$(cat "$tmp/explicit/repo/.ci-result/explicit/status.tsv")" >&2
  exit 1
fi

alias_output="$(require_success generator-ref-alias run_contract alias env -u GENERATED_UPGRADE_COMPAT_REFS GENERATOR_REF=v3.0.0)"
case "$alias_output" in
  *"v3.0.0"* ) ;;
  *) printf 'GENERATOR_REF alias output changed unexpectedly:\n%s\n' "$alias_output" >&2; exit 1 ;;
esac
if grep -q "v3.1.2" "$tmp/alias/repo/.ci-result/alias/status.tsv"; then
  printf 'GENERATOR_REF alias should not include default refs:\n%s\n' "$(cat "$tmp/alias/repo/.ci-result/alias/status.tsv")" >&2
  exit 1
fi

failure_output="$(require_failure failing-ref run_contract failure env GENERATED_UPGRADE_COMPAT_REFS="v-good v-bad" FAIL_REF=v-bad)"
case "$failure_output" in
  *"generated upgrade compatibility check failed for v-bad"* ) ;;
  *) printf 'failing ref output changed unexpectedly:\n%s\n' "$failure_output" >&2; exit 1 ;;
esac
failure_status="$tmp/failure/repo/.ci-result/failure/status.tsv"
for required in $'v-good\tpassed\t.ci-result/failure/upgrade-compat-v-good.log' $'v-bad\tfailed\t.ci-result/failure/upgrade-compat-v-bad.log'; do
  if ! grep -Fqx "$required" "$failure_status"; then
    printf 'failure status missing %q:\n%s\n' "$required" "$(cat "$failure_status")" >&2
    exit 1
  fi
done
if [ "$(cat "$tmp/failure/repo/.ci-result/failure/status")" != "failed" ]; then
  printf 'aggregate status should be failed\n' >&2
  exit 1
fi

echo "generated upgrade compatibility contract tests passed"
