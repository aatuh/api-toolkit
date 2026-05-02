#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

export API_BASE_REF=v2.0.1
export ALLOW_DIRTY_RELEASE_EVIDENCE=1
export RELEASE_CHECK_SUMMARY_SOURCE_ONLY=1
source "$repo_root/scripts/release_check_summary.sh"

repo_root="$tmp/repo"
created_at="2026-05-02T00:00:00Z"
mkdir -p "$repo_root/logs" "$repo_root/docs"

cat >"$repo_root/docs/vulnerability-dispositions.tsv" <<'TSV'
vulnerability_id	called_status	owning_dependency	affected_module	affected_package	reviewed_on	expires_on	owner	upgrade_trigger
GO-2026-1001	imported_not_called	example.com/dep	example.com/dep	example.com/dep/pkg	2026-05-01	2099-01-01	security	contract fixture
GO-2026-2001	imported_not_called	example.com/dep	example.com/dep	example.com/dep/pkg	2026-05-01	2099-01-01	security	contract fixture
GO-2026-2002	imported_not_called	example.com/dep	example.com/dep	example.com/dep/pkg	2026-05-01	2099-01-01	security	contract fixture
TSV

cat >"$repo_root/logs/vuln-called.log" <<'LOG'
Vulnerability #1: GO-2026-1001
Your code is affected by 1 vulnerabilities.
govulncheck found 1 vulnerabilities in packages you import and 0 vulnerabilities in modules you require.
LOG

cat >"$repo_root/logs/vuln-imported.log" <<'LOG'
Vulnerability #1: GO-2026-2001
Vulnerability #2: GO-2026-2002
Your code is affected by 0 vulnerabilities.
govulncheck found 2 vulnerabilities in packages you import and 0 vulnerabilities in modules you require.
LOG

cat >"$repo_root/logs/vuln-none.log" <<'LOG'
No vulnerabilities found.
LOG

cat >"$repo_root/logs/vuln-unexpected.log" <<'LOG'
Your code is affected by 0 vulnerabilities.
govulncheck found 2 vulnerabilities in packages you import and 0 vulnerabilities in modules you require.
The upstream output shape changed and omitted advisory headings.
LOG

vulnerability_evidence_json "logs/vuln-called.log" >"$tmp/vuln-called.json"
vulnerability_evidence_json "logs/vuln-imported.log" >"$tmp/vuln-imported.json"
vulnerability_evidence_json "logs/vuln-none.log" >"$tmp/vuln-none.json"
vulnerability_evidence_json "logs/vuln-unexpected.log" >"$tmp/vuln-unexpected.json"

python3 - "$tmp/vuln-called.json" "$tmp/vuln-imported.json" "$tmp/vuln-none.json" "$tmp/vuln-unexpected.json" <<'PY'
import json
import sys

called, imported, none, unexpected = [json.load(open(path, encoding="utf-8")) for path in sys.argv[1:]]
if called["called_vulnerability_count"] != 1 or called["imported_not_called_ids"] != ["GO-2026-1001"]:
    raise SystemExit(f"called vulnerability fixture parsed incorrectly: {called}")
if imported["imported_not_called_vulnerability_count"] != 2 or imported["imported_not_called_ids"] != ["GO-2026-2001", "GO-2026-2002"]:
    raise SystemExit(f"imported-only fixture parsed incorrectly: {imported}")
if imported["missing_disposition_count"] != 0 or imported["expired_disposition_count"] != 0:
    raise SystemExit(f"imported-only fixture should have disposition coverage: {imported}")
if none["imported_not_called_ids"]:
    raise SystemExit(f"no-vulnerability fixture should not have IDs: {none}")
if unexpected["missing_disposition_count"] == 0:
    raise SystemExit(f"unexpected govulncheck shape must fail closed: {unexpected}")
if not any(issue["id"] == "govulncheck-imported-id-parser" for issue in unexpected["disposition_issues"]):
    raise SystemExit(f"unexpected govulncheck shape missing parser issue: {unexpected}")
PY

cat >"$repo_root/docs/contrib-api-drift-dispositions.tsv" <<'TSV'
package	status	reason	release_note_acknowledgement	reviewed_on	expires_on	owner
github.com/aatuh/api-toolkit/contrib/v2/pkg/compatible	compatible	contract fixture	not_required	2026-05-01	2099-01-01	contrib
github.com/aatuh/api-toolkit/contrib/v2/pkg/incompatible	incompatible	contract fixture	docs/release-notes.md package-tied incompatible contrib acknowledgement	2026-05-01	2099-01-01	contrib
github.com/aatuh/api-toolkit/contrib/v2/pkg/malformed	compatible	contract fixture	not_required	2026-05-01	2099-01-01	contrib
TSV

