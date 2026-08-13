#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
evidence_file="$repo_root/.ci-result/provider-live/evidence.json"

output="$(cd "$repo_root" && env -u RUN_PROVIDER_LIVE_CHECKS -u STRIPE_SECRET_KEY -u RESEND_API_KEY -u CLERK_JWKS_URL GOWORK=off GOTOOLCHAIN=local make provider-live-check)"
if ! grep -Fq '"checked_at":null,"status":"not_requested"' <<<"$output" ||
  ! grep -Fq '"provider":"stripe","status":"not_requested"' <<<"$output" ||
  ! grep -Fq '"provider":"resend","status":"not_requested"' <<<"$output" ||
  ! grep -Fq '"provider":"clerk","status":"not_requested"' <<<"$output"; then
	printf '%s\n' 'provider live check did not report not_requested safely' >&2
	exit 1
fi

output="$(cd "$repo_root" && env -u STRIPE_SECRET_KEY -u RESEND_API_KEY -u CLERK_JWKS_URL RUN_PROVIDER_LIVE_CHECKS=true GOWORK=off GOTOOLCHAIN=local make provider-live-check)"
if ! grep -Eq '^\{"checked_at":"[0-9TZ:-]+","status":"skipped_no_credentials"' <<<"$output" ||
  ! grep -Fq '"provider":"stripe","status":"skipped_no_credentials"' <<<"$output" ||
  ! grep -Fq '"provider":"resend","status":"skipped_no_credentials"' <<<"$output" ||
  ! grep -Fq '"provider":"clerk","status":"skipped_no_credentials"' <<<"$output"; then
	printf '%s\n' 'provider live check did not report absent credentials as non-success' >&2
	exit 1
fi
if [ ! -f "$evidence_file" ] || [ "$(stat -c '%a' "$evidence_file")" != "600" ]; then
	printf '%s\n' 'provider live evidence file is missing or not owner-readable only' >&2
	exit 1
fi
