#!/usr/bin/env bash
set -euo pipefail

repo_root="${REFERENCE_SERVICE_LOAD_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
result_dir="${REFERENCE_SERVICE_LOAD_RESULT_DIR:-.ci-result/reference-service-load}"
requests="${REFERENCE_SERVICE_LOAD_REQUESTS:-240}"
concurrency="${REFERENCE_SERVICE_LOAD_CONCURRENCY:-8}"
go_cmd="${GO:-go}"
commit="$(git -C "$repo_root" rev-parse --verify HEAD 2>/dev/null || printf 'unknown')"
profile="${REFERENCE_SERVICE_LOAD_PROFILE:-local-in-process}"

case "$result_dir" in
  /*) result_path="$result_dir" ;;
  *) result_path="$repo_root/$result_dir" ;;
esac

mkdir -p "$result_path"
log_path="$result_path/load-smoke.log"

if (
  cd "$repo_root/examples/reference-saas-api"
  GOTOOLCHAIN="${GOTOOLCHAIN:-local}" GOWORK="${GOWORK:-off}" "$go_cmd" run ./cmd/loadsmoke \
    -requests "$requests" \
    -concurrency "$concurrency" \
    -commit "$commit" \
    -profile "$profile" \
    -out "$result_path"
) >"$log_path" 2>&1; then
  for required in status summary.json summary.md; do
    if [ ! -f "$result_path/$required" ]; then
      printf 'reference-service load smoke did not write %s\n' "$result_path/$required" >&2
      cat "$log_path" >&2
      exit 1
    fi
  done
  cat "$log_path"
  exit 0
fi

status=$?
cat "$log_path" >&2
exit "$status"
