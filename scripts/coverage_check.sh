#!/usr/bin/env bash
set -euo pipefail

check=0
if [[ "${1:-}" == "--check" ]]; then
  check=1
fi

go_cmd="${GO:-go}"
out_dir="${OUTPUT_DIR:-.ci-result}/coverage"
repo_root="$(pwd)"
mkdir -p "$out_dir"

root_min="${ROOT_COVERAGE_MIN:-70.0}"
contrib_min="${CONTRIB_COVERAGE_MIN:-52.0}"

run_module() {
  local name="$1"
  local dir="$2"
  local profile="$out_dir/$name.coverprofile"
  local log="$out_dir/$name.log"
  local funcs="$out_dir/$name.func"

  echo "==> $name coverage"
  (cd "$dir" && "$go_cmd" test ./... -covermode=atomic -coverprofile="$repo_root/$profile") | tee "$log"
  "$go_cmd" tool cover -func="$profile" | tee "$funcs"
}

coverage_total() {
  awk '/^total:/ { gsub("%", "", $3); print $3 }' "$1"
}

package_coverage() {
  local log="$1"
  local pkg="$2"
  awk -v pkg="$pkg" '
    $1 == "ok" && $2 == pkg {
      for (i = 1; i <= NF; i++) {
        if ($i == "coverage:") {
          value = $(i + 1)
          gsub("%", "", value)
          print value
        }
      }
    }
  ' "$log" | tail -1
}

check_min() {
  local label="$1"
  local got="$2"
  local want="$3"
  if [[ -z "$got" ]]; then
    echo "coverage-check: missing coverage for $label" >&2
    return 1
  fi
  awk -v got="$got" -v want="$want" -v label="$label" '
    BEGIN {
      if (got + 0 < want + 0) {
        printf "coverage-check: %s coverage %.1f%% is below %.1f%%\n", label, got, want > "/dev/stderr"
        exit 1
      }
      printf "coverage-check: %s coverage %.1f%% >= %.1f%%\n", label, got, want
    }
  '
}

run_module root .
run_module contrib contrib

root_total="$(coverage_total "$out_dir/root.func")"
contrib_total="$(coverage_total "$out_dir/contrib.func")"

echo "coverage-summary: root total ${root_total}%"
echo "coverage-summary: contrib total ${contrib_total}%"

if [[ "$check" -eq 1 ]]; then
  check_min "root aggregate" "$root_total" "$root_min"
  check_min "contrib aggregate" "$contrib_total" "$contrib_min"

  check_min "github.com/aatuh/api-toolkit/v3/apiclient" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/apiclient")" "${APICLIENT_COVERAGE_MIN:-80.0}"
  check_min "github.com/aatuh/api-toolkit/v3/apitest" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/apitest")" "${APITEST_COVERAGE_MIN:-75.0}"
  check_min "github.com/aatuh/api-toolkit/v3/oauth2" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/oauth2")" "${OAUTH2_COVERAGE_MIN:-85.0}"
  check_min "github.com/aatuh/api-toolkit/v3/upload" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/upload")" "${UPLOAD_COVERAGE_MIN:-85.0}"
fi
