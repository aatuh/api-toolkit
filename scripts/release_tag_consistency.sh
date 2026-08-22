#!/usr/bin/env bash
set -euo pipefail

# Enforce the current root/contrib release policy. This intentionally uses
# explicit tags only: selecting a "latest" tag could silently pair unrelated
# releases. A future independent-module policy requires a checked-in ADR with
# the approval marker below, rather than a broad environment-variable bypass.
repo_root="${RELEASE_REPOSITORY_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
if [ ! -d "$repo_root" ] || ! git -C "$repo_root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "RELEASE_REPOSITORY_ROOT must name a Git worktree" >&2
  exit 2
fi

release_tag="${RELEASE_TAG:-${GITHUB_REF_NAME:-}}"
if [[ ! "$release_tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] && \
  [[ ! "$release_tag" =~ ^v[0-9]+\.[0-9]+\.0-rc\.[0-9]+$ ]]; then
  echo "RELEASE_TAG must be vX.Y.Z or vX.Y.0-rc.N" >&2
  exit 2
fi

release_version="${release_tag#v}"
release_major="${release_version%%.*}"
contrib_release_tag="${CONTRIB_RELEASE_TAG:-contrib/$release_tag}"
if [ "$contrib_release_tag" != "contrib/$release_tag" ]; then
  echo "CONTRIB_RELEASE_TAG must be the matching contrib/$release_tag tag" >&2
  exit 2
fi

release_default_branch="${RELEASE_DEFAULT_BRANCH:-}"
if [ -z "$release_default_branch" ]; then
  release_default_branch="$(git -C "$repo_root" symbolic-ref --quiet --short refs/remotes/origin/HEAD 2>/dev/null || true)"
fi
if [ -z "$release_default_branch" ] && git -C "$repo_root" show-ref --verify --quiet refs/remotes/origin/master; then
  release_default_branch="origin/master"
fi
if [ -z "$release_default_branch" ] && git -C "$repo_root" show-ref --verify --quiet refs/heads/master; then
  release_default_branch="master"
fi
if [ -z "$release_default_branch" ] && git -C "$repo_root" show-ref --verify --quiet refs/heads/main; then
  release_default_branch="main"
fi
if [ -z "$release_default_branch" ]; then
  release_default_branch="$(git -C "$repo_root" symbolic-ref --quiet --short HEAD 2>/dev/null || true)"
fi
if [[ ! "$release_default_branch" =~ ^[A-Za-z0-9][A-Za-z0-9._/-]*$ ]] || \
  [[ "$release_default_branch" == *..* ]] || [[ "$release_default_branch" == *//* ]] || \
  [[ "$release_default_branch" == *'@{'* ]] || [[ "$release_default_branch" == */ ]] || \
  [[ "$release_default_branch" == *. ]]; then
  echo "RELEASE_DEFAULT_BRANCH is not a safe Git ref" >&2
  exit 2
fi

tag_commit() {
  git -C "$repo_root" rev-parse --verify "$1^{commit}" 2>/dev/null || true
}

root_tag_commit="$(tag_commit "$release_tag")"
contrib_tag_commit="$(tag_commit "$contrib_release_tag")"
default_branch_commit="$(tag_commit "$release_default_branch")"
if [ -z "$root_tag_commit" ]; then
  echo "root release tag $release_tag does not resolve to a commit" >&2
  exit 1
fi
if [ -z "$contrib_tag_commit" ]; then
  echo "contrib release tag $contrib_release_tag does not resolve to a commit" >&2
  exit 1
fi
if [ -z "$default_branch_commit" ]; then
  echo "default branch $release_default_branch does not resolve to a commit" >&2
  exit 1
fi

if [ "$root_tag_commit" != "$contrib_tag_commit" ]; then
  module_policy_adr="${RELEASE_MODULE_POLICY_ADR:-}"
  if [[ ! "$module_policy_adr" =~ ^docs/adr/[A-Za-z0-9][A-Za-z0-9._/-]*\.md$ ]] || \
    [[ "$module_policy_adr" == *..* ]] || [[ "$module_policy_adr" == *//* ]] || \
    [ ! -f "$repo_root/$module_policy_adr" ] || \
    ! grep -Fqx "Release module policy: independent releases approved" "$repo_root/$module_policy_adr"; then
    echo "root tag $release_tag and contrib tag $contrib_release_tag must point at the same commit; an approved docs/adr policy is required for independent releases" >&2
    exit 1
  fi
fi

if ! git -C "$repo_root" merge-base --is-ancestor "$root_tag_commit" "$default_branch_commit"; then
  echo "root release tag $release_tag is not reachable from $release_default_branch" >&2
  exit 1
fi
if ! git -C "$repo_root" merge-base --is-ancestor "$contrib_tag_commit" "$default_branch_commit"; then
  echo "contrib release tag $contrib_release_tag is not reachable from $release_default_branch" >&2
  exit 1
fi

module_path() {
  awk '$1 == "module" { print $2; exit }' "$1"
}

root_module_path="$(module_path "$repo_root/go.mod")"
contrib_module_path="$(module_path "$repo_root/contrib/go.mod")"
expected_root_module="github.com/aatuh/api-toolkit/v$release_major"
expected_contrib_module="github.com/aatuh/api-toolkit/contrib/v$release_major"
if [ "$root_module_path" != "$expected_root_module" ]; then
  echo "root go.mod module path $root_module_path does not match $release_tag (expected $expected_root_module)" >&2
  exit 1
fi
if [ "$contrib_module_path" != "$expected_contrib_module" ]; then
  echo "contrib go.mod module path $contrib_module_path does not match $contrib_release_tag (expected $expected_contrib_module)" >&2
  exit 1
fi

if ! grep -Eq "^## \[${release_version}\]([[:space:]-]|$)" "$repo_root/CHANGELOG.md"; then
  echo "CHANGELOG.md is missing a release heading for $release_tag" >&2
  exit 1
fi
if ! grep -Fq "$release_tag" "$repo_root/docs/release-notes.md"; then
  echo "docs/release-notes.md is missing $release_tag" >&2
  exit 1
fi
if ! grep -Fq "$release_tag" "$repo_root/docs/support-policy.md"; then
  echo "docs/support-policy.md is missing $release_tag" >&2
  exit 1
fi

workflow_path="$repo_root/.github/workflows/release.yml"
api_base_ref="$(awk '/^[[:space:]]*API_BASE_REF:[[:space:]]*/ { print $2; exit }' "$workflow_path")"
if [[ ! "$api_base_ref" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "release workflow API_BASE_REF must be a stable vX.Y.Z tag" >&2
  exit 1
fi
api_base_major="${api_base_ref#v}"
api_base_major="${api_base_major%%.*}"
if [ "$api_base_major" != "$release_major" ]; then
  echo "release workflow API_BASE_REF=$api_base_ref is an outdated major for $release_tag" >&2
  exit 1
fi

printf 'release tag coherence passed: root=%s contrib=%s commit=%s branch=%s baseline=%s\n' \
  "$release_tag" "$contrib_release_tag" "$root_tag_commit" "$release_default_branch" "$api_base_ref"
