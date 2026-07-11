#!/usr/bin/env bash
# Rejects unclassified TODO/FIXME markers in stable root packages.
set -euo pipefail

repo_root="${TODO_GATE_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
classification="${TODO_GATE_CLASSIFICATION:-$repo_root/docs/package-classification.tsv}"
exceptions="${TODO_GATE_EXCEPTIONS:-$repo_root/docs/todo-exceptions.tsv}"
root_module="${TODO_GATE_ROOT_MODULE:-github.com/aatuh/api-toolkit/v4}"

if [ ! -f "$classification" ]; then
  echo "package classification manifest not found: $classification" >&2
  exit 2
fi
if [ ! -f "$exceptions" ]; then
  echo "TODO exception manifest not found: $exceptions" >&2
  exit 2
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
exception_keys="$tmp_dir/exception-keys.tsv"
violations="$tmp_dir/violations.txt"

awk -F '\t' '
  NR == 1 {
    for (i = 1; i <= NF; i++) {
      h[$i] = i
    }
    next
  }
  /^#/ || NF == 0 {
    next
  }
  {
    path = $h["path"]
    marker = $h["marker"]
    issue = $h["issue"]
    classification = $h["classification"]
    if (path == "" || marker == "" || issue == "" || classification == "") {
      printf("invalid TODO exception row %d: missing required field\n", NR) > "/dev/stderr"
      exit 2
    }
    if (classification != "non-blocking") {
      printf("invalid TODO exception row %d: classification must be non-blocking\n", NR) > "/dev/stderr"
      exit 2
    }
    print path "\t" marker "\t" issue
  }
' "$exceptions" >"$exception_keys"

append_violation() {
  printf '%s\n' "$1" >>"$violations"
}

scan_file() {
  local file="$1"
  local rel_file="$2"
  local match line_no text marker issue key

  while IFS= read -r match; do
    line_no="${match%%:*}"
    text="${match#*:}"
    if [[ "$text" =~ (TODO|FIXME) ]]; then
      marker="${BASH_REMATCH[1]}"
    else
      continue
    fi
    if [[ "$text" =~ (https://github.com/[^[:space:]\)]+/issues/[0-9]+|#[0-9]+) ]]; then
      issue="${BASH_REMATCH[1]}"
    else
      append_violation "$rel_file:$line_no $marker missing linked issue"
      continue
    fi
    key="$rel_file"$'\t'"$marker"$'\t'"$issue"
    if ! grep -Fqx "$key" "$exception_keys"; then
      append_violation "$rel_file:$line_no $marker $issue missing non-blocking exception row"
    fi
  done < <(grep -nE 'TODO|FIXME' "$file" || true)
}

while IFS=$'\t' read -r import_path api_status _rest; do
  if [[ -z "${import_path// }" || "$import_path" == \#* ]]; then
    continue
  fi
  if [[ "$api_status" != "stable" && "$api_status" != "compatibility-only" ]]; then
    continue
  fi
  if [[ "$import_path" != "$root_module" && "$import_path" != "$root_module/"* ]]; then
    continue
  fi

  rel="${import_path#"$root_module"}"
  rel="${rel#/}"
  pkg_dir="$repo_root"
  if [ -n "$rel" ]; then
    pkg_dir="$repo_root/$rel"
  fi
  if [ ! -d "$pkg_dir" ]; then
    append_violation "$import_path classified as $api_status but directory is missing"
    continue
  fi

  while IFS= read -r -d '' file; do
    rel_file="${file#"$repo_root"/}"
    scan_file "$file" "$rel_file"
  done < <(find "$pkg_dir" -maxdepth 1 -type f -name '*.go' -print0)
done <"$classification"

if [ -s "$violations" ]; then
  echo "stable package TODO/FIXME gate failed:" >&2
  cat "$violations" >&2
  exit 1
fi

echo "stable package TODO/FIXME gate passed"
