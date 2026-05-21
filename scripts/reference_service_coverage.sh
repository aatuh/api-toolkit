#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
go_cmd="${GO:-go}"
out_dir="${OUTPUT_DIR:-.ci-result}/coverage"
service_dir="$repo_root/examples/reference-saas-api"
profile="$out_dir/reference-service.coverprofile"
log="$out_dir/reference-service.log"
funcs="$out_dir/reference-service.func"
summary="$out_dir/reference-service-summary.md"

mkdir -p "$repo_root/$out_dir"

echo "==> reference-service coverage"
(cd "$service_dir" && "$go_cmd" test ./... -covermode=atomic -coverprofile="$repo_root/$profile") | tee "$repo_root/$log"
(cd "$service_dir" && "$go_cmd" tool cover -func="$repo_root/$profile") | tee "$repo_root/$funcs"

total="$(awk '/^total:/ { gsub("%", "", $3); print $3 }' "$repo_root/$funcs")"
cat >"$repo_root/$summary" <<EOF_SUMMARY
## Reference service coverage

Reference-service coverage is non-Docker app-owned evidence. It is reported
separately from root and contrib aggregate thresholds.

| Module | Total |
| --- | ---: |
| examples/reference-saas-api | ${total}% |

Detailed function summary: \`${funcs}\`
EOF_SUMMARY

printf 'reference-service coverage total %s%%; summary=%s\n' "$total" "$summary"