cat >"$repo_root/logs/contrib-compatible.log" <<'LOG'
Contrib API drift report (report-only)
DRIFT github.com/aatuh/api-toolkit/contrib/v2/pkg/compatible
Compatible changes:
- added method
Report complete: drift_packages=1 skipped_packages=0 compatible_drift_packages=1 incompatible_drift_packages=0
LOG

cat >"$repo_root/logs/contrib-incompatible.log" <<'LOG'
Contrib API drift report (report-only)
DRIFT github.com/aatuh/api-toolkit/contrib/v2/pkg/incompatible
Incompatible changes:
- removed method
Report complete: drift_packages=1 skipped_packages=0 compatible_drift_packages=0 incompatible_drift_packages=1
LOG

cat >"$repo_root/logs/contrib-none.log" <<'LOG'
Contrib API drift report (report-only)
OK   github.com/aatuh/api-toolkit/contrib/v2/pkg/compatible
Report complete: drift_packages=0 skipped_packages=0 compatible_drift_packages=0 incompatible_drift_packages=0
LOG

cat >"$repo_root/logs/contrib-skipped.log" <<'LOG'
Contrib API drift report (report-only)
SKIP github.com/aatuh/api-toolkit/contrib/v2/pkg/skipped (missing in baseline or working tree)
Report complete: drift_packages=0 skipped_packages=1 compatible_drift_packages=0 incompatible_drift_packages=0
LOG

cat >"$repo_root/logs/contrib-malformed.log" <<'LOG'
Contrib API drift report (report-only)
DRIFT github.com/aatuh/api-toolkit/contrib/v2/pkg/malformed
Unexpected heading:
- parser should not call this compatible
Report complete: drift_packages=1 skipped_packages=0 compatible_drift_packages=0 incompatible_drift_packages=0
LOG

contrib_drift_json "passed" "0" "1" "logs/contrib-compatible.log" "true" >"$tmp/contrib-compatible.json"
contrib_drift_json "passed" "0" "1" "logs/contrib-incompatible.log" "true" >"$tmp/contrib-incompatible.json"
contrib_drift_json "passed" "0" "1" "logs/contrib-none.log" "true" >"$tmp/contrib-none.json"
contrib_drift_json "passed" "0" "1" "logs/contrib-skipped.log" "true" >"$tmp/contrib-skipped.json"
contrib_drift_json "passed" "0" "1" "logs/contrib-malformed.log" "true" >"$tmp/contrib-malformed.json"

python3 - "$tmp/contrib-compatible.json" "$tmp/contrib-incompatible.json" "$tmp/contrib-none.json" "$tmp/contrib-skipped.json" "$tmp/contrib-malformed.json" <<'PY'
import json
import sys

compatible, incompatible, none, skipped, malformed = [json.load(open(path, encoding="utf-8")) for path in sys.argv[1:]]
if compatible["packages"] != [{"package": "github.com/aatuh/api-toolkit/contrib/v2/pkg/compatible", "status": "compatible"}]:
    raise SystemExit(f"compatible drift fixture parsed incorrectly: {compatible}")
if incompatible["packages"] != [{"package": "github.com/aatuh/api-toolkit/contrib/v2/pkg/incompatible", "status": "incompatible"}]:
    raise SystemExit(f"incompatible drift fixture parsed incorrectly: {incompatible}")
if none["packages"] or none["drift_package_count"] != 0:
    raise SystemExit(f"no-drift fixture parsed incorrectly: {none}")
if skipped["packages"] or skipped["skipped_package_count"] != 1:
    raise SystemExit(f"skipped fixture parsed incorrectly: {skipped}")
if malformed["packages"] != [{"package": "github.com/aatuh/api-toolkit/contrib/v2/pkg/malformed", "status": "unknown"}]:
    raise SystemExit(f"malformed drift fixture should be unknown: {malformed}")
if malformed["missing_disposition_count"] == 0:
    raise SystemExit(f"unknown drift status must fail disposition review: {malformed}")
PY

echo "release evidence parser contract tests passed"
