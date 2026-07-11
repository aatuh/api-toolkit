#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

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
  printf '%s' "$output"
}

make_fake_tools() {
  local bin_dir="$1"
  mkdir -p "$bin_dir"
  cat >"$bin_dir/cosign" <<'FAKE'
#!/usr/bin/env sh
exit 0
FAKE
  cat >"$bin_dir/gh" <<'FAKE'
#!/usr/bin/env sh
printf '%s\n' "$*" >> "${GH_CALLS:?}"
exit 0
FAKE
  chmod +x "$bin_dir/cosign" "$bin_dir/gh"
}

write_manifest() {
  local dir="$1"

  (
    cd "$dir"
    sha256sum \
      release-check-summary.json \
      release-evidence-logs.tgz \
      sbom-root.spdx.json \
      sbom-contrib.spdx.json \
      dependency-licenses-root.tsv \
      dependency-licenses-contrib.tsv \
      sbom-root.spdx.json.sig \
      sbom-root.spdx.json.pem \
      sbom-contrib.spdx.json.sig \
      sbom-contrib.spdx.json.pem > release-asset-manifest.tsv
  )
}

make_bundle() {
  local dir="$1"
  local status="${2:-passed}"
  local include_extra_log="${3:-no}"
  local summary_extra_log="${4:-}"
  local logs_dir="$dir/logs"
  mkdir -p "$dir" "$logs_dir"

  python3 - "$dir/release-check-summary.json" "$status" "$summary_extra_log" <<'PY'
import json
import sys

path, status, extra_log = sys.argv[1:]
checks = [
    "tools",
    "lint",
    "vuln",
    "gosec",
    "ci-build-smoke",
    "release-api-check",
    "contrib-release-notes-check",
    "v3-readiness-check",
    "docs-check",
    "test",
    "test-race",
    "fuzz",
    "clean",
]
check_records = [
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
]
if extra_log:
    check_records.append(
        {
            "name": "custom-extra",
            "command_line": "custom-extra",
            "status": "passed",
            "exit_code": 0,
            "duration_ms": 1,
            "log_available": True,
            "log_path": f".ci-result/release-evidence/logs/{extra_log}",
            "artifacts": [],
        }
    )
summary = {
    "schema": "github.com/aatuh/api-toolkit/release-check-summary/v2",
    "created_at": "2026-05-02T00:00:00Z",
    "commit": "abc123",
    "git_state": {
        "commit": "abc123",
        "branch": "main",
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
        "message": "Publication release evidence requires a clean git worktree.",
    },
    "api_base_ref": "v2.1.0",
    "api_compatibility": {
        "previous_tag": "v2.1.0",
        "previous_ref": "v2.1.0",
        "checked_package_count": 2,
        "checked_packages": [
            "github.com/aatuh/api-toolkit/v4/httpx",
            "github.com/aatuh/api-toolkit/v4/middleware/maxbody",
        ],
        "incompatible_change_count": 0,
        "ignored_exception_count": 0,
        "generated_report_path": ".ci-result/release-evidence/logs/release-api-check.log",
        "log_available": True,
    },
    "quality_command": "API_BASE_REF=v2.1.0 GOTOOLCHAIN=local make release-check",
    "evidence_command": "API_BASE_REF=v2.1.0 GOTOOLCHAIN=local make release-evidence",
    "status": status,
    "publication_eligible": status == "passed",
    "checks": check_records,
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
        "review_disposition": "fixture",
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
    "dependency_license_evidence": {
        "source": "SPDX SBOM package metadata generated in the GitHub release workflow",
        "generator": "scripts/sbom_license_report.py",
        "format": "TSV",
        "root_report": "dependency-licenses-root.tsv",
        "contrib_report": "dependency-licenses-contrib.tsv",
        "scope": "contract fixture",
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
            "dependency-licenses-root.tsv",
            "dependency-licenses-contrib.tsv",
            "sbom-root.spdx.json.sig",
            "sbom-root.spdx.json.pem",
            "sbom-contrib.spdx.json.sig",
            "sbom-contrib.spdx.json.pem",
        ],
        "github_attestation_subjects": [
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
        ],
        "local_generates_signed_sboms": False,
    },
    "publication_artifact_checksums": {
        "algorithm": "sha256",
        "github_draft_release_manifest": "release-asset-manifest.tsv",
        "local_evidence_assets": [
            {
                "path": "release-evidence-logs.tgz",
                "source_path": ".ci-result/release-evidence/release-evidence-logs.tgz",
                "status": "present",
                "sha256": "fixture",
            }
        ],
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
    contrib-release-notes-check.log v3-readiness-check.log docs-check.log test.log test-race.log fuzz.log \
    clean.log contrib-api-drift-report.log; do
    printf 'fixture %s\n' "$log_name" >"$logs_dir/$log_name"
  done
  if [ "$include_extra_log" = "yes" ] && [ -n "$summary_extra_log" ]; then
    printf 'fixture extra\n' >"$logs_dir/$summary_extra_log"
  fi
  tar -C "$logs_dir" -czf "$dir/release-evidence-logs.tgz" .

  printf 'module\tversion\tlicense_expression\tstatus\tsource_purls\n' >"$dir/dependency-licenses-root.tsv"
  printf 'module\tversion\tlicense_expression\tstatus\tsource_purls\nexample.com/dependency-licenses-contrib.tsv\tv1.0.0\tMIT\tdetected\tpkg:golang/example.com/dependency-licenses-contrib.tsv@v1.0.0\n' >"$dir/dependency-licenses-contrib.tsv"

  for asset in \
    sbom-root.spdx.json sbom-contrib.spdx.json \
    sbom-root.spdx.json.sig sbom-root.spdx.json.pem \
    sbom-contrib.spdx.json.sig sbom-contrib.spdx.json.pem; do
    printf 'fixture %s\n' "$asset" >"$dir/$asset"
  done
  write_manifest "$dir"
}

fake_bin="$tmp/bin"
make_fake_tools "$fake_bin"

ok_dir="$tmp/ok"
make_bundle "$ok_dir"
require_success local-verify env PATH="$fake_bin:$PATH" API_BASE_REF=v2.1.0 bash "$repo_root/scripts/release_artifact_verify.sh" "$ok_dir" >/dev/null
require_success local-verify-summary-baseline env -u API_BASE_REF PATH="$fake_bin:$PATH" bash "$repo_root/scripts/release_artifact_verify.sh" "$ok_dir" >/dev/null
require_success local-wrapper-verify env PATH="$fake_bin:$PATH" API_BASE_REF=v2.1.0 RELEASE_ARTIFACT_VERIFY_MODE=local bash "$repo_root/scripts/verify-release.sh" "$ok_dir" >/dev/null

baseline_mismatch_output="$(require_failure baseline-mismatch env PATH="$fake_bin:$PATH" API_BASE_REF=v3.0.1 bash "$repo_root/scripts/release_artifact_verify.sh" "$ok_dir")"
case "$baseline_mismatch_output" in
  *"api_base_ref='v2.1.0', want 'v3.0.1'"*) ;;
  *) printf 'explicit baseline mismatch did not fail clearly:\n%s\n' "$baseline_mismatch_output" >&2; exit 1 ;;
