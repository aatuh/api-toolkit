#!/usr/bin/env bash
# Verify downloaded GitHub draft-release assets before publication.
set -euo pipefail

asset_dir="${1:-${RELEASE_ASSET_DIR:-.}}"
release_tag="${RELEASE_TAG:-}"
github_repo="${GITHUB_REPOSITORY:-aatuh/api-toolkit}"
verify_mode="${RELEASE_ARTIFACT_VERIFY_MODE:-publication}"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [ "$#" -gt 1 ]; then
  echo "usage: RELEASE_TAG=vX.Y.Z [GITHUB_REPOSITORY=owner/repo] $0 [asset-directory]" >&2
  exit 2
fi

case "$verify_mode" in
  local)
    RELEASE_ARTIFACT_VERIFY_MODE=local \
    bash "$repo_root/scripts/release_artifact_verify.sh" "$asset_dir"
    exit 0
    ;;
  publication) ;;
  *)
    echo "RELEASE_ARTIFACT_VERIFY_MODE must be local or publication, got: $verify_mode" >&2
    exit 2
    ;;
esac

if [ -z "$release_tag" ]; then
  echo "RELEASE_TAG is required to verify a published draft release" >&2
  exit 2
fi

if ! [[ "$release_tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] && \
  ! [[ "$release_tag" =~ ^v[0-9]+\.[0-9]+\.0-rc\.[0-9]+$ ]]; then
  echo "RELEASE_TAG must be vX.Y.Z or vX.Y.0-rc.N, got: $release_tag" >&2
  exit 2
fi

if ! [[ "$github_repo" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$ ]]; then
  echo "GITHUB_REPOSITORY must be an owner/repository value, got: $github_repo" >&2
  exit 2
fi

RELEASE_ARTIFACT_VERIFY_MODE=publication \
RELEASE_TAG="$release_tag" \
GITHUB_REPOSITORY="$github_repo" \
bash "$repo_root/scripts/release_artifact_verify.sh" "$asset_dir"
