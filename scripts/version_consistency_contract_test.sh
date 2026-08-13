#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/version_consistency_check.sh"
tmp="$(mktemp -d)"
cleanup() { rm -rf "$tmp"; }
trap cleanup EXIT

fixture="$tmp/repo"
mkdir -p "$fixture/docs"
for path in README.md SECURITY.md VERSIONING.md CONTRIBUTING.md ROADMAP.md docs/README.md docs/support-policy.md docs/production-readiness.md; do
  mkdir -p "$(dirname "$fixture/$path")"
  printf 'current v4 guidance\n' >"$fixture/$path"
done
printf '%s\n' \
  'VERIFIED_V4_BASE_REF is deliberately unset.' \
  '**Status:** Open — no v4 tag is approved.' >"$fixture/docs/release-incident-v4-release-identity.md"
printf '%s\n' \
  'VERIFIED_V4_BASE_REF' \
  'latest-published-tag fallback is forbidden' >"$fixture/docs/release-runbook.md"
printf '%s\n' \
  'v4.0.0 and v4.0.1 are not approved release baselines' >"$fixture/README.md"
cat >"$fixture/docs/version-consistency-current-paths.txt" <<'PATHS'
README.md
SECURITY.md
VERSIONING.md
CONTRIBUTING.md
ROADMAP.md
docs/README.md
docs/support-policy.md
docs/production-readiness.md
docs/release-runbook.md
PATHS
cat >"$fixture/docs/version-consistency-historical-allowlist.txt" <<'ALLOWLIST'
# Historical release record.
docs/history.md
ALLOWLIST
printf 'latest v3 release was archived\n' >"$fixture/docs/history.md"

if ! VERSION_CONSISTENCY_ROOT="$fixture" "$script" >/dev/null; then
  printf 'expected clean version-consistency fixture to pass\n' >&2
  exit 1
fi
printf 'use github.com/aatuh/api-toolkit/v3/httpx\n' >>"$fixture/README.md"
if VERSION_CONSISTENCY_ROOT="$fixture" "$script" >/dev/null 2>&1; then
  printf 'expected stale v3 import to fail the version-consistency gate\n' >&2
  exit 1
fi

echo 'version consistency contract tests passed'
