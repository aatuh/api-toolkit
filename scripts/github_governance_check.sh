#!/usr/bin/env bash
set -euo pipefail

repo="${GITHUB_REPOSITORY:-aatuh/api-toolkit}"
branch="${GITHUB_DEFAULT_BRANCH:-master}"

if ! command -v gh >/dev/null 2>&1; then
  echo "github-governance-check: gh is not installed; skipping optional governance verification"
  exit 0
fi
if ! gh auth status >/dev/null 2>&1; then
  echo "github-governance-check: gh is not authenticated; skipping optional governance verification"
  exit 0
fi

fail=0
require_jq() {
  if ! command -v jq >/dev/null 2>&1; then
    echo "github-governance-check: jq is required when gh is authenticated" >&2
    exit 2
  fi
}

check_true() {
  local name="$1"
  local value="$2"
  if [ "$value" = "true" ]; then
    echo "PASS $name"
  else
    echo "FAIL $name" >&2
    fail=1
  fi
}

require_jq

branch_json="$(gh api "repos/$repo/branches/$branch" 2>/dev/null || true)"
if [ -z "$branch_json" ]; then
  echo "FAIL branch protection: unable to read repos/$repo/branches/$branch" >&2
  exit 1
fi

check_true "branch protected" "$(printf '%s' "$branch_json" | jq -r '.protected == true')"

protection_json="$(gh api "repos/$repo/branches/$branch/protection" 2>/dev/null || true)"
if [ -z "$protection_json" ]; then
  echo "FAIL branch protection details" >&2
  exit 1
fi

check_true "required status checks enabled" "$(printf '%s' "$protection_json" | jq -r '.required_status_checks != null')"
check_true "required pull request reviews enabled" "$(printf '%s' "$protection_json" | jq -r '.required_pull_request_reviews != null')"
check_true "CODEOWNERS review required" "$(printf '%s' "$protection_json" | jq -r '.required_pull_request_reviews.require_code_owner_reviews == true')"
check_true "force pushes disabled" "$(printf '%s' "$protection_json" | jq -r '.allow_force_pushes.enabled == false')"
check_true "deletions disabled" "$(printf '%s' "$protection_json" | jq -r '.allow_deletions.enabled == false')"

rulesets_json="$(gh api "repos/$repo/rulesets?includes_parents=true" 2>/dev/null || printf '[]')"
check_true "tag protection or rulesets configured" "$(printf '%s' "$rulesets_json" | jq -r '[.[] | select((.target == "tag") or (.name | test("tag"; "i")))] | length > 0')"

if [ "$fail" -ne 0 ]; then
  echo "github-governance-check: governance settings are incomplete" >&2
  exit 1
fi
echo "github-governance-check: governance settings verified"
