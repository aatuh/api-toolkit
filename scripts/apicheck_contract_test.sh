#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/apicheck.sh"

tmp="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

make_fake_apidiff() {
  local bin_dir="$1"
  mkdir -p "$bin_dir"
  cat >"$bin_dir/apidiff" <<'FAKE'
#!/usr/bin/env sh
if [ -n "${APIDIFF_CALLS:-}" ]; then
  printf '%s\n' "$*" >>"$APIDIFF_CALLS"
fi
if [ "$1" = "-w" ]; then
  printf 'fake export\n' >"$2"
fi
exit 0
FAKE
  chmod +x "$bin_dir/apidiff"
}

init_repo() {
  local dir="$1"
  mkdir -p "$dir"
  git -C "$dir" init -q
  git -C "$dir" config user.email "apicheck-contract@example.invalid"
  git -C "$dir" config user.name "apicheck contract"
}

require_failure() {
  local name="$1"
  shift
  local output
  set +e
  output="$($@ 2>&1)"
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
  output="$($@ 2>&1)"
  local status=$?
  set -e
  if [ "$status" -ne 0 ]; then
    printf 'expected %s to pass, but it failed with %s\noutput:\n%s\n' "$name" "$status" "$output" >&2
    exit 1
  fi
  printf '%s' "$output"
}

run_script_in_dir() {
  local dir="$1"
  shift
  (cd "$dir" && env "$@" "$script")
}

missing_dir="$tmp/missing-required"
init_repo "$missing_dir"
missing_output="$(require_failure missing-required run_script_in_dir "$missing_dir" API_CHECK_REQUIRE_BASE=1)"
case "$missing_output" in
  *"API_BASE_REF is required"*) ;;
  *) printf 'missing baseline failure did not explain API_BASE_REF requirement:\n%s\n' "$missing_output" >&2; exit 1 ;;
esac

invalid_dir="$tmp/invalid-required"
init_repo "$invalid_dir"
touch "$invalid_dir/README.md"
git -C "$invalid_dir" add README.md
git -C "$invalid_dir" commit -qm initial
invalid_output="$(require_failure invalid-required run_script_in_dir "$invalid_dir" API_CHECK_REQUIRE_BASE=1 API_BASE_REF=does-not-exist)"
case "$invalid_output" in
  *"Base ref does-not-exist not found"*) ;;
  *) printf 'invalid baseline failure did not explain missing base ref:\n%s\n' "$invalid_output" >&2; exit 1 ;;
esac

fallback_skip_dir="$tmp/fallback-skip"
init_repo "$fallback_skip_dir"
fallback_skip_output="$(require_success fallback-skip run_script_in_dir "$fallback_skip_dir")"
case "$fallback_skip_output" in
  *"No base ref available. Skipping API check."*) ;;
  *) printf 'fallback skip output changed unexpectedly:\n%s\n' "$fallback_skip_output" >&2; exit 1 ;;
esac

success_dir="$tmp/success"
init_repo "$success_dir"
printf 'module github.com/aatuh/api-toolkit/v4\n\ngo 1.25.0\n' >"$success_dir/go.mod"
mkdir -p "$success_dir/ports"
printf 'package ports\n' >"$success_dir/ports/ports.go"
git -C "$success_dir" add go.mod ports/ports.go
git -C "$success_dir" commit -qm base
git -C "$success_dir" tag v-base
printf '\nconst Current = true\n' >>"$success_dir/ports/ports.go"
git -C "$success_dir" add ports/ports.go
git -C "$success_dir" commit -qm current
fake_bin="$tmp/bin"
make_fake_apidiff "$fake_bin"
export APIDIFF_CALLS="$tmp/apidiff.calls"
: >"$APIDIFF_CALLS"
require_success explicit-baseline run_script_in_dir "$success_dir" PATH="$fake_bin:$PATH" API_BASE_REF=v-base >/dev/null
if [ ! -s "$APIDIFF_CALLS" ]; then
  printf 'explicit baseline did not execute apidiff\n' >&2
  exit 1
fi
: >"$APIDIFF_CALLS"
require_success head-parent-fallback run_script_in_dir "$success_dir" PATH="$fake_bin:$PATH" >/dev/null
if [ ! -s "$APIDIFF_CALLS" ]; then
  printf 'HEAD~1 fallback did not execute apidiff\n' >&2
  exit 1
fi
