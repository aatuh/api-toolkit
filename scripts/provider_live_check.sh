#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
evidence_dir="$repo_root/.ci-result/provider-live"
evidence_file="$evidence_dir/evidence.json"

mkdir -p "$evidence_dir"
chmod 700 "$evidence_dir"
umask 077

if [ "${RUN_PROVIDER_LIVE_CHECKS:-}" != "true" ]; then
  printf '%s\n' '{"checked_at":null,"status":"not_requested","providers":[{"provider":"stripe","status":"not_requested"},{"provider":"resend","status":"not_requested"},{"provider":"clerk","status":"not_requested"}]}' >"$evidence_file"
  cat "$evidence_file"
  exit 0
fi

tmp_file="$(mktemp "$evidence_dir/evidence.XXXXXX")"
trap 'rm -f "$tmp_file"' EXIT
set +e
(cd "$repo_root/contrib" && GOWORK="$repo_root/go.work" GOTOOLCHAIN="${GOTOOLCHAIN:-local}" go run ./cmd/provider-live-check) >"$tmp_file"
status=$?
set -e

if ! grep -Eq '^\{"checked_at":"[0-9TZ:-]+","status":"(passed|failed|skipped_no_credentials)","providers":\[' "$tmp_file"; then
  printf '%s\n' 'provider live check did not emit sanitized evidence' >&2
  exit 1
fi
mv "$tmp_file" "$evidence_file"
trap - EXIT
cat "$evidence_file"
exit "$status"
