#!/usr/bin/env bash
# Verifies that every supported PostgreSQL adapter has PR-blocking real-service
# evidence and is included in the local test-postgres target.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
realism="$repo_root/docs/supported-adapter-test-realism.tsv"
contracts="$repo_root/docs/supported-adapter-contracts.tsv"
makefile="$repo_root/Makefile"

required_packages=(
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

for package in "${required_packages[@]}"; do
  realism_status="$(awk -F '\t' -v package="$package" '$1 == package { print $4; found = 1; exit } END { if (!found) exit 1 }' "$realism")" || {
    echo "missing real-service evidence row for $package" >&2
    exit 1
  }
  if [ "$realism_status" != "real-postgres-pr" ]; then
    echo "$package realism_status=$realism_status, want real-postgres-pr" >&2
    exit 1
  fi
  if ! awk -F '\t' -v package="$package" '$1 == package && $3 ~ /real PostgreSQL contract/ { found = 1 } END { exit found ? 0 : 1 }' "$contracts"; then
    echo "$package lacks real PostgreSQL contract evidence" >&2
    exit 1
  fi
  relative="./${package#github.com/aatuh/api-toolkit/contrib/v4/}"
  if ! grep -Fq "$relative" "$makefile"; then
    echo "$package is missing from POSTGRES_TEST_PACKAGES" >&2
    exit 1
  fi
done

if ! grep -Fq 'postgres-contract:' "$repo_root/.github/workflows/ci.yml" ||
  ! grep -Fq 'make test-postgres' "$repo_root/.github/workflows/ci.yml"; then
  echo "CI does not run the PostgreSQL contract target" >&2
  exit 1
fi
if ! grep -Fq 'postgres-contract:' "$repo_root/.github/workflows/release.yml" ||
  ! grep -Fq 'make test-postgres' "$repo_root/.github/workflows/release.yml"; then
  echo "release workflow does not run the PostgreSQL contract target" >&2
  exit 1
fi

echo "supported PostgreSQL adapter real-service evidence is complete"
