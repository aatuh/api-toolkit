#!/usr/bin/env bash
set -euo pipefail

repo="${GITHUB_REPOSITORY:-aatuh/api-toolkit}"
branch="${GITHUB_DEFAULT_BRANCH:-master}"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v gh >/dev/null 2>&1; then
  echo "github-governance-check: gh is not installed; skipping optional governance verification"
  exit 0
fi
if ! gh auth status >/dev/null 2>&1; then
  echo "github-governance-check: gh is not authenticated; skipping optional governance verification"
  exit 0
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "github-governance-check: jq is required when gh is authenticated" >&2
  exit 2
fi
if ! command -v git >/dev/null 2>&1; then
  echo "github-governance-check: git is required when gh is authenticated" >&2
  exit 2
fi

if [[ ! "$repo" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
  echo "github-governance-check: GITHUB_REPOSITORY must be an owner/repository pair" >&2
  exit 2
fi
repo_owner="${repo%%/*}"
repo_name="${repo#*/}"
if [[ "$repo_owner" == "." || "$repo_owner" == ".." || "$repo_name" == "." || "$repo_name" == ".." ]]; then
  echo "github-governance-check: GITHUB_REPOSITORY contains an invalid path segment" >&2
  exit 2
fi
if [ "${#branch}" -gt 255 ] || ! git check-ref-format --branch "$branch" >/dev/null 2>&1; then
  echo "github-governance-check: GITHUB_DEFAULT_BRANCH is not a valid branch name" >&2
  exit 2
fi
branch_encoded="$(jq -nr --arg value "$branch" '$value | @uri')"

fetch_json() {
  local endpoint="$1"
  local description="$2"
  local response
  if ! response="$(gh api --method GET "$endpoint" 2>/dev/null)"; then
    echo "FAIL $description: authenticated GitHub API request failed" >&2
    return 1
  fi
  if [ "${#response}" -gt 1048576 ]; then
    echo "FAIL $description: GitHub API response exceeds 1 MiB" >&2
    return 1
  fi
  if ! printf '%s' "$response" | jq -e . >/dev/null 2>&1; then
    echo "FAIL $description: GitHub API returned malformed JSON" >&2
    return 1
  fi
  printf '%s' "$response"
}

fail=0
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

"$script_dir/required_checks_verify.sh"

branch_json="$(fetch_json "repos/$repo_owner/$repo_name/branches/$branch_encoded" "branch protection")" || exit 1
if ! printf '%s' "$branch_json" | jq -e 'type == "object" and (.protected | type == "boolean")' >/dev/null; then
  echo "FAIL branch protection: response shape is invalid" >&2
  exit 1
fi
check_true "branch protected" "$(printf '%s' "$branch_json" | jq -r '.protected == true')"

protection_json="$(fetch_json "repos/$repo_owner/$repo_name/branches/$branch_encoded/protection" "branch protection details")" || exit 1
if ! printf '%s' "$protection_json" | "$script_dir/required_checks_verify.sh" --branch-protection -; then
  fail=1
fi
check_true "required status checks enabled" "$(printf '%s' "$protection_json" | jq -r '.required_status_checks != null')"
check_true "required status checks use strict branch updates" "$(printf '%s' "$protection_json" | jq -r '.required_status_checks.strict == true')"
check_true "admin enforcement enabled" "$(printf '%s' "$protection_json" | jq -r '.enforce_admins.enabled == true')"
check_true "linear history enabled" "$(printf '%s' "$protection_json" | jq -r '.required_linear_history.enabled == true')"
check_true "force pushes disabled" "$(printf '%s' "$protection_json" | jq -r '.allow_force_pushes.enabled == false')"
check_true "deletions disabled" "$(printf '%s' "$protection_json" | jq -r '.allow_deletions.enabled == false')"

rulesets_json="$(fetch_json "repos/$repo_owner/$repo_name/rulesets?includes_parents=true&per_page=100" "repository rulesets")" || exit 1
if ! printf '%s' "$rulesets_json" | jq -e '
  type == "array" and
  length > 0 and
  length <= 100 and
  all(.[]; (.id | type == "number" and floor == . and . > 0)) and
  ([.[].id] | unique | length) == length
' >/dev/null; then
  echo "FAIL repository rulesets: response shape is invalid" >&2
  exit 1
fi

rulesets_detail="$({
  printf '['
  first=1
  while IFS= read -r ruleset_id; do
    detail="$(fetch_json "repos/$repo_owner/$repo_name/rulesets/$ruleset_id" "ruleset detail")" || exit 1
    if ! printf '%s' "$detail" | jq -e 'type == "object"' >/dev/null; then
      echo "FAIL ruleset detail: response shape is invalid" >&2
      exit 1
    fi
    if [ "$first" -eq 0 ]; then
      printf ','
    fi
    first=0
    printf '%s' "$detail"
  done < <(printf '%s' "$rulesets_json" | jq -r '.[].id')
  printf ']'
})" || exit 1

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
