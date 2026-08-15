#!/usr/bin/env bash
set -euo pipefail

# PR_BODY is untrusted pull-request metadata. Treat it only as bounded text:
# this script never evaluates it, runs it as a command, or prints it.
body=${PR_BODY-}

if [[ -z "$body" ]]; then
  echo "pull-request body is required" >&2
  exit 2
fi
if ((${#body} > 65536)); then
  echo "pull-request body must be at most 65536 bytes" >&2
  exit 2
fi

require_heading() {
  local heading=$1
  if ! grep -Fqx -- "$heading" <<<"$body"; then
    echo "pull-request body is missing required section $heading" >&2
    exit 2
  fi
}

require_single_selection() {
  local field=$1
  shift
  local pattern
  pattern=$(IFS='|'; printf '%s' "$*")
  local selections
  selections=$(grep -E -- "^- \[[xX]\] (${pattern})$" <<<"$body" || true)
  local count=0
  if [[ -n "$selections" ]]; then
    count=$(grep -c . <<<"$selections")
  fi
  if ((count != 1)); then
    echo "pull-request body must select exactly one ${field} classification" >&2
    exit 2
  fi
}

for heading in \
  "# Backlog ticket" \
  "# Tests and verification" \
  "# Documentation" \
  "# Compatibility impact" \
  "# Security impact" \
  "# Dependency impact" \
  "# Generated-file impact" \
  "# Benchmark impact" \
  "# Migration impact"; do
  require_heading "$heading"
done

if ! grep -Eq -- '^- \[[xX]\] Ticket ID: `[A-Z]+-[0-9]+`$' <<<"$body"; then
  echo "pull-request body must include one checked backlog ticket ID" >&2
  exit 2
fi

require_single_selection "compatibility" \
  "No public effect" "Additive API" "Behavioral change" "Deprecation" "Breaking change"
require_single_selection "security" \
  "No new trust boundary or sensitive data handling\." "New or changed trust boundary is tested and documented\."
require_single_selection "dependency" \
  "No dependency, license, or supply-chain impact\." "Dependency, license, or supply-chain impact is documented and reviewed\."
require_single_selection "generated-file" \
  "No generated files changed\." "Generated files were regenerated and included in this PR\."
require_single_selection "benchmark" \
  "No performance-sensitive path changed\." "Benchmarks or rationale are included for performance-sensitive changes\."
require_single_selection "migration" \
  "No migration required\." "Migration steps and compatibility window are documented\."

echo "pull-request body is valid"
