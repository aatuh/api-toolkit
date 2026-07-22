#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
checker="$repo_root/scripts/pr_title_check.sh"

expect_accept() {
  local title=$1
  if ! PR_TITLE="$title" bash "$checker" >/dev/null 2>&1; then
    echo "expected title to pass validation" >&2
    exit 1
  fi
}

expect_reject() {
  local title=$1
  if PR_TITLE="$title" bash "$checker" >/dev/null 2>&1; then
    echo "expected title to fail validation" >&2
    exit 1
  fi
}

scratch_dir=$(mktemp -d)
trap 'rm -rf "$scratch_dir"' EXIT
probe_file="$scratch_dir/title-evaluation-probe"

expect_accept "docs: add contribution guidance"
expect_accept "feat(idempotency): add safe defaults"
expect_accept "refactor!: remove legacy constructor"
expect_accept "fix: preserve \$(touch $probe_file)"
if [[ -e "$probe_file" ]]; then
  echo "title validation evaluated shell input" >&2
  exit 1
fi

expect_reject ""
expect_reject "Add contribution guidance"
expect_reject "docs add contribution guidance"
expect_reject "feat(scope):"
expect_reject $'fix: contains\na second line'
expect_reject "feat(UPPER): reject invalid scope"

echo "pull-request title contract tests passed"
