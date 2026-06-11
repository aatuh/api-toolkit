#!/usr/bin/env bash
set -euo pipefail

go_cmd="${GO:-go}"
packages="${MUTATION_PACKAGES:-./binding,./queryparams,./negotiation,./webhooks}"
limit="${MUTATION_LIMIT:-12}"
timeout="${MUTATION_TIMEOUT:-30s}"
out="${MUTATION_OUT:-${OUTPUT_DIR:-.ci-result}/mutation/mutation-smoke.tsv}"

if [[ -z "$go_cmd" || "$go_cmd" == -* ]]; then
  echo "GO must name a command" >&2
  exit 2
fi
if [[ ! "$limit" =~ ^[0-9]+$ ]]; then
  echo "MUTATION_LIMIT must be a non-negative integer" >&2
  exit 2
fi
if [[ -z "$timeout" || "$timeout" == -* || "$timeout" =~ [[:space:]\;\&\|\`\$\<\>] ]]; then
  echo "MUTATION_TIMEOUT must be a single duration value such as 30s" >&2
  exit 2
fi
if [[ -z "$packages" || "$packages" == -* || "$packages" =~ [\;\&\|\`\$\<\>] ]]; then
  echo "MUTATION_PACKAGES must contain comma or whitespace separated ./ package patterns" >&2
  exit 2
fi
if [[ -z "$out" || "$out" == -* ]]; then
  echo "MUTATION_OUT must be an output path under the repository root" >&2
  exit 2
fi

GOWORK="${GOWORK:-off}" GOTOOLCHAIN="${GOTOOLCHAIN:-local}" "$go_cmd" run ./internal/tools/mutationsmoke \
  -packages "$packages" \
  -limit "$limit" \
  -timeout "$timeout" \
  -out "$out"
