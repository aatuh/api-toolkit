#!/usr/bin/env bash
# Build a synthetic release asset bundle and exercise the local verifier path.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
asset_dir="${RELEASE_ARTIFACT_FIXTURE_DIR:-$tmp/assets}"
fake_bin="$tmp/bin"

cleanup() {
  if [ -z "${RELEASE_ARTIFACT_FIXTURE_DIR:-}" ] && [ -z "${KEEP_RELEASE_ARTIFACT_FIXTURE:-}" ]; then
    rm -rf "$tmp"
  else
    printf 'synthetic release asset fixture retained at %s\n' "$asset_dir"
  fi
}
trap cleanup EXIT

mkdir -p "$asset_dir/logs" "$fake_bin"
cat >"$fake_bin/cosign" <<'FAKE'
#!/usr/bin/env sh
exit 0
FAKE
chmod +x "$fake_bin/cosign"

python3 - "$asset_dir/release-check-summary.json" <<'PY'
import json
import sys

path = sys.argv[1]
checks = [
    "tools",
    "lint",
    "vuln",
    "gosec",
    "ci-build-smoke",
    "release-api-check",
    "contrib-release-notes-check",
    "docs-check",
    "test",
    "test-race",
    "fuzz",
    "clean",
]
summary = {
    "schema": "github.com/aatuh/api-toolkit/release-check-summary/v2",
    "created_at": "2026-05-02T00:00:00Z",
    "commit": "fixture",
    "git_state": {
        "commit": "fixture",
        "branch": "fixture",
        "detached": False,
        "dirty": False,
        "staged_count": 0,
        "unstaged_count": 0,
        "untracked_count": 0,
        "deleted_count": 0,
    },
    "provenance_policy": {
        "mode": "publication",
        "allow_dirty_release_evidence": False,
        "status": "passed",
        "message": "synthetic fixture for local verifier behavior",
    },
    "api_base_ref": "v2.1.0",
    "quality_command": "API_BASE_REF=v2.1.0 GOTOOLCHAIN=local make release-check",
    "evidence_command": "API_BASE_REF=v2.1.0 GOTOOLCHAIN=local make release-evidence",
    "status": "passed",
    "publication_eligible": True,
    "checks": [
        {
            "name": name,
            "command_line": f"API_BASE_REF=v2.1.0 GOTOOLCHAIN=local make {name}",
            "status": "passed",
            "exit_code": 0,
            "duration_ms": 1,
            "log_available": True,
            "log_path": f".ci-result/release-evidence/logs/{name}.log",
            "artifacts": [],
        }
        for name in checks
    ],
    "tool_versions": [],
    "vulnerability_evidence": {
        "source_log_path": ".ci-result/release-evidence/logs/vuln.log",
        "status": "available",
        "review_date": "2026-05-02",
        "called_vulnerability_count": 0,
        "imported_not_called_vulnerability_count": 0,
        "required_not_called_module_vulnerability_count": 0,
        "imported_not_called_ids": [],
        "disposition_manifest_path": "docs/vulnerability-dispositions.tsv",
        "missing_disposition_count": 0,
        "expired_disposition_count": 0,
        "disposition_issues": [],
        "review_disposition": "synthetic fixture",
    },
    "contrib_drift": {
        "command_line": "API_BASE_REF=v2.1.0 GOTOOLCHAIN=local make contrib-api-drift-report",
        "status": "passed",
        "exit_code": 0,
        "duration_ms": 1,
        "log_available": True,
        "artifact_path": ".ci-result/release-evidence/logs/contrib-api-drift-report.log",
        "disposition_manifest_path": "docs/contrib-api-drift-dispositions.tsv",
        "review_date": "2026-05-02",
        "drift_package_count": 0,
        "skipped_package_count": 0,
        "compatible_drift_count": 0,
        "incompatible_drift_count": 0,
        "packages": [],
        "missing_disposition_count": 0,
        "expired_disposition_count": 0,
        "disposition_issues": [],
    },
    "artifact_tiers": {},
    "publication_artifact_expectations": {
        "local_evidence_assets": [
            "release-check-summary.json",
            ".ci-result/release-evidence/logs/*.log",
            ".ci-result/release-evidence/release-evidence-logs.tgz",
        ],
        "github_draft_release_assets": [
            "release-check-summary.json",
            "release-evidence-logs.tgz",
            "release-asset-manifest.tsv",
            "sbom-root.spdx.json",
            "sbom-contrib.spdx.json",
            "sbom-root.spdx.json.sig",
            "sbom-root.spdx.json.pem",
            "sbom-contrib.spdx.json.sig",
            "sbom-contrib.spdx.json.pem",
        ],
        "github_attestation_subjects": [
            "release-check-summary.json",
            "sbom-root.spdx.json",
            "sbom-contrib.spdx.json",
        ],
        "local_generates_signed_sboms": False,
    },
    "publication_artifact_checksums": {
        "algorithm": "sha256",
        "github_draft_release_manifest": "release-asset-manifest.tsv",
        "local_evidence_assets": [],
    },
    "sbom_status": "not_generated",
    "sbom_assets": [
        "sbom-root.spdx.json",
        "sbom-contrib.spdx.json",
        "sbom-root.spdx.json.sig",
        "sbom-root.spdx.json.pem",
        "sbom-contrib.spdx.json.sig",
        "sbom-contrib.spdx.json.pem",
    ],
}
with open(path, "w", encoding="utf-8") as handle:
    json.dump(summary, handle)
PY

for log_name in \
  tools.log lint.log vuln.log gosec.log ci-build-smoke.log release-api-check.log \
  contrib-release-notes-check.log docs-check.log test.log test-race.log fuzz.log \
  clean.log contrib-api-drift-report.log; do
  printf 'synthetic fixture %s\n' "$log_name" >"$asset_dir/logs/$log_name"
done
tar -C "$asset_dir/logs" -czf "$asset_dir/release-evidence-logs.tgz" .

for asset in \
  sbom-root.spdx.json sbom-contrib.spdx.json \
  sbom-root.spdx.json.sig sbom-root.spdx.json.pem \
  sbom-contrib.spdx.json.sig sbom-contrib.spdx.json.pem; do
  printf 'synthetic fixture %s\n' "$asset" >"$asset_dir/$asset"
done

(
  cd "$asset_dir"
  sha256sum \
    release-check-summary.json \
    release-evidence-logs.tgz \
    sbom-root.spdx.json \
    sbom-contrib.spdx.json \
    sbom-root.spdx.json.sig \
    sbom-root.spdx.json.pem \
    sbom-contrib.spdx.json.sig \
    sbom-contrib.spdx.json.pem > release-asset-manifest.tsv
)

PATH="$fake_bin:$PATH" \
  API_BASE_REF="${API_BASE_REF:-v2.1.0}" \
  RELEASE_ARTIFACT_VERIFY_MODE=local \
  bash "$repo_root/scripts/release_artifact_verify.sh" "$asset_dir"

cat <<'MSG'
Synthetic local release artifact verifier fixture passed.
This is not publication verification. Publication verification still requires
downloaded GitHub draft release assets, RELEASE_ARTIFACT_VERIFY_MODE=publication,
RELEASE_TAG, GITHUB_REPOSITORY, real Sigstore certificates, and online
attestation checks.
MSG
