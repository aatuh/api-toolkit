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

manifest="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/docs/required-checks.json"
"$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/required_checks_verify.sh"

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
required_contexts="$(printf '%s' "$protection_json" | jq -r '.required_status_checks.contexts[]?')"
while IFS= read -r check_name; do
  [ -n "$check_name" ] || continue
  if printf '%s\n' "$required_contexts" | grep -Fxq "$check_name"; then
    echo "PASS required check $check_name"
  else
    echo "FAIL required check $check_name" >&2
    fail=1
  fi
done < <(jq -r '.[] | select(.required_for_pr) | .check_name' "$manifest")
check_true "admin enforcement enabled" "$(printf '%s' "$protection_json" | jq -r '.enforce_admins.enabled == true')"
check_true "linear history enabled" "$(printf '%s' "$protection_json" | jq -r '.required_linear_history.enabled == true')"
check_true "force pushes disabled" "$(printf '%s' "$protection_json" | jq -r '.allow_force_pushes.enabled == false')"
check_true "deletions disabled" "$(printf '%s' "$protection_json" | jq -r '.allow_deletions.enabled == false')"

rulesets_json="$(gh api "repos/$repo/rulesets?includes_parents=true" 2>/dev/null || printf '[]')"
rulesets_detail="$(
  printf '['
  first=1
  while IFS= read -r ruleset_id; do
    [ -n "$ruleset_id" ] || continue
    detail="$(gh api "repos/$repo/rulesets/$ruleset_id" 2>/dev/null || true)"
    [ -n "$detail" ] || continue
    if [ "$first" -eq 0 ]; then
      printf ','
    fi
    first=0
    printf '%s' "$detail"
  done < <(printf '%s' "$rulesets_json" | jq -r '.[].id')
  printf ']'
)"
master_ref="refs/heads/$branch"
check_true "sole-maintainer pull request gate configured" "$(printf '%s' "$rulesets_detail" | jq -r --arg ref "$master_ref" '[.[] | select(.target == "branch" and (.conditions.ref_name.include | index($ref))) | .rules[]? | select(.type == "pull_request") | select(.parameters.required_approving_review_count == 0 and .parameters.require_code_owner_review == false and .parameters.require_last_push_approval == false and .parameters.required_review_thread_resolution == true)] | length > 0')"
check_true "master rulesets have no bypass actors" "$(printf '%s' "$rulesets_detail" | jq -r --arg ref "$master_ref" '[.[] | select(.target == "branch" and (.conditions.ref_name.include | index($ref)))] | length > 0 and all(.[]; (.bypass_actors | length) == 0)')"
check_true "CodeQL merge protection configured" "$(printf '%s' "$rulesets_detail" | jq -r --arg ref "$master_ref" '[.[] | select(.target == "branch" and (.conditions.ref_name.include | index($ref))) | .rules[]? | select(.type == "code_scanning") | .parameters.code_scanning_tools[]? | select(.tool == "CodeQL" and .alerts_threshold == "errors_and_warnings" and .security_alerts_threshold == "high_or_higher")] | length > 0')"
check_true "tag protection or rulesets configured" "$(printf '%s' "$rulesets_detail" | jq -r '[.[] | select(.target == "tag")] | length > 0')"
check_true "root release tags protected" "$(printf '%s' "$rulesets_detail" | jq -r '[.[] | select(.target == "tag") | .conditions.ref_name.include[]? | select(. == "refs/tags/v*")] | length > 0')"
check_true "contrib release tags protected" "$(printf '%s' "$rulesets_detail" | jq -r '[.[] | select(.target == "tag") | .conditions.ref_name.include[]? | select(. == "refs/tags/contrib/v*")] | length > 0')"

if [ "$fail" -ne 0 ]; then
  echo "github-governance-check: governance settings are incomplete" >&2
  exit 1
fi
echo "github-governance-check: governance settings verified"
