#!/usr/bin/env bash
set -euo pipefail
shopt -s nullglob

go_cmd="${GO:-go}"
fuzztime="${FUZZTIME:-10s}"
if [[ -z "$fuzztime" || "$fuzztime" == -* || "$fuzztime" == *[[:space:]\;]* ]]; then
  echo "invalid FUZZTIME: $fuzztime" >&2
  exit 2
fi

modules=("$@")
if [[ ${#modules[@]} -eq 0 ]]; then
  modules=(.)
fi

for mod in "${modules[@]}"; do
  echo "==> $mod"
  (
    cd "$mod"
    while IFS= read -r pkg; do
      dir="$("$go_cmd" list -f '{{.Dir}}' "$pkg")"
      test_files=("$dir"/*_test.go)
      if [[ ${#test_files[@]} -eq 0 ]]; then
        continue
      fi
      mapfile -t fuzzers < <(
        { grep -hoE '^func Fuzz[[:alnum:]_]*[[:space:]]*\(' "${test_files[@]}" 2>/dev/null || true; } |
          sed -E 's/^func (Fuzz[[:alnum:]_]*)[[:space:]]*\(.*/\1/' |
          sort -u
      )
      if [[ ${#fuzzers[@]} -eq 0 ]]; then
        continue
      fi
      for fuzzer in "${fuzzers[@]}"; do
        echo "==> $pkg $fuzzer"
        "$go_cmd" test "$pkg" -run='^$' -fuzz="^${fuzzer}$" -fuzztime="$fuzztime" -parallel=1
      done
    done < <("$go_cmd" list ./...)
  )
done
