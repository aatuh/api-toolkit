#!/usr/bin/env bash
# Verify downloaded draft-release assets before publication.
set -euo pipefail

asset_dir="${1:-.}"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
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
  MANIFEST_PATH="$manifest" ASSET_DIR="$asset_dir" python3 - <<'PY'
import os
import sys


def fail(message):
    print(f"release-asset-manifest.tsv invariant failed: {message}", file=sys.stderr)
    sys.exit(1)


asset_dir = os.environ["ASSET_DIR"]
manifest_path = os.environ["MANIFEST_PATH"]
expected = {
    "release-check-summary.json",
    "release-evidence-logs.tgz",
    "sbom-root.spdx.json",
    "sbom-contrib.spdx.json",
    "dependency-licenses-root.tsv",
    "dependency-licenses-contrib.tsv",
    "sbom-root.spdx.json.sig",
    "sbom-root.spdx.json.pem",
    "sbom-contrib.spdx.json.sig",
    "sbom-contrib.spdx.json.pem",
}
seen = set()

with open(manifest_path, "r", encoding="utf-8") as handle:
    for line_number, raw in enumerate(handle, 1):
        line = raw.rstrip("\n")
        if not line:
            fail(f"empty row at line {line_number}")
        parts = line.split(None, 1)
        if len(parts) != 2:
            fail(f"malformed row at line {line_number}")
        digest, name = parts
        name = name.lstrip(" *")
        if len(digest) != 64 or any(char not in "0123456789abcdef" for char in digest.lower()):
            fail(f"invalid sha256 at line {line_number}")
        if name not in expected:
            fail(f"unrecognized asset {name!r} at line {line_number}")
        if name in seen:
            fail(f"duplicate asset {name!r}")
        seen.add(name)

if seen != expected:
    fail(f"asset set {sorted(seen)!r}, want {sorted(expected)!r}")

top_level_files = {
    entry.name
    for entry in os.scandir(asset_dir)
    if entry.is_file(follow_symlinks=False)
}
allowed_files = expected | {"release-asset-manifest.tsv"}
unexpected = top_level_files - allowed_files
if unexpected:
    fail(f"unrecognized downloaded asset(s): {sorted(unexpected)!r}")
missing = allowed_files - top_level_files
if missing:
    fail(f"missing downloaded asset(s): {sorted(missing)!r}")
PY
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
    EXPECTED_RELEASE_TAG="$release_tag" \
    VERIFY_MODE="$verify_mode" \
    SOURCE_REPOSITORY="${RELEASE_SOURCE_DIR:-$repo_root}" \
    python3 - <<'PY'
import json
import os
import re
import subprocess
import sys


def fail(message):
    print(f"release-check-summary.json invariant failed: {message}", file=sys.stderr)
    sys.exit(1)


summary_path = os.environ["SUMMARY_PATH"]
listing_path = os.environ["ARCHIVE_LISTING_PATH"]
expected_api_base_ref = os.environ["EXPECTED_API_BASE_REF"]
expected_release_tag = os.environ["EXPECTED_RELEASE_TAG"]
verify_mode = os.environ["VERIFY_MODE"]
source_repository = os.environ["SOURCE_REPOSITORY"]

with open(summary_path, "r", encoding="utf-8") as handle:
    summary = json.load(handle)

required_assets = [
    "release-check-summary.json",
    "release-evidence-logs.tgz",
    "release-asset-manifest.tsv",
    "sbom-root.spdx.json",
    "sbom-contrib.spdx.json",
    "dependency-licenses-root.tsv",
    "dependency-licenses-contrib.tsv",
    "sbom-root.spdx.json.sig",
    "sbom-root.spdx.json.pem",
    "sbom-contrib.spdx.json.sig",
    "sbom-contrib.spdx.json.pem",
]
required_subjects = list(required_assets)

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

api_compatibility = summary.get("api_compatibility") or {}
if api_compatibility.get("previous_tag") != summary_api_base_ref:
    fail(f"api_compatibility.previous_tag={api_compatibility.get('previous_tag')!r}, want {summary_api_base_ref!r}")
if api_compatibility.get("previous_ref") != summary_api_base_ref:
    fail(f"api_compatibility.previous_ref={api_compatibility.get('previous_ref')!r}, want {summary_api_base_ref!r}")
checked_packages = api_compatibility.get("checked_packages")
if not isinstance(checked_packages, list) or not checked_packages:
    fail("api_compatibility.checked_packages must be a non-empty array")
