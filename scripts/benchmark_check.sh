#!/usr/bin/env bash
set -euo pipefail

# benchmark_check.sh runs each reviewed hot-path benchmark repeatedly and
# compares its allocation output with the checked-in review budgets. It is a
# guardrail for controlled runners, not a portable latency claim.

repo_root="${BENCHMARK_CHECK_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
baseline_path="$repo_root/docs/benchmark-baselines.tsv"
result_dir="${BENCHMARK_RESULT_DIR:-.ci-result/benchmark-check}"
runs="${BENCHMARK_RUNS:-3}"
benchtime="${BENCHMARK_BENCHTIME:-1s}"
go_cmd="${GO:-go}"

case "$result_dir" in
  /*|*'..'*)
    printf 'BENCHMARK_RESULT_DIR must be a repository-relative path without parent traversal\n' >&2
    exit 2
    ;;
esac
case "$runs" in
  ''|*[!0-9]*|0)
    printf 'BENCHMARK_RUNS must be a positive integer\n' >&2
    exit 2
    ;;
esac
case "$benchtime" in
  ''|*[!0-9a-zA-Z.])
    printf 'BENCHMARK_BENCHTIME must be a Go benchmark duration\n' >&2
    exit 2
    ;;
esac
if [ ! -f "$baseline_path" ]; then
  printf 'benchmark baseline manifest is missing: %s\n' "$baseline_path" >&2
  exit 2
fi

repo_root="$(cd "$repo_root" && pwd -P)"
workspace="$repo_root/go.work"
if [ ! -f "$workspace" ]; then
  printf 'workspace file is missing: %s\n' "$workspace" >&2
  exit 2
fi
result_path="$repo_root/$result_dir"
mkdir -p "$result_path/raw"
result_path="$(cd "$result_path" && pwd -P)"
case "$result_path" in
  "$repo_root"/*) ;;
  *)
    printf 'BENCHMARK_RESULT_DIR resolves outside the repository\n' >&2
    exit 2
    ;;
esac

metadata_path="$result_path/metadata.tsv"
samples_path="$result_path/samples.tsv"
summary_path="$result_path/summary.tsv"
printf 'commit\tgo_version\tgoos\tgoarch\tmachine\tbenchmark_runs\tbenchmark_benchtime\n' >"$metadata_path"
printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
  "$(git -C "$repo_root" rev-parse HEAD)" \
  "$(GOTOOLCHAIN="${GOTOOLCHAIN:-local}" "$go_cmd" version)" \
  "$(GOTOOLCHAIN="${GOTOOLCHAIN:-local}" "$go_cmd" env GOOS)" \
  "$(GOTOOLCHAIN="${GOTOOLCHAIN:-local}" "$go_cmd" env GOARCH)" \
  "$(uname -m)" "$runs" "$benchtime" >>"$metadata_path"
printf 'module\tpackage\tbenchmark\trun\tbytes_per_op\tallocs_per_op\tmax_bytes_per_op\tmax_allocs_per_op\tbaseline_bytes_per_op\tbaseline_allocs_per_op\n' >"$samples_path"
printf 'module\tpackage\tbenchmark\tmax_bytes_per_op\tmax_allocs_per_op\tresult\n' >"$summary_path"

sample_index=0
failed=0
while IFS=$'\t' read -r module package benchmark observed_bytes max_bytes observed_allocs max_allocs evidence; do
  [ -z "$module" ] && continue
  case "$module" in \#*) continue ;; esac
  case "$module:$package:$benchmark:$observed_bytes:$max_bytes:$observed_allocs:$max_allocs" in
    *[!a-zA-Z0-9_./:-]*)
      printf 'invalid benchmark baseline row for %s\n' "$benchmark" >&2
      exit 2
      ;;
  esac
  case "$observed_bytes:$max_bytes:$observed_allocs:$max_allocs" in
    *[!0-9:]*)
      printf 'benchmark baseline numeric fields must be non-negative integers for %s\n' "$benchmark" >&2
      exit 2
      ;;
  esac
  case "$module" in
    root) module_dir="$repo_root" ;;
    contrib) module_dir="$repo_root/contrib" ;;
    *) printf 'unknown benchmark module %q\n' "$module" >&2; exit 2 ;;
  esac

  row_result="pass"
  for run in $(seq 1 "$runs"); do
    sample_index=$((sample_index + 1))
    raw_path="$result_path/raw/${sample_index}.txt"
    if ! (
      cd "$module_dir"
      GOTOOLCHAIN="${GOTOOLCHAIN:-local}" GOWORK="$workspace" \
        "$go_cmd" test "./$package" -run '^$' -bench "^${benchmark}$" -benchmem -benchtime="$benchtime" -count=1
    ) >"$raw_path" 2>&1; then
      cat "$raw_path" >&2
      exit 1
    fi
    metrics="$(awk -v benchmark="$benchmark" '
      $1 ~ ("^" benchmark "(-[0-9]+)?$") {
        bytes = ""; allocs = ""
        for (i = 1; i <= NF; i++) {
          if ($i == "B/op") bytes = $(i - 1)
          if ($i == "allocs/op") allocs = $(i - 1)
        }
        if (bytes != "" && allocs != "") { print bytes "\t" allocs; exit }
      }
    ' "$raw_path")"
    if [ -z "$metrics" ]; then
      printf 'benchmark output did not include allocation metrics for %s\n' "$benchmark" >&2
      cat "$raw_path" >&2
      exit 1
    fi
    bytes="${metrics%%$'\t'*}"
    allocs="${metrics#*$'\t'}"
    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
      "$module" "$package" "$benchmark" "$run" "$bytes" "$allocs" "$max_bytes" "$max_allocs" "$observed_bytes" "$observed_allocs" >>"$samples_path"
    if [ "$bytes" -gt "$max_bytes" ] || [ "$allocs" -gt "$max_allocs" ]; then
      row_result="over_budget"
      failed=1
    fi
  done
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$module" "$package" "$benchmark" "$max_bytes" "$max_allocs" "$row_result" >>"$summary_path"
done <"$baseline_path"

if [ "$failed" -ne 0 ]; then
  printf 'benchmark allocation budget exceeded; inspect %s and add a performance note before changing a reviewed threshold\n' "$result_path" >&2
  exit 1
fi
printf 'benchmark allocation checks passed; raw samples and machine metadata are in %s\n' "$result_path"
