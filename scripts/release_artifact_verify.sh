#!/usr/bin/env bash
# Verify downloaded draft-release assets before publication.
set -euo pipefail

asset_dir="${1:-.}"
identity_regexp="${COSIGN_CERTIFICATE_IDENTITY_REGEXP:-^https://github.com/aatuh/api-toolkit/\\.github/workflows/release\\.yml@refs/tags/v.*$}"
issuer="${COSIGN_CERTIFICATE_OIDC_ISSUER:-https://token.actions.githubusercontent.com}"
release_tag="${RELEASE_TAG:-}"
github_repo="${GITHUB_REPOSITORY:-aatuh/api-toolkit}"
verify_mode="${RELEASE_ARTIFACT_VERIFY_MODE:-local}"
expected_api_base_ref="${API_BASE_REF:-}"

case "$verify_mode" in
  local|publication) ;;
  *)
    echo "RELEASE_ARTIFACT_VERIFY_MODE must be local or publication, got: $verify_mode" >&2
    exit 2
    ;;
esac

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
    "v3-readiness-check.log"
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

verify_summary_invariants() {
  local summary
  local archive
  local listing_file

  summary="$(asset_path "release-check-summary.json")"
  archive="$(asset_path "release-evidence-logs.tgz")"
  listing_file="$(mktemp)"
  tar -tzf "$archive" >"$listing_file"

  SUMMARY_PATH="$summary" \
    ARCHIVE_LISTING_PATH="$listing_file" \
    EXPECTED_API_BASE_REF="$expected_api_base_ref" \
    python3 - <<'PY'
import json
import os
import sys


def fail(message):
    print(f"release-check-summary.json invariant failed: {message}", file=sys.stderr)
    sys.exit(1)


summary_path = os.environ["SUMMARY_PATH"]
listing_path = os.environ["ARCHIVE_LISTING_PATH"]
expected_api_base_ref = os.environ["EXPECTED_API_BASE_REF"]

with open(summary_path, "r", encoding="utf-8") as handle:
    summary = json.load(handle)

required_assets = [
    "release-check-summary.json",
    "release-evidence-logs.tgz",
    "release-asset-manifest.tsv",
    "sbom-root.spdx.json",
    "sbom-contrib.spdx.json",
    "sbom-root.spdx.json.sig",
    "sbom-root.spdx.json.pem",
    "sbom-contrib.spdx.json.sig",
    "sbom-contrib.spdx.json.pem",
]
required_subjects = [
    "release-check-summary.json",
    "sbom-root.spdx.json",
    "sbom-contrib.spdx.json",
]

if summary.get("schema") != "github.com/aatuh/api-toolkit/release-check-summary/v2":
    fail(f"schema={summary.get('schema')!r}, want release-check-summary/v2")
if summary.get("status") != "passed":
    fail(f"status={summary.get('status')!r}, want passed")
if summary.get("publication_eligible") is not True:
    fail("publication_eligible must be true")
summary_api_base_ref = summary.get("api_base_ref")
if not summary_api_base_ref:
    fail("api_base_ref must be recorded")
if expected_api_base_ref and summary_api_base_ref != expected_api_base_ref:
    fail(f"api_base_ref={summary_api_base_ref!r}, want {expected_api_base_ref!r}")

provenance = summary.get("provenance_policy") or {}
if provenance.get("status") != "passed":
    fail(f"provenance_policy.status={provenance.get('status')!r}, want passed")
if provenance.get("mode") != "publication":
    fail(f"provenance_policy.mode={provenance.get('mode')!r}, want publication")

git_state = summary.get("git_state") or {}
if git_state.get("dirty") is not False:
    fail("git_state.dirty must be false")
for field in ("staged_count", "unstaged_count", "untracked_count", "deleted_count"):
    if git_state.get(field) != 0:
        fail(f"git_state.{field}={git_state.get(field)!r}, want 0")

checks = summary.get("checks")
if not isinstance(checks, list) or not checks:
    fail("checks must be a non-empty array")
for check in checks:
    name = check.get("name") or "<unnamed>"
    if check.get("status") != "passed":
        fail(f"check {name} status={check.get('status')!r}, want passed")
    if check.get("log_available") is not True:
        fail(f"check {name} must have log_available=true")
    if not check.get("log_path"):
        fail(f"check {name} is missing log_path")

vulnerability = summary.get("vulnerability_evidence") or {}
if vulnerability.get("called_vulnerability_count") != 0:
    fail(f"called_vulnerability_count={vulnerability.get('called_vulnerability_count')!r}, want 0")
for field in ("missing_disposition_count", "expired_disposition_count"):
    if vulnerability.get(field) != 0:
        fail(f"vulnerability_evidence.{field}={vulnerability.get(field)!r}, want 0")

contrib = summary.get("contrib_drift") or {}
if contrib.get("status") not in ("passed", "failed_disposition_review"):
    fail(f"contrib_drift.status={contrib.get('status')!r}, want passed")
if contrib.get("status") != "passed":
    fail("contrib_drift must pass disposition review for publication")
if not contrib.get("artifact_path"):
    fail("contrib_drift.artifact_path is required")
for field in ("missing_disposition_count", "expired_disposition_count"):
    if contrib.get(field) != 0:
        fail(f"contrib_drift.{field}={contrib.get(field)!r}, want 0")

expectations = summary.get("publication_artifact_expectations") or {}
draft_assets = expectations.get("github_draft_release_assets") or []
subjects = expectations.get("github_attestation_subjects") or []
for asset in required_assets:
    if asset not in draft_assets:
        fail(f"publication_artifact_expectations.github_draft_release_assets missing {asset}")
for subject in required_subjects:
    if subject not in subjects:
        fail(f"publication_artifact_expectations.github_attestation_subjects missing {subject}")
if expectations.get("local_generates_signed_sboms") is not False:
    fail("local_generates_signed_sboms must be false")

sbom_assets = summary.get("sbom_assets") or []
for asset in required_assets[3:]:
    if asset not in sbom_assets:
        fail(f"sbom_assets missing {asset}")
if summary.get("sbom_status") not in ("not_generated", "generated_and_signed"):
    fail(f"sbom_status={summary.get('sbom_status')!r}, want not_generated or generated_and_signed")

archive_entries = set()
with open(listing_path, "r", encoding="utf-8") as handle:
    for raw in handle:
        entry = raw.strip()
        if not entry:
            continue
        normalized = entry[2:] if entry.startswith("./") else entry
        archive_entries.add(normalized)
        archive_entries.add(os.path.basename(normalized))


def archive_contains(summary_log_path):
    candidates = {
        summary_log_path,
        summary_log_path[2:] if summary_log_path.startswith("./") else summary_log_path,
        os.path.basename(summary_log_path),
    }
    marker = "/logs/"
    if marker in summary_log_path:
        candidates.add(summary_log_path.split(marker, 1)[1])
    return any(candidate in archive_entries for candidate in candidates if candidate)


for check in checks:
    if not archive_contains(check["log_path"]):
        fail(f"release-evidence-logs.tgz missing checks[].log_path {check['log_path']}")
if not archive_contains(contrib["artifact_path"]):
    fail(f"release-evidence-logs.tgz missing contrib_drift.artifact_path {contrib['artifact_path']}")
PY
  rm -f "$listing_file"
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
  if [ "$verify_mode" = "publication" ] && [ -z "$release_tag" ]; then
    echo "RELEASE_TAG is required when RELEASE_ARTIFACT_VERIFY_MODE=publication" >&2
    exit 1
  fi
  if [ -z "$release_tag" ]; then
    echo "RELEASE_TAG is not set; verified local attestation subjects only."
    echo "Set RELEASE_ARTIFACT_VERIFY_MODE=publication, RELEASE_TAG, and GITHUB_REPOSITORY to verify GitHub provenance attestations with gh."
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
verify_log_archive
verify_summary_invariants
verify_sbom_signature "sbom-root.spdx.json" "sbom-root.spdx.json.sig" "sbom-root.spdx.json.pem"
verify_sbom_signature "sbom-contrib.spdx.json" "sbom-contrib.spdx.json.sig" "sbom-contrib.spdx.json.pem"
verify_attestations_if_requested

echo "release artifact verification passed"