esac

missing_tag_output="$(require_failure publication-missing-tag env PATH="$fake_bin:$PATH" API_BASE_REF=v2.1.0 RELEASE_ARTIFACT_VERIFY_MODE=publication bash "$repo_root/scripts/release_artifact_verify.sh" "$ok_dir")"
case "$missing_tag_output" in
  *"RELEASE_TAG is required when RELEASE_ARTIFACT_VERIFY_MODE=publication"*) ;;
  *) printf 'publication mode did not require RELEASE_TAG:\n%s\n' "$missing_tag_output" >&2; exit 1 ;;
esac

bad_summary_dir="$tmp/bad-summary"
make_bundle "$bad_summary_dir" "failed"
bad_summary_output="$(require_failure failed-summary env PATH="$fake_bin:$PATH" API_BASE_REF=v2.1.0 bash "$repo_root/scripts/release_artifact_verify.sh" "$bad_summary_dir")"
case "$bad_summary_output" in
  *"status='failed', want passed"*) ;;
  *) printf 'failed summary invariant did not fail clearly:\n%s\n' "$bad_summary_output" >&2; exit 1 ;;
esac

malformed_license_dir="$tmp/malformed-license"
make_bundle "$malformed_license_dir"
printf 'unexpected\theader\n' >"$malformed_license_dir/dependency-licenses-root.tsv"
write_manifest "$malformed_license_dir"
malformed_license_output="$(require_failure malformed-license-report env PATH="$fake_bin:$PATH" API_BASE_REF=v2.1.0 bash "$repo_root/scripts/release_artifact_verify.sh" "$malformed_license_dir")"
case "$malformed_license_output" in
  *"dependency-licenses-root.tsv has an unexpected header"*) ;;
  *) printf 'malformed dependency license report did not fail clearly:\n%s\n' "$malformed_license_output" >&2; exit 1 ;;
