#!/usr/bin/env bash
set -euo pipefail
shopt -s nullglob

go_cmd="${GO:-go}"
fuzztime="${FUZZTIME:-10s}"
fuzz_deadline_retries="${FUZZ_DEADLINE_RETRIES:-1}"
if [[ -z "$fuzztime" || "$fuzztime" == -* || "$fuzztime" == *[[:space:]\;]* ]]; then
  echo "invalid FUZZTIME: $fuzztime" >&2
  exit 2
fi
if ! [[ "$fuzz_deadline_retries" =~ ^[0-9]+$ ]] || (( fuzz_deadline_retries > 3 )); then
  echo "FUZZ_DEADLINE_RETRIES must be an integer from 0 through 3: $fuzz_deadline_retries" >&2
  exit 2
fi

modules=("$@")
if [[ ${#modules[@]} -eq 0 ]]; then
  modules=(.)
fi

run_fuzzer() {
  local pkg="$1"
  local fuzzer="$2"
  local attempt=1
  local max_attempts=$((fuzz_deadline_retries + 1))
  local output_file status

  while :; do
    output_file="$(mktemp)"
    set +e
    "$go_cmd" test "$pkg" -run='^$' -fuzz="^${fuzzer}$" -fuzztime="$fuzztime" -parallel=1 2>&1 | tee "$output_file"
    status="${PIPESTATUS[0]}"
    set -e

    if [ "$status" -eq 0 ]; then
      rm -f "$output_file"
      return 0
    fi

    # Go issue #75804 can report this exact parent-level failure while ending
    # an otherwise successful timed fuzz run. The upstream fix is #79199.
    # Retry only that signature, never a persisted/generated failing input,
    # and only once by default, so genuine fuzz findings remain hard failures.
    if [ "$attempt" -lt "$max_attempts" ] && \
      grep -Eq '^--- FAIL: Fuzz[[:alnum:]_]+ \([0-9.]+s\)$' "$output_file" && \
      grep -Eq '^[[:space:]]+context deadline exceeded$' "$output_file" && \
      ! grep -Fq 'Failing input written to' "$output_file"; then
      echo "Retrying $pkg $fuzzer after known Go timed-fuzz deadline race (attempt $attempt of $max_attempts)." >&2
      rm -f "$output_file"
      attempt=$((attempt + 1))
      continue
    fi

    rm -f "$output_file"
    return "$status"
  done
}

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
        run_fuzzer "$pkg" "$fuzzer"
      done
    done < <("$go_cmd" list ./...)
  )
done
