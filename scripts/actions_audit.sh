#!/usr/bin/env bash
set -euo pipefail

repo_root="${ACTIONS_AUDIT_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
status=0

current_checkout="actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # actions/checkout v6.0.2"
current_setup_go="actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # actions/setup-go v6.4.0"

report() {
  printf 'actions-audit: %s\n' "$*" >&2
  status=1
}

relpath() {
  local path="$1"
  if [[ "$path" == "$repo_root/"* ]]; then
    printf '%s' "${path#"$repo_root/"}"
  else
    printf '%s' "$path"
  fi
}

audit_uses_line() {
  local file="$1"
  local line_no="$2"
  local line="$3"
  local ref
  local rel
  rel="$(relpath "$file")"

  if [[ "$line" != *uses:* ]]; then
    return
  fi
  if [[ "$line" =~ uses:[[:space:]]*([^[:space:]#]+) ]]; then
    ref="${BASH_REMATCH[1]}"
  else
    return
  fi
  case "$ref" in
    ./*|docker://*) return ;;
  esac
  if [[ "$ref" != *@* ]]; then
    report "$rel:$line_no unpinned action ref $ref"
    return
  fi
  local version="${ref##*@}"
  if [[ ! "$version" =~ ^[0-9a-f]{40}$ ]]; then
    report "$rel:$line_no unpinned action ref $ref"
  fi
}

audit_file() {
  local file="$1"
  local rel
  local line_no=0
  rel="$(relpath "$file")"
  while IFS= read -r line || [ -n "$line" ]; do
    line_no=$((line_no + 1))
    if [[ "$line" == *"actions/attest-build-provenance v1"* ]]; then
      report "$rel:$line_no deprecated action comment actions/attest-build-provenance v1"
    fi
    if [[ "$line" == *"actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5"* || "$line" == *"actions/checkout v4"* ]]; then
      report "$rel:$line_no stale generated checkout action; want $current_checkout"
    fi
    if [[ "$line" == *"actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff"* || "$line" == *"actions/setup-go v5"* ]]; then
      report "$rel:$line_no stale generated setup-go action; want $current_setup_go"
    fi
    audit_uses_line "$file" "$line_no" "$line"
  done <"$file"
}

files=()
if [ -d "$repo_root/.github/workflows" ]; then
  while IFS= read -r file; do
    files+=("$file")
  done < <(find "$repo_root/.github/workflows" -type f \( -name '*.yml' -o -name '*.yaml' \) | sort)
fi
if [ -f "$repo_root/contrib/cmd/api-toolkit/main.go" ]; then
  files+=("$repo_root/contrib/cmd/api-toolkit/main.go")
fi

for file in "${files[@]}"; do
  audit_file "$file"
done

generator="$repo_root/contrib/cmd/api-toolkit/main.go"
if [ -f "$generator" ]; then
  if ! grep -Fq "$current_checkout" "$generator"; then
    report "contrib/cmd/api-toolkit/main.go missing current generated checkout action $current_checkout"
  fi
  if ! grep -Fq "$current_setup_go" "$generator"; then
    report "contrib/cmd/api-toolkit/main.go missing current generated setup-go action $current_setup_go"
  fi
fi

if [ "$status" -ne 0 ]; then
  exit "$status"
fi
printf 'actions-audit: passed\n'
