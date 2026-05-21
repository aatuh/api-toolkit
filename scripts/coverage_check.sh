#!/usr/bin/env bash
set -euo pipefail

check=0
if [[ "${1:-}" == "--check" ]]; then
  check=1
fi

go_cmd="${GO:-go}"
out_dir="${OUTPUT_DIR:-.ci-result}/coverage"
repo_root="$(pwd)"
mkdir -p "$out_dir"

root_min="${ROOT_COVERAGE_MIN:-70.0}"
contrib_min="${CONTRIB_COVERAGE_MIN:-52.0}"

run_module() {
  local name="$1"
  local dir="$2"
  local profile="$out_dir/$name.coverprofile"
  local log="$out_dir/$name.log"
  local funcs="$out_dir/$name.func"

  echo "==> $name coverage"
  (cd "$dir" && "$go_cmd" test ./... -covermode=atomic -coverprofile="$repo_root/$profile") | tee "$log"
  (cd "$dir" && "$go_cmd" tool cover -func="$repo_root/$profile") | tee "$repo_root/$funcs"
}

coverage_total() {
  awk '/^total:/ { gsub("%", "", $3); print $3 }' "$1"
}

package_coverage() {
  local log="$1"
  local pkg="$2"
  awk -v pkg="$pkg" '
    $1 == "ok" && $2 == pkg {
      for (i = 1; i <= NF; i++) {
        if ($i == "coverage:") {
          value = $(i + 1)
          gsub("%", "", value)
          print value
        }
      }
    }
  ' "$log" | tail -1
}

check_min() {
  local label="$1"
  local got="$2"
  local want="$3"
  if [[ -z "$got" ]]; then
    echo "coverage-check: missing coverage for $label" >&2
    return 1
  fi
  awk -v got="$got" -v want="$want" -v label="$label" '
    BEGIN {
      if (got + 0 < want + 0) {
        printf "coverage-check: %s coverage %.1f%% is below %.1f%%\n", label, got, want > "/dev/stderr"
        exit 1
      }
      printf "coverage-check: %s coverage %.1f%% >= %.1f%%\n", label, got, want
    }
  '
}

run_module root .
run_module contrib contrib

root_total="$(coverage_total "$out_dir/root.func")"
contrib_total="$(coverage_total "$out_dir/contrib.func")"

echo "coverage-summary: root total ${root_total}%"
echo "coverage-summary: contrib total ${contrib_total}%"

cat >"$out_dir/summary.md" <<EOF_SUMMARY
## Coverage summary

Coverage is a review signal, not a substitute for behavior and contract tests.

| Module | Total |
| --- | ---: |
| root | ${root_total}% |
| contrib | ${contrib_total}% |

Detailed function summaries:

