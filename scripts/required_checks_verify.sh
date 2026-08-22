#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
manifest="$repo_root/docs/required-checks.json"

usage() {
  echo "usage: required_checks_verify.sh [--branch-protection -]" >&2
  exit 2
}

branch_mode=false
case "$#" in
  0)
    ;;
  2)
    if [ "$1" != "--branch-protection" ] || [ "$2" != "-" ]; then
      usage
    fi
    branch_mode=true
    ;;
  *)
    usage
    ;;
esac

if ! command -v jq >/dev/null 2>&1; then
  echo "required-checks: jq is required" >&2
  exit 2
fi
if [ ! -f "$manifest" ]; then
  echo "required-checks: manifest is missing: docs/required-checks.json" >&2
  exit 1
fi

if ! jq -e '
  def bounded_text($maximum):
    . as $value |
    ($value | type) == "string" and
    ($value | length) > 0 and
    ($value | length) <= $maximum and
    ($value | explode | all(. >= 32 and . != 127)) and
    $value == ($value | sub("^\\s+"; "") | sub("\\s+$"; ""));
  type == "array" and
  length > 0 and
  all(.[ ];
    keys == [
      "app_id",
      "check_name",
      "job_id",
      "job_name",
      "owner",
      "required_for_pr",
      "required_for_release",
      "workflow_file"
    ] and
    (.check_name | bounded_text(200)) and
    (.workflow_file | type == "string" and test("^\\.github/workflows/[A-Za-z0-9][A-Za-z0-9._-]*\\.ya?ml$")) and
    (.job_id | type == "string" and test("^[A-Za-z0-9][A-Za-z0-9_-]*$")) and
    (.job_name | bounded_text(200)) and
    (.app_id | type == "number" and floor == . and . > 0) and
    (.required_for_pr | type == "boolean") and
    (.required_for_release | type == "boolean") and
    (.required_for_pr or .required_for_release) and
    (.owner | type == "string" and test("^[a-z0-9][a-z0-9-]*$"))
  ) and
  (map(.check_name) | unique | length) == length and
  any(.[ ]; .required_for_pr) and
  any(.[ ]; .required_for_release)
' "$manifest" >/dev/null; then
  echo "required-checks: manifest schema, path, identity, or ownership validation failed" >&2
  exit 1
fi

workflow_job_name() {
  local workflow_path="$1"
  local job_id="$2"
  awk -v wanted="$job_id" '
    $0 == "  " wanted ":" {
      in_job = 1
      next
    }
    in_job && $0 ~ /^  [A-Za-z0-9][A-Za-z0-9_-]*:[[:space:]]*$/ {
      exit
    }
    in_job && $0 ~ /^    name:[[:space:]]*/ {
      sub(/^    name:[[:space:]]*/, "")
      print
      exit
    }
  ' "$workflow_path"
}

while IFS=$'\t' read -r workflow job_id expected_job_name; do
  workflow_path="$repo_root/$workflow"
  if [ ! -f "$workflow_path" ]; then
    echo "required-checks: workflow is missing: $workflow" >&2
    exit 1
  fi
  actual_job_name="$(workflow_job_name "$workflow_path" "$job_id")"
  if [ -z "$actual_job_name" ]; then
    echo "required-checks: $workflow is missing explicit job $job_id with a stable name" >&2
    exit 1
  fi
  if [ "$actual_job_name" != "$expected_job_name" ]; then
    echo "required-checks: $workflow job $job_id name is '$actual_job_name'; manifest expects '$expected_job_name'" >&2
    exit 1
  fi
done < <(jq -r 'unique_by(.workflow_file + "\u0000" + .job_id + "\u0000" + .job_name)[] | [.workflow_file, .job_id, .job_name] | @tsv' "$manifest")

manifest_count="$(jq 'length' "$manifest")"
echo "required-checks: verified $manifest_count manifest identities and workflow job names"

if [ "$branch_mode" != true ]; then
  exit 0
fi

protection_json="$(cat)"
if [ "${#protection_json}" -gt 1048576 ]; then
  echo "required-checks: branch-protection response exceeds 1 MiB" >&2
  exit 1
fi
if ! printf '%s' "$protection_json" | jq -e '
  def safe_context:
    . as $value |
    ($value | type) == "string" and
    ($value | length) > 0 and
    ($value | length) <= 200 and
    ($value | explode | all(. >= 32 and . != 127));
  type == "object" and
  (.required_status_checks | type == "object") and
  (.required_status_checks.strict | type == "boolean") and
  (.required_status_checks.checks | type == "array") and
  all(.required_status_checks.checks[];
    type == "object" and
    (.context | safe_context) and
    (.app_id | type == "number" and floor == . and . > 0)
  )
' >/dev/null 2>&1; then
  echo "required-checks: malformed or unbound branch-protection response" >&2
  exit 1
fi
if [ "$(printf '%s' "$protection_json" | jq -r '.required_status_checks.strict')" != true ]; then
  echo "required-checks: branch protection must require an up-to-date branch" >&2
  exit 1
fi

comparison="$(printf '%s' "$protection_json" | jq -c --slurpfile manifest "$manifest" '
  ([$manifest[0][] | select(.required_for_pr) | {context: .check_name, app_id}] | sort_by(.context, .app_id)) as $expected |
  ([.required_status_checks.checks[] | {context, app_id}] | sort_by(.context, .app_id)) as $actual |
  {
    matches: ($expected == $actual),
    actual_has_duplicates: (($actual | unique_by(.context, .app_id) | length) != ($actual | length)),
    missing: ($expected - $actual),
    unexpected: ($actual - $expected),
    expected_count: ($expected | length)
  }
')"

if [ "$(printf '%s' "$comparison" | jq -r '.matches and (.actual_has_duplicates | not)')" != true ]; then
  printf '%s' "$comparison" | jq -r '.missing[] | "FAIL missing required check \(.context) (app_id=\(.app_id))"' >&2
  printf '%s' "$comparison" | jq -r '.unexpected[] | "FAIL unmanifested required check \(.context) (app_id=\(.app_id))"' >&2
  if [ "$(printf '%s' "$comparison" | jq -r '.actual_has_duplicates')" = true ]; then
    echo "FAIL branch protection contains duplicate required-check identities" >&2
  fi
  exit 1
fi

expected_count="$(printf '%s' "$comparison" | jq -r '.expected_count')"
echo "required-checks: branch protection exactly matches $expected_count app-bound pull-request checks"
