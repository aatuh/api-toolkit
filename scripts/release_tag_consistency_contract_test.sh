#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
gate="$repo_root/scripts/release_tag_consistency.sh"
tmp="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

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
}

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

write_fixture() {
  local fixture="$1"
  mkdir -p "$fixture/contrib" "$fixture/docs" "$fixture/.github/workflows"
  git init -q -b master "$fixture"
  git -C "$fixture" config user.name "release contract"
  git -C "$fixture" config user.email "release-contract@example.invalid"
  printf 'module github.com/aatuh/api-toolkit/v4\n\ngo 1.25.0\n' >"$fixture/go.mod"
  printf 'module github.com/aatuh/api-toolkit/contrib/v4\n\ngo 1.25.0\n' >"$fixture/contrib/go.mod"
  printf '# Changelog\n\n## [4.0.2] - fixture\n\n## [4.1.0-rc.1] - fixture\n' >"$fixture/CHANGELOG.md"
  printf '# Release notes\n\nv4.0.2\nv4.1.0-rc.1\n' >"$fixture/docs/release-notes.md"
  printf '# Support policy\n\nv4.0.2\nv4.1.0-rc.1\n' >"$fixture/docs/support-policy.md"
  printf 'env:\n  API_BASE_REF: v4.0.1\n' >"$fixture/.github/workflows/release.yml"
  git -C "$fixture" add -- CHANGELOG.md go.mod contrib/go.mod docs/release-notes.md docs/support-policy.md .github/workflows/release.yml
  git -C "$fixture" commit -qm "fixture release inputs"
  git -C "$fixture" tag v4.0.2
  git -C "$fixture" tag contrib/v4.0.2
  git -C "$fixture" tag v4.1.0-rc.1
  git -C "$fixture" tag contrib/v4.1.0-rc.1
}

run_gate() {
  local fixture="$1"
  local tag="$2"
  RELEASE_REPOSITORY_ROOT="$fixture" RELEASE_TAG="$tag" RELEASE_DEFAULT_BRANCH=master "$gate"
}

valid_fixture="$tmp/valid"
write_fixture "$valid_fixture"
require_success "valid stable release" run_gate "$valid_fixture" v4.0.2
require_success "valid release candidate" run_gate "$valid_fixture" v4.1.0-rc.1

mismatched_fixture="$tmp/mismatched"
write_fixture "$mismatched_fixture"
printf '\nfixture change\n' >>"$mismatched_fixture/docs/release-notes.md"
git -C "$mismatched_fixture" add -- docs/release-notes.md
git -C "$mismatched_fixture" commit -qm "separate contrib release"
git -C "$mismatched_fixture" tag -f contrib/v4.0.2 >/dev/null
output="$(require_failure "mismatched root and contrib tags" run_gate "$mismatched_fixture" v4.0.2)"
if [[ "$output" != *"must point at the same commit"* ]]; then
  printf 'mismatched tag failure did not explain the policy:\n%s\n' "$output" >&2
  exit 1
fi

wrong_module_fixture="$tmp/wrong-module"
write_fixture "$wrong_module_fixture"
printf 'module github.com/aatuh/api-toolkit/v3\n\ngo 1.25.0\n' >"$wrong_module_fixture/go.mod"
output="$(require_failure "wrong root module major" run_gate "$wrong_module_fixture" v4.0.2)"
if [[ "$output" != *"does not match v4.0.2"* ]]; then
  printf 'wrong module failure did not identify the tag mismatch:\n%s\n' "$output" >&2
  exit 1
fi

missing_changelog_fixture="$tmp/missing-changelog"
write_fixture "$missing_changelog_fixture"
printf '# Changelog\n\n## [4.0.1] - stale\n' >"$missing_changelog_fixture/CHANGELOG.md"
output="$(require_failure "missing changelog release" run_gate "$missing_changelog_fixture" v4.0.2)"
if [[ "$output" != *"CHANGELOG.md is missing"* ]]; then
  printf 'missing changelog failure was unclear:\n%s\n' "$output" >&2
  exit 1
fi

outdated_baseline_fixture="$tmp/outdated-baseline"
write_fixture "$outdated_baseline_fixture"
printf 'env:\n  API_BASE_REF: v3.1.2\n' >"$outdated_baseline_fixture/.github/workflows/release.yml"
output="$(require_failure "outdated release workflow baseline" run_gate "$outdated_baseline_fixture" v4.0.2)"
if [[ "$output" != *"outdated major"* ]]; then
  printf 'outdated baseline failure was unclear:\n%s\n' "$output" >&2
  exit 1
fi

printf 'release tag consistency contract tests passed\n'
