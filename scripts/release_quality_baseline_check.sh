#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "release quality baseline check: $*" >&2
  exit 1
}

manifest_header() {
  awk 'NF && $0 !~ /^#/ { print; exit }' "$1"
}

repo_root="${RELEASE_QUALITY_REPOSITORY_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || true)}"
[ -n "$repo_root" ] || fail "run inside a Git repository or set RELEASE_QUALITY_REPOSITORY_ROOT"
repo_root="$(git -C "$repo_root" rev-parse --show-toplevel 2>/dev/null || true)"
[ -n "$repo_root" ] || fail "RELEASE_QUALITY_REPOSITORY_ROOT is not a Git repository"

# This is a release identifier, never an arbitrary revision. Validate it before
# handing it to git so CI environment values cannot alter revision parsing.
release="${RELEASE_QUALITY_RELEASE:-${RELEASE_TAG:-v4.0.1}}"
if [[ ! "$release" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] && \
  [[ ! "$release" =~ ^v[0-9]+\.[0-9]+\.0-rc\.[0-9]+$ ]]; then
  fail "release must be vX.Y.Z or vX.Y.0-rc.N"
fi
release_commit="$(git -C "$repo_root" rev-parse --verify "${release}^{commit}" 2>/dev/null || true)"
[ -n "$release_commit" ] || fail "release tag $release does not resolve to a commit"

coverage_path="$repo_root/docs/coverage-trend.tsv"
benchmark_path="$repo_root/docs/benchmark-baselines.tsv"
[ -f "$coverage_path" ] || fail "missing $coverage_path"
[ -f "$benchmark_path" ] || fail "missing $benchmark_path"

coverage_header='release_tag	release_commit	module	package	api_status	test_status	observed_percent'
benchmark_header='release_tag	release_commit	go_version	goos	goarch	cpu	benchmark_flags	module	package	benchmark	observed_ns_per_op	observed_bytes_per_op	max_bytes_per_op	observed_allocs_per_op	max_allocs_per_op	evidence'
[ "$(manifest_header "$coverage_path")" = "$coverage_header" ] || fail "coverage baseline header is invalid"
[ "$(manifest_header "$benchmark_path")" = "$benchmark_header" ] || fail "benchmark baseline header is invalid"

# The immutable v4.0.1 root release is the one historical exception: its paired
# contrib tag is withdrawn because its v4.0.0 root dependency checksum is not
# reproducible. Do not generalize this exception to later releases.
root_only_historical=false
if [ "$release" = "v4.0.1" ] && [ "$release_commit" = "09e0117828c960453e3fb4cd028a02bc3e56ff33" ]; then
  root_only_historical=true
fi

awk -F '\t' \
  -v release="$release" \
  -v commit="$release_commit" \
  -v root_only="$root_only_historical" '
  $1 == release {
    rows++
    if (NF != 7) { bad = "coverage row has an unexpected field count"; next }
    if ($2 != commit) { bad = "coverage row commit differs from the release tag"; next }
    if ($3 == "root" && $4 == "(aggregate)") {
      root_aggregate++
      root_value = $7
    } else if ($3 == "contrib" && $4 == "(aggregate)") {
      contrib_aggregate++
      contrib_value = $7
      contrib_status = $6
    } else if ($3 == "root") {
      root_packages++
    } else if ($3 == "contrib") {
      contrib_packages++
      if (root_only == "true" && ($6 != "release-integrity-blocked" || $7 != "not-reported")) {
        bad = "historical contrib coverage must remain release-integrity-blocked and not-reported"
      }
    } else {
      bad = "coverage row has an unknown module"
    }
  }
  END {
    if (bad != "") { print bad > "/dev/stderr"; exit 1 }
    if (rows == 0) { print "coverage source has no row for the release tag" > "/dev/stderr"; exit 1 }
    if (root_aggregate != 1 || root_value !~ /^[0-9]+(\.[0-9]+)?$/ || root_packages == 0) {
      print "coverage source needs one numeric root aggregate and package rows" > "/dev/stderr"; exit 1
    }
    if (contrib_aggregate != 1 || contrib_packages == 0) {
      print "coverage source needs one contrib aggregate and package rows" > "/dev/stderr"; exit 1
    }
    if (root_only == "true") {
      if (contrib_status != "release-integrity-blocked" || contrib_value != "not-reported") {
        print "v4.0.1 must retain its explicit contrib integrity limitation" > "/dev/stderr"; exit 1
      }
    } else if (contrib_value !~ /^[0-9]+(\.[0-9]+)?$/) {
      print "non-historical releases require numeric contrib coverage" > "/dev/stderr"; exit 1
    }
  }
' "$coverage_path" || fail "coverage source does not bind $release to $release_commit"

awk -F '\t' \
  -v release="$release" \
  -v commit="$release_commit" \
  -v root_only="$root_only_historical" '
  $1 == release {
    rows++
    if (NF != 16) { bad = "benchmark row has an unexpected field count"; next }
    if ($2 != commit) { bad = "benchmark row commit differs from the release tag"; next }
    for (i = 3; i <= 16; i++) if ($i == "") { bad = "benchmark row has an empty metadata field" }
    if ($8 != "root" && $8 != "contrib") { bad = "benchmark row has an unknown module" }
    if ($11 !~ /^[0-9]+$/ || $12 !~ /^[0-9]+$/ || $13 !~ /^[0-9]+$/ || $14 !~ /^[0-9]+$/ || $15 !~ /^[0-9]+$/) {
      bad = "benchmark row has non-numeric measurement data"
    }
    if (($13 + 0) < ($12 + 0) || ($15 + 0) < ($14 + 0)) { bad = "benchmark threshold is below its observed value" }
    if ($8 == "root") root_rows++
    if ($8 == "contrib") contrib_rows++
  }
  END {
    if (bad != "") { print bad > "/dev/stderr"; exit 1 }
    if (rows == 0 || root_rows == 0) { print "benchmark source has no root row for the release tag" > "/dev/stderr"; exit 1 }
    if (root_only == "true") {
      if (contrib_rows != 0) { print "v4.0.1 must not contain invented contrib benchmark values" > "/dev/stderr"; exit 1 }
    } else if (contrib_rows == 0) {
      print "non-historical releases require contrib benchmark rows" > "/dev/stderr"; exit 1
    }
  }
' "$benchmark_path" || fail "benchmark source does not bind $release to $release_commit"

if [ "$root_only_historical" = true ]; then
  grep -Fq "root-only historical baseline" "$repo_root/docs/coverage-trend.md" || \
    fail "coverage documentation must describe the v4.0.1 root-only baseline"
  grep -Fq "checksum" "$repo_root/docs/performance.md" || \
    fail "benchmark documentation must describe the contrib checksum limitation"
fi

printf 'release quality baselines verified: %s (%s)\n' "$release" "$release_commit"
