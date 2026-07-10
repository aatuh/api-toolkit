#!/usr/bin/env bash
set -euo pipefail

summary_path="${1:-release-check-summary.json}"
if [ ! -s "$summary_path" ]; then
  echo "release summary not found: $summary_path" >&2
  exit 1
fi

SUMMARY_PATH="$summary_path" python3 - <<'PY'
import json
import os
import sys


def value(data, path, default=""):
    current = data
    for part in path:
        if not isinstance(current, dict) or part not in current:
            return default
        current = current[part]
    return current


with open(os.environ["SUMMARY_PATH"], "r", encoding="utf-8") as handle:
    summary = json.load(handle)

git_state = value(summary, ("git_state",), {})
provenance = value(summary, ("provenance_policy",), {})
vulnerability = value(summary, ("vulnerability_evidence",), {})
contrib = value(summary, ("contrib_drift",), {})
licenses = value(summary, ("dependency_license_evidence",), {})
full_profile = value(summary, ("full_profile_scaffold_evidence",), {})
expectations = value(summary, ("publication_artifact_expectations",), {})

checks = summary.get("checks") or []
failed_checks = [check.get("name", "<unnamed>") for check in checks if check.get("status") != "passed"]

decision = "accept-local-evidence"
if summary.get("status") != "passed" or summary.get("publication_eligible") is not True:
    decision = "reject"
elif provenance.get("status") != "passed" or git_state.get("dirty") is not False:
    decision = "reject"
elif failed_checks:
    decision = "reject"
elif vulnerability.get("called_vulnerability_count") != 0:
    decision = "reject"
elif vulnerability.get("missing_disposition_count") != 0 or vulnerability.get("expired_disposition_count") != 0:
    decision = "reject"
elif contrib.get("missing_disposition_count") != 0 or contrib.get("expired_disposition_count") != 0:
    decision = "reject"

print(f"summary: {os.environ['SUMMARY_PATH']}")
print(f"status: {summary.get('status')}")
print(f"publication_eligible: {summary.get('publication_eligible')}")
print(f"api_base_ref: {summary.get('api_base_ref')}")
print(
    "git_state: "
    f"dirty={git_state.get('dirty')} "
    f"staged={git_state.get('staged_count')} "
    f"unstaged={git_state.get('unstaged_count')} "
    f"untracked={git_state.get('untracked_count')} "
    f"deleted={git_state.get('deleted_count')}"
)
print(f"provenance_policy: status={provenance.get('status')} mode={provenance.get('mode')}")
print(f"checks: total={len(checks)} failed={','.join(failed_checks) if failed_checks else 'none'}")
print(
    "vulnerability_dispositions: "
    f"called={vulnerability.get('called_vulnerability_count')} "
    f"imported_not_called={vulnerability.get('imported_not_called_vulnerability_count')} "
    f"missing={vulnerability.get('missing_disposition_count')} "
    f"expired={vulnerability.get('expired_disposition_count')}"
)
print(
    "contrib_drift: "
    f"status={contrib.get('status')} "
    f"drift={contrib.get('drift_package_count')} "
    f"compatible={contrib.get('compatible_drift_count')} "
    f"incompatible={contrib.get('incompatible_drift_count')} "
    f"missing={contrib.get('missing_disposition_count')} "
    f"expired={contrib.get('expired_disposition_count')} "
    f"artifact={contrib.get('artifact_path')}"
)
print(
    "dependency_license_evidence: "
    f"root_report={licenses.get('root_report')} "
    f"contrib_report={licenses.get('contrib_report')} "
    f"generator={licenses.get('generator')}"
)
print(
	    "full_profile_scaffold: "
	    f"profile={full_profile.get('profile')} "
	    f"validation={value(full_profile, ('scaffold_validation', 'status'))} "
	    f"openapi31={value(full_profile, ('openapi_31_full_scaffold', 'status'))} "
	    f"typed_client={value(full_profile, ('typed_client_generation', 'status'))} "
	    f"typescript_client={value(full_profile, ('typescript_client_generation', 'status'))} "
	    f"resource_generator={value(full_profile, ('resource_generator_check', 'status'))} "
	    f"provider_flags={value(full_profile, ('provider_flag_generation', 'status'))} "
	    f"observability={value(full_profile, ('observability_bundle', 'status'))} "
	    f"helm={value(full_profile, ('deployment_packaging', 'helm', 'status'))} "
	    f"terraform_aws={value(full_profile, ('deployment_packaging', 'terraform_aws', 'status'))} "
	    f"migration_lifecycle={value(full_profile, ('migration_lifecycle', 'status'))} "
	    f"session_profile={value(full_profile, ('session_profile', 'status'))} "
	    f"worker={value(full_profile, ('worker_check', 'status'))} "
	    f"integration_workflow={value(full_profile, ('integration_workflow', 'status'))} "
	    f"integration={value(full_profile, ('integration_check', 'status'))}"
	)
print(
    "artifact_expectations: "
    f"draft_assets={len(expectations.get('github_draft_release_assets') or [])} "
    f"attestation_subjects={len(expectations.get('github_attestation_subjects') or [])} "
    f"local_generates_signed_sboms={expectations.get('local_generates_signed_sboms')}"
)
print(f"retained_log_archive: {value(summary, ('publication_artifact_checksums', 'local_evidence_assets'), [{}])[0].get('source_path', '') if value(summary, ('publication_artifact_checksums', 'local_evidence_assets'), []) else ''}")
print(f"sbom_status: {summary.get('sbom_status')}")
print(f"review_decision: {decision}")

if decision == "reject":
    sys.exit(1)
PY