if api_compatibility.get("checked_package_count") != len(checked_packages):
    fail("api_compatibility.checked_package_count must match checked_packages length")
if not isinstance(api_compatibility.get("incompatible_change_count"), int) or api_compatibility.get("incompatible_change_count") < 0:
    fail("api_compatibility.incompatible_change_count must be a non-negative integer")
if api_compatibility.get("ignored_exception_count") != 0:
    fail("api_compatibility.ignored_exception_count must be 0 until an exception manifest exists")
if api_compatibility.get("generated_report_path") != ".ci-result/release-evidence/logs/release-api-check.log":
    fail("api_compatibility.generated_report_path must point at the release-api-check log")
if api_compatibility.get("log_available") is not True:
    fail("api_compatibility.log_available must be true for publication evidence")

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

identity = summary.get("release_identity") or {}
if identity.get("status") != "passed":
    fail(f"release_identity.status={identity.get('status')!r}, want passed")
release_tag = identity.get("tag")
if not isinstance(release_tag, str) or not re.fullmatch(r"v\d+\.\d+\.\d+|v\d+\.\d+\.0-rc\.\d+", release_tag):
    fail(f"release_identity.tag={release_tag!r}, want a supported release tag")
if expected_release_tag and release_tag != expected_release_tag:
    fail(f"release_identity.tag={release_tag!r}, want {expected_release_tag!r}")
for field in ("commit", "tree", "head_commit", "head_tree", "default_branch", "default_branch_commit"):
    if not isinstance(identity.get(field), str) or not identity[field]:
        fail(f"release_identity.{field} must be a non-empty string")
if identity["commit"] != summary.get("commit") or identity["head_commit"] != summary.get("commit"):
    fail("release_identity commit values must match summary.commit")
if identity["head_commit"] != git_state.get("commit"):
    fail("release_identity.head_commit must match git_state.commit")
if identity["head_tree"] != git_state.get("tree"):
    fail("release_identity.head_tree must match git_state.tree")
for field in ("tag_points_at_head", "tag_tree_matches_head", "tag_reachable_from_default_branch"):
    if identity.get(field) is not True:
        fail(f"release_identity.{field} must be true")

modules = identity.get("modules") or {}
major = release_tag.split(".", 1)[0]
for name, expected_suffix in (("root", f"/{major}"), ("contrib", f"/contrib/{major}")):
    module = modules.get(name) or {}
    if module.get("present") is not True:
        fail(f"release_identity.modules.{name}.present must be true")
    if module.get("version") != release_tag:
        fail(f"release_identity.modules.{name}.version={module.get('version')!r}, want {release_tag!r}")
    module_path = module.get("module_path")
    if not isinstance(module_path, str) or not module_path.endswith(expected_suffix):
        fail(f"release_identity.modules.{name}.module_path={module_path!r}, want suffix {expected_suffix!r}")
cli = modules.get("cli") or {}
if cli.get("present") is True and cli.get("version") != release_tag:
    fail(f"release_identity.modules.cli.version={cli.get('version')!r}, want {release_tag!r}")

workflow = identity.get("workflow") or {}
if workflow.get("trusted") is not True:
    fail("release_identity.workflow.trusted must be true")
if workflow.get("provider") != "github_actions":
    fail(f"release_identity.workflow.provider={workflow.get('provider')!r}, want 'github_actions'")
if workflow.get("repository") != "aatuh/api-toolkit" or workflow.get("expected_repository") != "aatuh/api-toolkit":
    fail("release_identity.workflow repository identity must be aatuh/api-toolkit")
if workflow.get("workflow") != "release":
    fail(f"release_identity.workflow.workflow={workflow.get('workflow')!r}, want 'release'")
workflow_ref = workflow.get("workflow_ref")
if not isinstance(workflow_ref, str) or not re.fullmatch(
    r"aatuh/api-toolkit/\.github/workflows/release\.yml@refs/(heads/master|tags/v\d+\.\d+\.\d+)",
    workflow_ref,
):
    fail(f"release_identity.workflow.workflow_ref={workflow_ref!r}, want the canonical release workflow")
if workflow.get("ref") != f"refs/tags/{release_tag}":
    fail(f"release_identity.workflow.ref={workflow.get('ref')!r}, want refs/tags/{release_tag}")
if workflow.get("sha") != identity["commit"]:
    fail("release_identity.workflow.sha must match release_identity.commit")

