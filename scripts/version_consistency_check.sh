#!/usr/bin/env bash
set -euo pipefail

# Keep current release guidance coherent without rewriting immutable migration or
# release history. The allowlist is exact-path only so a new legacy reference
# cannot be hidden by a glob, a directory exemption, or a path traversal entry.
repo_root="${VERSION_CONSISTENCY_REPOSITORY_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
if [ ! -d "$repo_root" ] || ! git -C "$repo_root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
	echo "VERSION_CONSISTENCY_REPOSITORY_ROOT must name a Git worktree" >&2
	exit 2
fi
repo_root="$(git -C "$repo_root" rev-parse --show-toplevel)"

allowlist="$repo_root/docs/version-consistency-historical-allowlist.tsv"
if [ ! -f "$allowlist" ]; then
	echo "missing historical version allowlist: docs/version-consistency-historical-allowlist.tsv" >&2
	exit 1
fi

declare -A historical_files=()
while IFS=$'\t' read -r path rationale extra || [ -n "${path:-}" ]; do
	if [ -z "${path:-}" ] || [[ "$path" == \#* ]]; then
		continue
	fi
	if [ -z "${rationale:-}" ] || [ -n "${extra:-}" ] ||
		[[ ! "$path" =~ ^[A-Za-z0-9][A-Za-z0-9._/-]*\.md$ ]] ||
		[[ "$path" == *..* ]] || [[ "$path" == *//* ]] || [[ "$path" == */ ]] ||
		! git -C "$repo_root" ls-files --error-unmatch -- "$path" >/dev/null 2>&1; then
		echo "invalid historical version allowlist entry: ${path:-<empty>}" >&2
		exit 1
	fi
	case "$path" in
		CHANGELOG.md|docs/migration/*|docs/archive/*|docs/release-notes.md|docs/coverage-trend.md|docs/api-inventory.md) ;;
		*)
			echo "historical version allowlist path is not an approved historical record: $path" >&2
			exit 1
			;;
	esac
	if [ -n "${historical_files[$path]:-}" ]; then
		echo "duplicate historical version allowlist entry: $path" >&2
		exit 1
	fi
	historical_files["$path"]=1
done <"$allowlist"

current_sources=(
	README.md
	SECURITY.md
	VERSIONING.md
	CONTRIBUTING.md
	ROADMAP.md
	docs/README.md
	docs/production-readiness.md
	docs/support-policy.md
	docs/release-runbook.md
	docs/release-review.md
)
for path in "${current_sources[@]}"; do
	if [ ! -f "$repo_root/$path" ]; then
		echo "missing current-version source: $path" >&2
		exit 1
	fi
	if ! grep -Fq 'v4.0.1' "$repo_root/$path"; then
		echo "current-version source does not name the verified v4.0.1 baseline: $path" >&2
		exit 1
	fi
done

identity_record="$repo_root/docs/release-incident-v4-release-identity.md"
if [ ! -f "$identity_record" ]; then
	echo "missing v4 release-identity record" >&2
	exit 1
fi
for withdrawn in 'v4.0.0' 'contrib/v4.0.0' 'contrib/v4.0.1'; do
	escaped_withdrawn="$(printf '%s' "$withdrawn" | sed 's/[][\\.^$*+?(){}|]/\\&/g')"
	if ! grep -Eiq "${escaped_withdrawn}.*[Ww]ithdrawn|[Ww]ithdrawn.*${escaped_withdrawn}" "$identity_record"; then
		echo "v4 release-identity record does not mark $withdrawn withdrawn" >&2
		exit 1
	fi
done
if ! grep -Eiq 'v4\.0\.1.*(verified|supported)|(verified|supported).*v4\.0\.1' "$identity_record"; then
	echo "v4 release-identity record does not mark v4.0.1 verified and supported" >&2
	exit 1
fi

legacy_import='github\.com/aatuh/api-toolkit(/contrib)?/v3([/`[:space:])]|$)'
legacy_release_command='(API_BASE_REF|TAG)=v3\.[0-9]+\.[0-9]+'
stale_current_claim='api-toolkit[[:space:]]+v3[[:space:]]+is[[:space:]]+(the[[:space:]]+)?(current|latest|supported|production-credible)'

while IFS= read -r -d '' path; do
	if [ -n "${historical_files[$path]:-}" ]; then
		continue
	fi
	file="$repo_root/$path"
	for rule in "$legacy_import" "$legacy_release_command" "$stale_current_claim"; do
		matches="$(grep -EinE "$rule" "$file" || true)"
		if [ -n "$matches" ]; then
			printf 'stale current-version guidance in %s:\n%s\n' "$path" "$matches" >&2
			exit 1
		fi
	done
done < <(git -C "$repo_root" ls-files -z -- '*.md')

printf 'current-version consistency passed: baseline=v4.0.1 allowlist=%d\n' "${#historical_files[@]}"
