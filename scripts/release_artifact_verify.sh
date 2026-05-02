#!/usr/bin/env bash
# Verify downloaded draft-release assets before publication.
set -euo pipefail

asset_dir="${1:-.}"
identity_regexp="${COSIGN_CERTIFICATE_IDENTITY_REGEXP:-^https://github.com/aatuh/api-toolkit/\\.github/workflows/release\\.yml@refs/tags/v.*$}"
issuer="${COSIGN_CERTIFICATE_OIDC_ISSUER:-https://token.actions.githubusercontent.com}"
release_tag="${RELEASE_TAG:-}"
github_repo="${GITHUB_REPOSITORY:-aatuh/api-toolkit}"

asset_path() {
  local name="$1"

  if [ -f "$asset_dir/$name" ]; then
    printf '%s/%s' "$asset_dir" "$name"
    return 0
  fi
  if [ "$name" = "release-evidence-logs.tgz" ] && [ -f "$asset_dir/.ci-result/release-evidence/release-evidence-logs.tgz" ]; then
    printf '%s/.ci-result/release-evidence/release-evidence-logs.tgz' "$asset_dir"
    return 0
  fi
  return 1
}

require_asset() {
  local name="$1"
  local path

  if ! path="$(asset_path "$name")"; then
    echo "missing release asset: $name" >&2
    exit 1
  fi
  if [ ! -s "$path" ]; then
    echo "release asset is empty: $name" >&2
    exit 1
  fi
}

verify_manifest_checksums() {
  local manifest="$asset_dir/release-asset-manifest.tsv"

  if [ ! -s "$manifest" ]; then
    echo "release-asset-manifest.tsv is required for checksum verification" >&2
    exit 1
  fi
  (cd "$asset_dir" && sha256sum -c release-asset-manifest.tsv)
}

verify_log_archive() {
  local archive
  local listing
  local required_logs=(
    "tools.log"
    "lint.log"
    "vuln.log"
    "gosec.log"
    "ci-build-smoke.log"
    "release-api-check.log"
    "contrib-release-notes-check.log"
    "docs-check.log"
    "test.log"
    "test-race.log"
    "fuzz.log"
    "clean.log"
    "contrib-api-drift-report.log"
  )

  archive="$(asset_path "release-evidence-logs.tgz")"
  listing="$(tar -tzf "$archive")"
  for log_name in "${required_logs[@]}"; do
    if ! printf '%s\n' "$listing" | grep -Eq "(^|/)${log_name}$|^\\./${log_name}$"; then
      echo "release-evidence-logs.tgz missing $log_name" >&2
      exit 1
    fi
  done
}

verify_summary_expectations() {
  local summary="$asset_dir/release-check-summary.json"
  local expected=(
    "release-check-summary.json"
    "release-evidence-logs.tgz"
    "release-asset-manifest.tsv"
    "sbom-root.spdx.json"
    "sbom-contrib.spdx.json"
    "sbom-root.spdx.json.sig"
    "sbom-root.spdx.json.pem"
    "sbom-contrib.spdx.json.sig"
    "sbom-contrib.spdx.json.pem"
  )
  local subjects=(
    "release-check-summary.json"
    "sbom-root.spdx.json"
    "sbom-contrib.spdx.json"
  )

  for asset in "${expected[@]}"; do
    if ! grep -Fq "\"$asset\"" "$summary"; then
      echo "release-check-summary.json missing expected draft asset $asset" >&2
      exit 1
    fi
  done
  for subject in "${subjects[@]}"; do
    if ! grep -Fq "\"$subject\"" "$summary"; then
      echo "release-check-summary.json missing attestation subject $subject" >&2
      exit 1
    fi
  done
}

verify_sbom_signature() {
  local sbom="$1"
  local signature="$2"
  local certificate="$3"

  if ! command -v cosign >/dev/null 2>&1; then
    echo "cosign is required to verify SBOM signatures" >&2
    exit 1
  fi
  cosign verify-blob \
    --certificate "$asset_dir/$certificate" \
    --signature "$asset_dir/$signature" \
    --certificate-identity-regexp "$identity_regexp" \
    --certificate-oidc-issuer "$issuer" \
    "$asset_dir/$sbom" >/dev/null
}

verify_attestations_if_requested() {
  if [ -z "$release_tag" ]; then
    echo "RELEASE_TAG is not set; verified local attestation subjects only."
    echo "Set RELEASE_TAG and GITHUB_REPOSITORY to verify GitHub provenance attestations with gh."
    return 0
  fi
  if ! command -v gh >/dev/null 2>&1; then
    echo "gh is required when RELEASE_TAG is set for online provenance attestation verification" >&2
    exit 1
  fi

  gh attestation verify "$asset_dir/release-check-summary.json" --repo "$github_repo" --source-ref "refs/tags/$release_tag"
  gh attestation verify "$asset_dir/sbom-root.spdx.json" --repo "$github_repo" --source-ref "refs/tags/$release_tag"
  gh attestation verify "$asset_dir/sbom-contrib.spdx.json" --repo "$github_repo" --source-ref "refs/tags/$release_tag"
}

required_assets=(
  "release-check-summary.json"
  "release-evidence-logs.tgz"
  "release-asset-manifest.tsv"
  "sbom-root.spdx.json"
  "sbom-contrib.spdx.json"
  "sbom-root.spdx.json.sig"
  "sbom-root.spdx.json.pem"
  "sbom-contrib.spdx.json.sig"
  "sbom-contrib.spdx.json.pem"
)

for asset in "${required_assets[@]}"; do
  require_asset "$asset"
done

verify_manifest_checksums
verify_summary_expectations
verify_log_archive
verify_sbom_signature "sbom-root.spdx.json" "sbom-root.spdx.json.sig" "sbom-root.spdx.json.pem"
verify_sbom_signature "sbom-contrib.spdx.json" "sbom-contrib.spdx.json.sig" "sbom-contrib.spdx.json.pem"
verify_attestations_if_requested

echo "release artifact verification passed"
