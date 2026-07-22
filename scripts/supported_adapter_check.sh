#!/usr/bin/env bash
# Verifies that every supported PostgreSQL and Redis adapter has PR-blocking
# real-service evidence and is included in its local contract target.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
realism="$repo_root/docs/supported-adapter-test-realism.tsv"
contracts="$repo_root/docs/supported-adapter-contracts.tsv"
makefile="$repo_root/Makefile"

postgres_packages=(
  "github.com/aatuh/api-toolkit/contrib/v4/adapters/auditpostgres"
  "github.com/aatuh/api-toolkit/contrib/v4/adapters/migrate"
  "github.com/aatuh/api-toolkit/contrib/v4/adapters/operationpostgres"
  "github.com/aatuh/api-toolkit/contrib/v4/adapters/outboxpostgres"
  "github.com/aatuh/api-toolkit/contrib/v4/adapters/pgxpool"
  "github.com/aatuh/api-toolkit/contrib/v4/adapters/txpostgres"
  "github.com/aatuh/api-toolkit/contrib/v4/adapters/webhookdeliverypostgres"
  "github.com/aatuh/api-toolkit/contrib/v4/integrations/pgxpool"
  "github.com/aatuh/api-toolkit/contrib/v4/integrations/txpostgres"
  "github.com/aatuh/api-toolkit/contrib/v4/migrator"
  "github.com/aatuh/api-toolkit/contrib/v4/scheduler/postgres"
)

redis_packages=(
  "github.com/aatuh/api-toolkit/contrib/v4/adapters/cacheredis"
  "github.com/aatuh/api-toolkit/contrib/v4/adapters/idempotencyredis"
  "github.com/aatuh/api-toolkit/contrib/v4/adapters/ratelimitredis"
)

verify_packages() {
  local expected_status="$1"
  local contract_evidence="$2"
  local make_packages="$3"
  shift 3
  local package realism_status relative
  for package in "$@"; do
  realism_status="$(awk -F '\t' -v package="$package" '$1 == package { print $4; found = 1; exit } END { if (!found) exit 1 }' "$realism")" || {
    echo "missing real-service evidence row for $package" >&2
    exit 1
  }
  if [ "$realism_status" != "$expected_status" ]; then
    echo "$package realism_status=$realism_status, want $expected_status" >&2
    exit 1
  fi
  if ! awk -F '\t' -v package="$package" -v evidence="$contract_evidence" '$1 == package && index($3, evidence) { found = 1 } END { exit found ? 0 : 1 }' "$contracts"; then
    echo "$package lacks $contract_evidence evidence" >&2
    exit 1
  fi
  relative="./${package#github.com/aatuh/api-toolkit/contrib/v4/}"
  if ! awk -v variable="$make_packages" -v package="$relative" '$0 ~ "^" variable "[[:space:]]*:=" && index($0, package) { found = 1 } END { exit found ? 0 : 1 }' "$makefile"; then
    echo "$package is missing from $make_packages" >&2
    exit 1
  fi
  done
}

verify_workflow() {
  local workflow="$1"
  local job="$2"
  local target="$3"
  if ! grep -Fq "$job:" "$workflow" || ! grep -Fq "make $target" "$workflow"; then
    echo "$(basename "$workflow") does not run $target through $job" >&2
    exit 1
  fi
}

verify_packages "real-postgres-pr" "real PostgreSQL contract" "POSTGRES_TEST_PACKAGES" "${postgres_packages[@]}"
verify_packages "real-redis-pr" "real Redis contract" "REDIS_TEST_PACKAGES" "${redis_packages[@]}"

verify_workflow "$repo_root/.github/workflows/ci.yml" "postgres-contract" "test-postgres"
verify_workflow "$repo_root/.github/workflows/release.yml" "postgres-contract" "test-postgres"
verify_workflow "$repo_root/.github/workflows/ci.yml" "redis-contract" "test-redis"
verify_workflow "$repo_root/.github/workflows/release.yml" "redis-contract" "test-redis"

echo "supported PostgreSQL and Redis adapter real-service evidence is complete"