esac

missing_summary_log_dir="$tmp/missing-summary-log"
make_bundle "$missing_summary_log_dir" "passed" "no" "custom-extra.log"
missing_summary_log_output="$(require_failure missing-summary-log env PATH="$fake_bin:$PATH" API_BASE_REF=v2.1.0 bash "$repo_root/scripts/release_artifact_verify.sh" "$missing_summary_log_dir")"
case "$missing_summary_log_output" in
  *"release-evidence-logs.tgz missing checks[].log_path .ci-result/release-evidence/logs/custom-extra.log"*) ;;
  *) printf 'summary-driven retained log verification did not fail clearly:\n%s\n' "$missing_summary_log_output" >&2; exit 1 ;;
esac

publication_dir="$tmp/publication"
make_bundle "$publication_dir"
GH_CALLS="$tmp/gh-calls" require_success publication-verify env \
  PATH="$fake_bin:$PATH" \
  GH_CALLS="$tmp/gh-calls" \
  API_BASE_REF=v2.1.0 \
  RELEASE_ARTIFACT_VERIFY_MODE=publication \
  RELEASE_TAG=v2.1.0 \
  GITHUB_REPOSITORY=aatuh/api-toolkit \
  bash "$repo_root/scripts/verify-release.sh" "$publication_dir" >/dev/null
if [ "$(wc -l < "$tmp/gh-calls")" -ne 11 ]; then
  printf 'publication verifier should run one gh attestation check per release asset, got:\n%s\n' "$(cat "$tmp/gh-calls")" >&2
  exit 1
fi

GH_CALLS="$tmp/gh-calls-rc" require_success publication-rc-verify env \
  PATH="$fake_bin:$PATH" \
  GH_CALLS="$tmp/gh-calls-rc" \
  API_BASE_REF=v2.1.0 \
  RELEASE_TAG=v2.1.0-rc.1 \
  GITHUB_REPOSITORY=aatuh/api-toolkit \
  bash "$repo_root/scripts/verify-release.sh" "$publication_dir" >/dev/null

missing_wrapper_tag_output="$(require_failure publication-wrapper-missing-tag env PATH="$fake_bin:$PATH" bash "$repo_root/scripts/verify-release.sh" "$publication_dir")"
case "$missing_wrapper_tag_output" in
  *"RELEASE_TAG is required to verify a published draft release"*) ;;
  *) printf 'publication wrapper did not require a release tag:\n%s\n' "$missing_wrapper_tag_output" >&2; exit 1 ;;
esac

malformed_wrapper_tag_output="$(require_failure publication-wrapper-malformed-tag env PATH="$fake_bin:$PATH" RELEASE_TAG=v2.1.1-rc.1 bash "$repo_root/scripts/verify-release.sh" "$publication_dir")"
case "$malformed_wrapper_tag_output" in
  *"RELEASE_TAG must be vX.Y.Z or vX.Y.0-rc.N, got: v2.1.1-rc.1"*) ;;
  *) printf 'publication wrapper did not reject malformed release tag:\n%s\n' "$malformed_wrapper_tag_output" >&2; exit 1 ;;
esac

malformed_wrapper_repo_output="$(require_failure publication-wrapper-malformed-repository env PATH="$fake_bin:$PATH" RELEASE_TAG=v2.1.0 GITHUB_REPOSITORY=aatuh/api-toolkit/extra bash "$repo_root/scripts/verify-release.sh" "$publication_dir")"
case "$malformed_wrapper_repo_output" in
  *"GITHUB_REPOSITORY must be an owner/repository value, got: aatuh/api-toolkit/extra"*) ;;
  *) printf 'publication wrapper did not reject malformed repository:\n%s\n' "$malformed_wrapper_repo_output" >&2; exit 1 ;;
esac

echo "release artifact verifier contract tests passed"
