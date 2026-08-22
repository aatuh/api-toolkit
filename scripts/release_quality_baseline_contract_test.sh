#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
checker="$repo_root/scripts/release_quality_baseline_check.sh"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT

git -C "$fixture" init -q -b master
git -C "$fixture" config user.name "release baseline contract"
git -C "$fixture" config user.email "release-baseline-contract@example.invalid"
mkdir -p "$fixture/docs"
touch "$fixture/.baseline-fixture"
git -C "$fixture" add .baseline-fixture
git -C "$fixture" commit -qm "fixture"
git -C "$fixture" tag v4.0.2
commit="$(git -C "$fixture" rev-parse v4.0.2^{commit})"

{
  printf 'release_tag\trelease_commit\tmodule\tpackage\tapi_status\ttest_status\tobserved_percent\n'
  printf 'v4.0.2\t%s\troot\t(aggregate)\taggregate\taggregate\t70.0\n' "$commit"
  printf 'v4.0.2\t%s\troot\tgithub.com/example/root\tstable\tdirect-tests\t70.0\n' "$commit"
  printf 'v4.0.2\t%s\tcontrib\t(aggregate)\taggregate\taggregate\t60.0\n' "$commit"
  printf 'v4.0.2\t%s\tcontrib\tgithub.com/example/contrib\tsupported-adapter\tdirect-tests\t60.0\n' "$commit"
} > "$fixture/docs/coverage-trend.tsv"
{
  printf 'release_tag\trelease_commit\tgo_version\tgoos\tgoarch\tcpu\tbenchmark_flags\tmodule\tpackage\tbenchmark\tobserved_ns_per_op\tobserved_bytes_per_op\tmax_bytes_per_op\tobserved_allocs_per_op\tmax_allocs_per_op\tevidence\n'
  printf 'v4.0.2\t%s\tgo1.test\tlinux\tamd64\tcontract-cpu\t-benchtime=1x\troot\trootpkg\tBenchmarkRoot\t10\t20\t30\t2\t3\tcontract sample\n' "$commit"
  printf 'v4.0.2\t%s\tgo1.test\tlinux\tamd64\tcontract-cpu\t-benchtime=1x\tcontrib\tcontribpkg\tBenchmarkContrib\t10\t20\t30\t2\t3\tcontract sample\n' "$commit"
} > "$fixture/docs/benchmark-baselines.tsv"

RELEASE_QUALITY_REPOSITORY_ROOT="$fixture" RELEASE_QUALITY_RELEASE=v4.0.2 bash "$checker" >/dev/null

cp "$fixture/docs/coverage-trend.tsv" "$fixture/docs/coverage-trend.complete.tsv"
sed -i '/^v4\.0\.2/d' "$fixture/docs/coverage-trend.tsv"
if RELEASE_QUALITY_REPOSITORY_ROOT="$fixture" RELEASE_QUALITY_RELEASE=v4.0.2 bash "$checker" >/dev/null 2>&1; then
  echo "checker accepted an absent coverage release row" >&2
  exit 1
fi
mv "$fixture/docs/coverage-trend.complete.tsv" "$fixture/docs/coverage-trend.tsv"

if RELEASE_QUALITY_REPOSITORY_ROOT="$fixture" RELEASE_QUALITY_RELEASE='v4.0.2;touch-pwned' bash "$checker" >/dev/null 2>&1; then
  echo "checker accepted an invalid release identifier" >&2
  exit 1
fi
[ ! -e "$fixture/touch-pwned" ] || { echo "invalid release input was interpreted" >&2; exit 1; }

sed -i 's/60\.0$/not-reported/' "$fixture/docs/coverage-trend.tsv"
if RELEASE_QUALITY_REPOSITORY_ROOT="$fixture" RELEASE_QUALITY_RELEASE=v4.0.2 bash "$checker" >/dev/null 2>&1; then
  echo "checker accepted missing numeric contrib coverage" >&2
  exit 1
fi

sed -i 's/not-reported$/60.0/' "$fixture/docs/coverage-trend.tsv"
sed -i "s/$commit/0000000000000000000000000000000000000000/" "$fixture/docs/benchmark-baselines.tsv"
if RELEASE_QUALITY_REPOSITORY_ROOT="$fixture" RELEASE_QUALITY_RELEASE=v4.0.2 bash "$checker" >/dev/null 2>&1; then
  echo "checker accepted a benchmark commit mismatch" >&2
  exit 1
fi

echo "release quality baseline contract tests passed"
