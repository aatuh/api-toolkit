#!/usr/bin/env bash
set -euo pipefail

# This is a static, read-only workflow policy check. WORKFLOW_SECURITY_ROOT is
# test-only input; it is canonicalized before inspection and never written to.
repo_root="${WORKFLOW_SECURITY_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
repo_root=$(cd "$repo_root" && pwd -P)
workflow_dir="$repo_root/.github/workflows"

if [[ ! -d "$workflow_dir" ]]; then
  echo "workflow-security-check: missing workflow directory" >&2
  exit 2
fi

status=0

report() {
  printf 'workflow-security-check: %s\n' "$*" >&2
  status=1
}

has_pull_request_trigger() {
  grep -Eq '^[[:space:]]*pull_request:([[:space:]]*(#.*)?)$' "$1"
}

require_bounded_artifact_retention() {
  local file=$1
  local line_no=0
  local line
  while IFS= read -r line || [[ -n "$line" ]]; do
    line_no=$((line_no + 1))
    if [[ "$line" != *"actions/upload-artifact@"* ]]; then
      continue
    fi
    local block
    block=$(sed -n "$((line_no + 1)),$((line_no + 14))p" "$file")
    local retention
    retention=$(sed -nE 's/^[[:space:]]*retention-days:[[:space:]]*([0-9]+)[[:space:]]*(#.*)?$/\1/p' <<<"$block" | head -n1)
    if [[ -z "$retention" ]] || ((retention < 1 || retention > 90)); then
      report "${file#"$repo_root/"}:$line_no artifact retention must be explicit and between 1 and 90 days"
    fi
  done <"$file"
}

while IFS= read -r file; do
  rel=${file#"$repo_root/"}
  name=$(basename "$file")

  if grep -Eq '^[[:space:]]*(pull_request_target|workflow_run):' "$file"; then
    report "$rel uses a forbidden privileged workflow trigger"
  fi
  if ! grep -Eq '^permissions:' "$file"; then
    report "$rel has no explicit workflow-level permissions"
  fi
  if grep -Eq '^permissions:[[:space:]]*(read-all|write-all)([[:space:]]*(#.*)?)?$' "$file"; then
    report "$rel uses broad workflow-level permissions"
  fi
  if grep -Fq 'actions/download-artifact@' "$file"; then
    report "$rel downloads workflow artifacts; artifact provenance must be reviewed before enabling this"
  fi
  if grep -Fq 'actions/cache@' "$file"; then
    report "$rel uses a shared actions cache; cache-key and trust review are required before enabling this"
  fi
  if grep -Eq '^[[:space:]]*(-[[:space:]]*)?run:.*\$\{\{[[:space:]]*github\.event\.' "$file"; then
    report "$rel interpolates event data directly in a run command"
  fi

  if has_pull_request_trigger "$file"; then
    if grep -Fq 'secrets.' "$file"; then
      report "$rel exposes secrets to pull-request code"
    fi
    if grep -E '^[[:space:]]+[a-z-]+:[[:space:]]*write([[:space:]]*(#.*)?)?$' "$file" | grep -Ev 'security-events:[[:space:]]*write' >/dev/null; then
      report "$rel grants a write permission to pull-request code"
    fi
  fi

  if grep -Fq 'secrets.' "$file"; then
    case "$name" in
      nightly.yml)
        if has_pull_request_trigger "$file" || ! grep -Fq 'environment: provider-sandbox' "$file"; then
          report "$rel secret-bearing provider checks must be scheduled and environment-gated"
        fi
        ;;
      release.yml)
        if has_pull_request_trigger "$file" || ! grep -Eq '^[[:space:]]*tags:' "$file" || ! grep -Fq 'release-preflight:' "$file"; then
          report "$rel secret-bearing release work must be tag-only and release-preflight gated"
        fi
        ;;
      *) report "$rel references secrets outside the approved scheduled or tag-release workflows" ;;
    esac
  fi

  if [[ "$name" == "release.yml" ]]; then
    if ! grep -Eq '^[[:space:]]*contents:[[:space:]]*write' "$file" || ! grep -Eq '^[[:space:]]*attestations:[[:space:]]*write' "$file" || ! grep -Eq '^[[:space:]]*id-token:[[:space:]]*write' "$file"; then
      report "$rel release-preflight must declare the scoped publication permissions"
    fi
    if grep -Eq '^[[:space:]]*continue-on-error:[[:space:]]*true' "$file"; then
      report "$rel cannot ignore release-preflight failures"
    fi
  fi

  require_bounded_artifact_retention "$file"
done < <(find "$workflow_dir" -type f \( -name '*.yml' -o -name '*.yaml' \) | sort)

if ((status != 0)); then
  exit 1
fi
echo "workflow-security-check: passed"