if verify_mode == "publication":
    if not os.path.isdir(source_repository):
        fail(f"release source repository is unavailable: {source_repository!r}")

    def git(*args):
        result = subprocess.run(
            ["git", "-C", source_repository, *args],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        if result.returncode != 0:
            fail(f"source git command failed: {' '.join(args)}")
        return result.stdout.strip()

    source_head = git("rev-parse", "--verify", "HEAD")
    source_tag_commit = git("rev-parse", "--verify", f"{release_tag}^{{commit}}")
    source_head_tree = git("rev-parse", f"{source_head}^{{tree}}")
    source_tag_tree = git("rev-parse", f"{source_tag_commit}^{{tree}}")
    source_default_branch = identity["default_branch"]
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._/-]*", source_default_branch) or ".." in source_default_branch or "//" in source_default_branch:
        fail("release_identity.default_branch is not a safe git ref")
    source_branch_commit = git("rev-parse", "--verify", f"{source_default_branch}^{{commit}}")
    reachable = subprocess.run(
        ["git", "-C", source_repository, "merge-base", "--is-ancestor", source_tag_commit, source_branch_commit],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    ).returncode == 0
    if source_head != identity["commit"]:
        fail(f"release_identity.commit={identity['commit']!r}, source HEAD={source_head!r}")
    if source_tag_commit != source_head:
        fail(f"release tag {release_tag!r} does not point at source HEAD")
    if source_head_tree != identity["tree"] or source_tag_tree != source_head_tree:
        fail("release identity tree does not match the source tag and HEAD")
    if source_branch_commit != identity["default_branch_commit"] or not reachable:
        fail(f"release tag {release_tag!r} is not reachable from {source_default_branch!r}")

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

license_evidence = summary.get("dependency_license_evidence") or {}
if license_evidence.get("source") != "SPDX SBOM package metadata generated in the GitHub release workflow":
    fail("dependency_license_evidence.source must describe the SPDX SBOM source")
if license_evidence.get("generator") != "scripts/sbom_license_report.py":
    fail("dependency_license_evidence.generator must name the report generator")
if license_evidence.get("root_report") != "dependency-licenses-root.tsv":
    fail("dependency_license_evidence.root_report must name the root report")
if license_evidence.get("contrib_report") != "dependency-licenses-contrib.tsv":
    fail("dependency_license_evidence.contrib_report must name the contrib report")

sbom_assets = summary.get("sbom_assets") or []
for asset in (
    "sbom-root.spdx.json",
    "sbom-contrib.spdx.json",
    "sbom-root.spdx.json.sig",
    "sbom-root.spdx.json.pem",
    "sbom-contrib.spdx.json.sig",
    "sbom-contrib.spdx.json.pem",
):
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

verify_dependency_license_report() {
  local report="$1"

  awk -F '\t' -v report="$report" '
    NR == 1 {
      if ($0 != "module\tversion\tlicense_expression\tstatus\tsource_purls") {
        printf "%s has an unexpected header\n", report > "/dev/stderr"
        exit 1
      }
      next
    }
    NF != 5 || $1 == "" || $2 == "" || $3 == "" || $4 == "" || $5 == "" {
      printf "%s has an incomplete dependency license row at line %d\n", report, NR > "/dev/stderr"
      exit 1
    }
    $4 != "detected" && $4 != "needs_review" && $4 != "missing_from_sbom" {
      printf "%s has an unknown dependency license status %s at line %d\n", report, $4, NR > "/dev/stderr"
      exit 1
    }
    { rows++ }
    END {
      if (NR == 0) {
        printf "%s is empty\n", report > "/dev/stderr"
        exit 1
      }
    }
  ' "$asset_dir/$report"
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

  local subject
  local subject_path
  for subject in "${required_assets[@]}"; do
    subject_path="$(asset_path "$subject")"
    gh attestation verify "$subject_path" --repo "$github_repo" --source-ref "refs/tags/$release_tag"
  done
}

required_assets=(
  "release-check-summary.json"
  "release-evidence-logs.tgz"
  "release-asset-manifest.tsv"
  "sbom-root.spdx.json"
  "sbom-contrib.spdx.json"
  "dependency-licenses-root.tsv"
  "dependency-licenses-contrib.tsv"
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
verify_dependency_license_report "dependency-licenses-root.tsv"
verify_dependency_license_report "dependency-licenses-contrib.tsv"
verify_sbom_signature "sbom-root.spdx.json" "sbom-root.spdx.json.sig" "sbom-root.spdx.json.pem"
verify_sbom_signature "sbom-contrib.spdx.json" "sbom-contrib.spdx.json.sig" "sbom-contrib.spdx.json.pem"
verify_attestations_if_requested

echo "release artifact verification passed"
