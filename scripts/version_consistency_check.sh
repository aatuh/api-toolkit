#!/usr/bin/env bash
set -euo pipefail

# Reject stale current-version prose while retaining explicit historical
# migration and evidence records. This gate is intentionally fail-closed: an
# open release-identity incident is not permission to select a fallback tag.

repo_root="${VERSION_CONSISTENCY_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
current_paths_file="$repo_root/docs/version-consistency-current-paths.txt"
allowlist_file="$repo_root/docs/version-consistency-historical-allowlist.txt"
incident_file="$repo_root/docs/release-incident-v4-release-identity.md"

repo_root="$(cd "$repo_root" && pwd -P)"
for required in "$current_paths_file" "$allowlist_file" "$incident_file"; do
  if [ ! -f "$required" ]; then
    printf 'version consistency input is missing: %s\n' "$required" >&2
    exit 2
  fi
done

is_allowlisted() {
  local path="$1"
  grep -Fqx -- "$path" "$allowlist_file"
}

check_current_document() {
  local path="$1"
  local full_path="$repo_root/$path"
  if [ ! -f "$full_path" ]; then
    printf 'current guidance document is missing: %s\n' "$path" >&2
    return 1
  fi
  if rg -n -i 'github\.com/aatuh/api-toolkit/v3\b|\b(latest|current)[^\n]{0,64}\bv3\b' "$full_path" >&2; then
    printf 'stale v3 guidance in current document: %s\n' "$path" >&2
    return 1
  fi
}

failed=0
while IFS= read -r path; do
  path="${path%%#*}"
  path="${path//[$'\t\r\n ']/}"
  [ -z "$path" ] && continue
  check_current_document "$path" || failed=1
done <"$current_paths_file"

while IFS= read -r full_path; do
  relative_path="${full_path#"$repo_root"/}"
  case "$relative_path" in .audits/*|docs/site/*) continue ;; esac
  if is_allowlisted "$relative_path"; then
    continue
  fi
  if rg -n -i 'github\.com/aatuh/api-toolkit/v3\b|\b(latest|current)[^\n]{0,64}\bv3\b' "$full_path" >&2; then
    printf 'historical version reference requires an allowlist entry: %s\n' "$relative_path" >&2
    failed=1
  fi
done < <(find "$repo_root" -path "$repo_root/.git" -prune -o -type f -name '*.md' -print)

if ! rg -Fq 'VERIFIED_V4_BASE_REF' "$incident_file"; then
  printf 'release incident must state VERIFIED_V4_BASE_REF policy\n' >&2
  failed=1
fi
if rg -q '^\*\*Status:\*\* Open' "$incident_file"; then
  if ! rg -Fq 'deliberately unset' "$incident_file"; then
    printf 'open release incident must state that VERIFIED_V4_BASE_REF is unset\n' >&2
    failed=1
  fi
  if rg -n 'API_BASE_REF=v4\.0\.[01]' "$repo_root/docs/release-runbook.md" >&2; then
    printf 'open release incident forbids v4.0.0/v4.0.1 release command baselines\n' >&2
    failed=1
  fi
fi
if ! rg -Fq 'VERIFIED_V4_BASE_REF' "$repo_root/docs/release-runbook.md" || ! rg -Fq 'latest-published-tag fallback is forbidden' "$repo_root/docs/release-runbook.md"; then
  printf 'release runbook must name the verified v4 baseline and forbid fallback tags\n' >&2
  failed=1
fi
if ! rg -Fq 'not approved release baselines' "$repo_root/README.md"; then
  printf 'README must link the v4 incident and reject unverified release baselines\n' >&2
  failed=1
fi

if [ "$failed" -ne 0 ]; then
  exit 1
fi
printf 'current-version consistency check passed\n'
