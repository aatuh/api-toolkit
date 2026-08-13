#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
manifest="$repo_root/docs/required-checks.json"
command -v jq >/dev/null || { echo 'required-checks: jq is required' >&2; exit 2; }
jq -e 'type == "array" and length > 0 and all(.[]; (.check_name|type=="string" and length>0) and (.workflow_file|type=="string" and startswith(".github/workflows/")) and (.job_id|type=="string" and length>0) and (.required_for_pr|type=="boolean") and (.required_for_release|type=="boolean") and (.owner|type=="string" and length>0))' "$manifest" >/dev/null
while IFS=$'\t' read -r workflow job; do
  grep -Fq "  $job:" "$repo_root/$workflow" || { echo "required-checks: $workflow missing job $job" >&2; exit 1; }
done < <(jq -r '.[] | [.workflow_file, .job_id] | @tsv' "$manifest")
echo 'required-checks: manifest verified'