- \`${out_dir}/root.func\`
- \`${out_dir}/contrib.func\`
EOF_SUMMARY

if [[ "$check" -eq 1 ]]; then
  check_min "root aggregate" "$root_total" "$root_min"
  check_min "contrib aggregate" "$contrib_total" "$contrib_min"

  check_min "github.com/aatuh/api-toolkit/v3/apiclient" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/apiclient")" "${APICLIENT_COVERAGE_MIN:-80.0}"
  check_min "github.com/aatuh/api-toolkit/v3/apitest" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/apitest")" "${APITEST_COVERAGE_MIN:-75.0}"
  check_min "github.com/aatuh/api-toolkit/v3/oauth2" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/oauth2")" "${OAUTH2_COVERAGE_MIN:-85.0}"
  check_min "github.com/aatuh/api-toolkit/v3/upload" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/upload")" "${UPLOAD_COVERAGE_MIN:-85.0}"
  check_min "github.com/aatuh/api-toolkit/v3/contracttest" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/contracttest")" "${CONTRACTTEST_COVERAGE_MIN:-83.0}"
  check_min "github.com/aatuh/api-toolkit/v3/endpoints/docs" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/endpoints/docs")" "${ENDPOINTS_DOCS_COVERAGE_MIN:-70.0}"
  check_min "github.com/aatuh/api-toolkit/v3/endpoints/health" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/endpoints/health")" "${HEALTH_COVERAGE_MIN:-80.0}"
  check_min "github.com/aatuh/api-toolkit/v3/httpx/identity" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/httpx/identity")" "${HTTPX_IDENTITY_COVERAGE_MIN:-70.0}"
  check_min "github.com/aatuh/api-toolkit/v3/httpx/recover" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/httpx/recover")" "${HTTPX_RECOVER_COVERAGE_MIN:-55.0}"
  check_min "github.com/aatuh/api-toolkit/v3/middleware/auth/apikey" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/middleware/auth/apikey")" "${AUTH_APIKEY_COVERAGE_MIN:-79.0}"
  check_min "github.com/aatuh/api-toolkit/v3/middleware/auth/authz" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/middleware/auth/authz")" "${AUTH_AUTHZ_COVERAGE_MIN:-77.0}"
  check_min "github.com/aatuh/api-toolkit/v3/middleware/auth/jwt" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/middleware/auth/jwt")" "${AUTH_JWT_COVERAGE_MIN:-90.0}"
  check_min "github.com/aatuh/api-toolkit/v3/middleware/auth/tenant" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/middleware/auth/tenant")" "${AUTH_TENANT_COVERAGE_MIN:-84.0}"
  check_min "github.com/aatuh/api-toolkit/v3/middleware/idempotency" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/middleware/idempotency")" "${IDEMPOTENCY_COVERAGE_MIN:-71.0}"
  check_min "github.com/aatuh/api-toolkit/v3/middleware/json" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/middleware/json")" "${JSON_MIDDLEWARE_COVERAGE_MIN:-70.0}"
  check_min "github.com/aatuh/api-toolkit/v3/middleware/maxbody" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/middleware/maxbody")" "${MAXBODY_COVERAGE_MIN:-80.0}"
  check_min "github.com/aatuh/api-toolkit/v3/middleware/querylimits" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/middleware/querylimits")" "${QUERYLIMITS_COVERAGE_MIN:-90.0}"
  check_min "github.com/aatuh/api-toolkit/v3/middleware/ratelimit" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/middleware/ratelimit")" "${RATELIMIT_COVERAGE_MIN:-68.0}"
  check_min "github.com/aatuh/api-toolkit/v3/routecontracts" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/routecontracts")" "${ROUTECONTRACTS_COVERAGE_MIN:-83.0}"
  check_min "github.com/aatuh/api-toolkit/v3/securityprofile" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/securityprofile")" "${SECURITYPROFILE_COVERAGE_MIN:-80.0}"
  check_min "github.com/aatuh/api-toolkit/v3/specs" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/specs")" "${SPECS_COVERAGE_MIN:-82.0}"
  check_min "github.com/aatuh/api-toolkit/v3/webhooks" "$(package_coverage "$out_dir/root.log" "github.com/aatuh/api-toolkit/v3/webhooks")" "${WEBHOOKS_COVERAGE_MIN:-80.0}"

  check_min "github.com/aatuh/api-toolkit/contrib/v3/adapters/auditpostgres" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/auditpostgres")" "${CONTRIB_AUDITPOSTGRES_COVERAGE_MIN:-90.0}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/adapters/cacheredis" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/cacheredis")" "${CONTRIB_CACHEREDIS_COVERAGE_MIN:-74.0}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/adapters/idempotencyredis" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/idempotencyredis")" "${CONTRIB_IDEMPOTENCYREDIS_COVERAGE_MIN:-68.0}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/adapters/objectstores3" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/objectstores3")" "${CONTRIB_OBJECTSTORES3_COVERAGE_MIN:-75.0}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/adapters/operationpostgres" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/operationpostgres")" "${CONTRIB_OPERATIONPOSTGRES_COVERAGE_MIN:-80.0}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/adapters/outboxpostgres" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/outboxpostgres")" "${CONTRIB_OUTBOXPOSTGRES_COVERAGE_MIN:-86.0}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/adapters/pgxpool" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/pgxpool")" "${CONTRIB_PGXPOOL_COVERAGE_MIN:-71.0}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/adapters/ratelimitredis" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/ratelimitredis")" "${CONTRIB_RATELIMITREDIS_COVERAGE_MIN:-65.0}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/adapters/webhookdeliverypostgres" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/webhookdeliverypostgres")" "${CONTRIB_WEBHOOKDELIVERYPOSTGRES_COVERAGE_MIN:-93.0}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/bootstrap" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/bootstrap")" "${CONTRIB_BOOTSTRAP_COVERAGE_MIN:-71.5}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/middleware/auth/oidc" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/middleware/auth/oidc")" "${CONTRIB_AUTH_OIDC_COVERAGE_MIN:-76.0}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics")" "${CONTRIB_METRICS_COVERAGE_MIN:-74.0}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/middleware/openapi" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/middleware/openapi")" "${CONTRIB_OPENAPI_COVERAGE_MIN:-85.0}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/middleware/requestlog" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/middleware/requestlog")" "${CONTRIB_REQUESTLOG_COVERAGE_MIN:-69.0}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/webhookdelivery" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/webhookdelivery")" "${CONTRIB_WEBHOOKDELIVERY_COVERAGE_MIN:-88.0}"
fi
