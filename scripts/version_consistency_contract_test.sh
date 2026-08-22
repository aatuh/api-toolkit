#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
gate="$repo_root/scripts/version_consistency_check.sh"
tmp="$(mktemp -d)"
cleanup() {
	rm -rf "$tmp"
}
trap cleanup EXIT

require_success() {
	local name="$1"
	shift
	local output
	set +e
	output="$("$@" 2>&1)"
	local status=$?
	set -e
	if [ "$status" -ne 0 ]; then
		printf 'expected %s to pass, but it failed with %s\noutput:\n%s\n' "$name" "$status" "$output" >&2
		exit 1
	fi
}

require_failure() {
	local name="$1"
	shift
	local output
	set +e
	output="$("$@" 2>&1)"
	local status=$?
	set -e
	if [ "$status" -eq 0 ]; then
		printf 'expected %s to fail, but it passed\noutput:\n%s\n' "$name" "$output" >&2
		exit 1
	fi
	printf '%s' "$output"
}

write_fixture() {
	local fixture="$1"
	mkdir -p "$fixture/docs/migration"
	git init -q -b master "$fixture"
	git -C "$fixture" config user.name 'version consistency contract'
	git -C "$fixture" config user.email 'version-consistency-contract@example.invalid'
	for path in \
		README.md SECURITY.md VERSIONING.md CONTRIBUTING.md ROADMAP.md \
		docs/README.md docs/production-readiness.md docs/support-policy.md \
		docs/release-runbook.md docs/release-review.md; do
		printf '# Fixture\n\nVerified current root baseline: v4.0.1.\n' >"$fixture/$path"
	done
	cat >"$fixture/docs/release-incident-v4-release-identity.md" <<'EOF'
# Release identity

v4.0.0 is Withdrawn.
contrib/v4.0.0 is Withdrawn.
contrib/v4.0.1 is Withdrawn.
v4.0.1 is the verified supported root baseline.
EOF
	printf '# V4 migration\n' >"$fixture/docs/migration/v4.md"
	cat >"$fixture/docs/version-consistency-historical-allowlist.tsv" <<'EOF'
# path	rationale
EOF
	git -C "$fixture" add -- .
	git -C "$fixture" commit -qm 'fixture current version guidance'
}

run_gate() {
	local fixture="$1"
	VERSION_CONSISTENCY_REPOSITORY_ROOT="$fixture" "$gate"
}

valid_fixture="$tmp/valid"
write_fixture "$valid_fixture"
require_success 'valid current guidance' run_gate "$valid_fixture"

stale_prose_fixture="$tmp/stale-prose"
write_fixture "$stale_prose_fixture"
printf '\napi-toolkit v3 is production-credible.\n' >>"$stale_prose_fixture/README.md"
output="$(require_failure 'stale v3 production claim' run_gate "$stale_prose_fixture")"
if [[ "$output" != *'stale current-version guidance in README.md'* ]]; then
	printf 'stale-prose failure did not identify README.md:\n%s\n' "$output" >&2
	exit 1
fi

stale_import_fixture="$tmp/stale-import"
write_fixture "$stale_import_fixture"
printf '\nimport "github.com/aatuh/api-toolkit/v3/httpx"\n' >>"$stale_import_fixture/README.md"
output="$(require_failure 'stale root import' run_gate "$stale_import_fixture")"
if [[ "$output" != *'stale current-version guidance in README.md'* ]]; then
	printf 'stale-import failure did not identify README.md:\n%s\n' "$output" >&2
	exit 1
fi

historical_fixture="$tmp/historical"
write_fixture "$historical_fixture"
printf '\nOld import: github.com/aatuh/api-toolkit/v3/httpx\n' >>"$historical_fixture/docs/migration/v4.md"
printf 'docs/migration/v4.md\tV3-to-v4 migration examples require the old import path.\n' >>"$historical_fixture/docs/version-consistency-historical-allowlist.tsv"
require_success 'allowlisted migration evidence' run_gate "$historical_fixture"

invalid_allowlist_fixture="$tmp/invalid-allowlist"
write_fixture "$invalid_allowlist_fixture"
printf '../README.md\tPath traversal must be rejected.\n' >>"$invalid_allowlist_fixture/docs/version-consistency-historical-allowlist.tsv"
output="$(require_failure 'unsafe allowlist path' run_gate "$invalid_allowlist_fixture")"
if [[ "$output" != *'invalid historical version allowlist entry'* ]]; then
	printf 'unsafe allowlist failure was unclear:\n%s\n' "$output" >&2
	exit 1
fi

printf 'version consistency contract tests passed\n'
