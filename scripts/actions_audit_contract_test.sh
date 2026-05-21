#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/actions_audit.sh"

tmp="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

current_checkout="actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # actions/checkout v6.0.2"
current_setup_go="actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # actions/setup-go v6.4.0"

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

write_repo() {
  local dir="$1"
  local workflow_uses="$2"
  local generator_checkout="$3"
  local generator_setup_go="$4"
  mkdir -p "$dir/.github/workflows" "$dir/contrib/cmd/api-toolkit"
  cat >"$dir/.github/workflows/ci.yml" <<EOF_WORKFLOW
name: ci
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: $workflow_uses
EOF_WORKFLOW
  cat >"$dir/contrib/cmd/api-toolkit/main.go" <<EOF_GENERATOR
package main

const ciWorkflowTemplate = \`
      - uses: $generator_checkout
      - uses: $generator_setup_go
\`
EOF_GENERATOR
}

pass_repo="$tmp/pass"
write_repo "$pass_repo" "${current_checkout%% #*}" "$current_checkout" "$current_setup_go"
pass_output="$(require_success pass env ACTIONS_AUDIT_ROOT="$pass_repo" "$script")"
case "$pass_output" in
  *"actions-audit: passed"*) ;;
  *) printf 'pass output missing success marker:\n%s\n' "$pass_output" >&2; exit 1 ;;
esac

unpinned_repo="$tmp/unpinned"
write_repo "$unpinned_repo" "actions/checkout@v6" "$current_checkout" "$current_setup_go"
unpinned_output="$(require_failure unpinned env ACTIONS_AUDIT_ROOT="$unpinned_repo" "$script")"
case "$unpinned_output" in
  *"unpinned action ref actions/checkout@v6"*) ;;
  *) printf 'unpinned output missing finding:\n%s\n' "$unpinned_output" >&2; exit 1 ;;
esac

deprecated_repo="$tmp/deprecated"
write_repo "$deprecated_repo" "actions/attest-build-provenance@a2bbfa25375fe432b6a289bc6b6cd05ecd0c4c32 # actions/attest-build-provenance v1" "$current_checkout" "$current_setup_go"
deprecated_output="$(require_failure deprecated env ACTIONS_AUDIT_ROOT="$deprecated_repo" "$script")"
case "$deprecated_output" in
  *"deprecated action comment actions/attest-build-provenance v1"*) ;;
  *) printf 'deprecated output missing finding:\n%s\n' "$deprecated_output" >&2; exit 1 ;;
esac

stale_repo="$tmp/stale"
write_repo "$stale_repo" "${current_checkout%% #*}" "actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # actions/checkout v4" "actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # actions/setup-go v5"
stale_output="$(require_failure stale env ACTIONS_AUDIT_ROOT="$stale_repo" "$script")"
case "$stale_output" in
  *"stale generated checkout action"*"+want $current_checkout"* | *"stale generated checkout action"* ) ;;
  *) printf 'stale output missing checkout finding:\n%s\n' "$stale_output" >&2; exit 1 ;;
esac
case "$stale_output" in
  *"stale generated setup-go action"* ) ;;
  *) printf 'stale output missing setup-go finding:\n%s\n' "$stale_output" >&2; exit 1 ;;
esac

echo "actions audit contract tests passed"
