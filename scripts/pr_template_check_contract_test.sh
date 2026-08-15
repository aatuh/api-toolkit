#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
checker="$repo_root/scripts/pr_template_check.sh"

valid_body=$(cat <<'EOF'
# Backlog ticket
- [x] Ticket ID: `GOV-003`
# Tests and verification
# Documentation
# Compatibility impact
- [x] No public effect
# Security impact
- [x] No new trust boundary or sensitive data handling.
# Dependency impact
- [x] No dependency, license, or supply-chain impact.
# Generated-file impact
- [x] No generated files changed.
# Benchmark impact
- [x] No performance-sensitive path changed.
# Migration impact
- [x] No migration required.
EOF
)

expect_accept() {
  local body=$1
  if ! PR_BODY="$body" bash "$checker" >/dev/null 2>&1; then
    echo "expected pull-request body to pass validation" >&2
    exit 1
  fi
}

expect_reject() {
  local body=$1
  if PR_BODY="$body" bash "$checker" >/dev/null 2>&1; then
    echo "expected pull-request body to fail validation" >&2
    exit 1
  fi
}

scratch_dir=$(mktemp -d)
trap 'rm -rf "$scratch_dir"' EXIT
probe_file="$scratch_dir/body-evaluation-probe"

expect_accept "$valid_body"
expect_accept "${valid_body/# Documentation/# Documentation
\$(touch $probe_file)}"
if [[ -e "$probe_file" ]]; then
  echo "pull-request body validation evaluated shell input" >&2
  exit 1
fi

expect_reject "${valid_body/"# Compatibility impact"/"## Compatibility impact"}"
expect_reject "${valid_body/"- [x] No public effect"/"- [ ] No public effect"}"
expect_reject "$valid_body"$'\n''- [x] Behavioral change'
expect_reject "${valid_body/GOV-003/NOT_A_TICKET}"

echo "pull-request template contract tests passed"
