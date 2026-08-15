#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
checker="$repo_root/scripts/workflow_security_check.sh"
scratch_dir=$(mktemp -d)
trap 'rm -rf "$scratch_dir"' EXIT
workflow_dir="$scratch_dir/.github/workflows"
mkdir -p "$workflow_dir"

expect_pass() {
  if ! WORKFLOW_SECURITY_ROOT="$scratch_dir" bash "$checker" >/dev/null 2>&1; then
    echo "expected workflow policy fixture to pass" >&2
    exit 1
  fi
}

expect_fail() {
  if WORKFLOW_SECURITY_ROOT="$scratch_dir" bash "$checker" >/dev/null 2>&1; then
    echo "expected workflow policy fixture to fail" >&2
    exit 1
  fi
}

cat >"$workflow_dir/safe.yml" <<'EOF'
name: safe
on:
  pull_request:
permissions:
  contents: read
jobs:
  checks:
    runs-on: ubuntu-latest
    steps:
      - run: make docs-check
EOF
expect_pass

cat >>"$workflow_dir/safe.yml" <<'EOF'
pull_request_target:
EOF
expect_fail
sed -i '$d' "$workflow_dir/safe.yml"

cat >>"$workflow_dir/safe.yml" <<'EOF'
      - run: echo ${{ github.event.pull_request.title }}
EOF
expect_fail
sed -i '$d' "$workflow_dir/safe.yml"

cat >>"$workflow_dir/safe.yml" <<'EOF'
      - uses: actions/download-artifact@0123456789012345678901234567890123456789 # test v1
EOF
expect_fail
sed -i '$d' "$workflow_dir/safe.yml"

cat >>"$workflow_dir/safe.yml" <<'EOF'
      - uses: actions/cache@0123456789012345678901234567890123456789 # test v1
EOF
expect_fail

echo "workflow security contract tests passed"
