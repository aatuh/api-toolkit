#!/usr/bin/env bash
set -euo pipefail

check=0
if [[ "${1:-}" == "--check" ]]; then
  check=1
fi

go_cmd="${GO:-go}"
out_dir="${OUTPUT_DIR:-.ci-result}/coverage"
repo_root="$(pwd)"
classification="$repo_root/docs/package-classification.tsv"
root_module="github.com/aatuh/api-toolkit/v3"
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
    $1 == "?" && $2 == pkg && index($0, "[no test files]") > 0 {
      print "no-test-files"
    }
    $1 == "ok" && $2 == pkg {
      if (index($0, "coverage: [no statements]") > 0) {
        print "no-statements"
        next
      }
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

coverage_floor() {
  local env_name="$1"
  local default_value="$2"
  printf '%s' "${!env_name:-$default_value}"
}

coverage_floor_env() {
  case "$1" in
    github.com/aatuh/api-toolkit/v3/apiclient) printf '%s' "APICLIENT_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/apitest) printf '%s' "APITEST_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/oauth2) printf '%s' "OAUTH2_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/upload) printf '%s' "UPLOAD_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/contracttest) printf '%s' "CONTRACTTEST_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/endpoints/docs) printf '%s' "ENDPOINTS_DOCS_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/endpoints/health) printf '%s' "HEALTH_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/httpx/identity) printf '%s' "HTTPX_IDENTITY_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/httpx/recover) printf '%s' "HTTPX_RECOVER_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/middleware/auth/apikey) printf '%s' "AUTH_APIKEY_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/middleware/auth/authz) printf '%s' "AUTH_AUTHZ_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/middleware/auth/jwt) printf '%s' "AUTH_JWT_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/middleware/auth/tenant) printf '%s' "AUTH_TENANT_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/middleware/idempotency) printf '%s' "IDEMPOTENCY_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/middleware/json) printf '%s' "JSON_MIDDLEWARE_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/middleware/maxbody) printf '%s' "MAXBODY_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/middleware/querylimits) printf '%s' "QUERYLIMITS_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/middleware/ratelimit) printf '%s' "RATELIMIT_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/routecontracts) printf '%s' "ROUTECONTRACTS_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/securityprofile) printf '%s' "SECURITYPROFILE_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/specs) printf '%s' "SPECS_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/v3/webhooks) printf '%s' "WEBHOOKS_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/contrib/v3/adapters/auditpostgres) printf '%s' "CONTRIB_AUDITPOSTGRES_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/contrib/v3/adapters/cacheredis) printf '%s' "CONTRIB_CACHEREDIS_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/contrib/v3/adapters/idempotencyredis) printf '%s' "CONTRIB_IDEMPOTENCYREDIS_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/contrib/v3/adapters/objectstores3) printf '%s' "CONTRIB_OBJECTSTORES3_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/contrib/v3/adapters/operationpostgres) printf '%s' "CONTRIB_OPERATIONPOSTGRES_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/contrib/v3/adapters/outboxpostgres) printf '%s' "CONTRIB_OUTBOXPOSTGRES_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/contrib/v3/adapters/pgxpool) printf '%s' "CONTRIB_PGXPOOL_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/contrib/v3/adapters/ratelimitredis) printf '%s' "CONTRIB_RATELIMITREDIS_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/contrib/v3/adapters/webhookdeliverypostgres) printf '%s' "CONTRIB_WEBHOOKDELIVERYPOSTGRES_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/contrib/v3/bootstrap) printf '%s' "CONTRIB_BOOTSTRAP_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/contrib/v3/middleware/auth/oidc) printf '%s' "CONTRIB_AUTH_OIDC_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics) printf '%s' "CONTRIB_METRICS_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/contrib/v3/middleware/openapi) printf '%s' "CONTRIB_OPENAPI_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/contrib/v3/middleware/oteltrace) printf '%s' "CONTRIB_OTELTRACE_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/contrib/v3/middleware/requestlog) printf '%s' "CONTRIB_REQUESTLOG_COVERAGE_MIN" ;;
    github.com/aatuh/api-toolkit/contrib/v3/webhookdelivery) printf '%s' "CONTRIB_WEBHOOKDELIVERY_COVERAGE_MIN" ;;
    *) printf '%s' "not-enforced" ;;
  esac
}

coverage_default_floor() {
  case "$1" in
    APICLIENT_COVERAGE_MIN) printf '%s' "80.0" ;;
    APITEST_COVERAGE_MIN) printf '%s' "75.0" ;;
    OAUTH2_COVERAGE_MIN) printf '%s' "85.0" ;;
    UPLOAD_COVERAGE_MIN) printf '%s' "85.0" ;;
    CONTRACTTEST_COVERAGE_MIN) printf '%s' "83.0" ;;
    ENDPOINTS_DOCS_COVERAGE_MIN) printf '%s' "70.0" ;;
    HEALTH_COVERAGE_MIN) printf '%s' "80.0" ;;
    HTTPX_IDENTITY_COVERAGE_MIN) printf '%s' "70.0" ;;
    HTTPX_RECOVER_COVERAGE_MIN) printf '%s' "55.0" ;;
    AUTH_APIKEY_COVERAGE_MIN) printf '%s' "79.0" ;;
    AUTH_AUTHZ_COVERAGE_MIN) printf '%s' "77.0" ;;
    AUTH_JWT_COVERAGE_MIN) printf '%s' "90.0" ;;
    AUTH_TENANT_COVERAGE_MIN) printf '%s' "84.0" ;;
    IDEMPOTENCY_COVERAGE_MIN) printf '%s' "71.0" ;;
    JSON_MIDDLEWARE_COVERAGE_MIN) printf '%s' "70.0" ;;
    MAXBODY_COVERAGE_MIN) printf '%s' "80.0" ;;
    QUERYLIMITS_COVERAGE_MIN) printf '%s' "90.0" ;;
    RATELIMIT_COVERAGE_MIN) printf '%s' "68.0" ;;
    ROUTECONTRACTS_COVERAGE_MIN) printf '%s' "83.0" ;;
    SECURITYPROFILE_COVERAGE_MIN) printf '%s' "80.0" ;;
    SPECS_COVERAGE_MIN) printf '%s' "82.0" ;;
    WEBHOOKS_COVERAGE_MIN) printf '%s' "80.0" ;;
    CONTRIB_AUDITPOSTGRES_COVERAGE_MIN) printf '%s' "90.0" ;;
    CONTRIB_CACHEREDIS_COVERAGE_MIN) printf '%s' "74.0" ;;
    CONTRIB_IDEMPOTENCYREDIS_COVERAGE_MIN) printf '%s' "68.0" ;;
    CONTRIB_OBJECTSTORES3_COVERAGE_MIN) printf '%s' "75.0" ;;
    CONTRIB_OPERATIONPOSTGRES_COVERAGE_MIN) printf '%s' "80.0" ;;
    CONTRIB_OUTBOXPOSTGRES_COVERAGE_MIN) printf '%s' "86.0" ;;
    CONTRIB_PGXPOOL_COVERAGE_MIN) printf '%s' "71.0" ;;
    CONTRIB_RATELIMITREDIS_COVERAGE_MIN) printf '%s' "65.0" ;;
    CONTRIB_WEBHOOKDELIVERYPOSTGRES_COVERAGE_MIN) printf '%s' "93.0" ;;
    CONTRIB_BOOTSTRAP_COVERAGE_MIN) printf '%s' "71.5" ;;
    CONTRIB_AUTH_OIDC_COVERAGE_MIN) printf '%s' "76.0" ;;
    CONTRIB_METRICS_COVERAGE_MIN) printf '%s' "82.0" ;;
    CONTRIB_OPENAPI_COVERAGE_MIN) printf '%s' "85.0" ;;
    CONTRIB_OTELTRACE_COVERAGE_MIN) printf '%s' "90.0" ;;
    CONTRIB_REQUESTLOG_COVERAGE_MIN) printf '%s' "81.0" ;;
    CONTRIB_WEBHOOKDELIVERY_COVERAGE_MIN) printf '%s' "88.0" ;;
    *) printf '%s' "not-enforced" ;;
  esac
}

coverage_branch_notes() {
  case "$1" in
    github.com/aatuh/api-toolkit/v3/middleware/auth/*|github.com/aatuh/api-toolkit/v3/oauth2)
      printf '%s' "auth/security branches: invalid credentials, bypass controls, claims, and failure paths" ;;
    github.com/aatuh/api-toolkit/v3/middleware/idempotency|github.com/aatuh/api-toolkit/v3/idempotent)
      printf '%s' "idempotency branches: replay, conflict, ambiguous, store failure, and body/response limits" ;;
    github.com/aatuh/api-toolkit/v3/middleware/timeout|github.com/aatuh/api-toolkit/v3/httpx/recover)
      printf '%s' "runtime branches: timeout, panic, committed response, and capture overflow behavior" ;;
    github.com/aatuh/api-toolkit/v3/binding|github.com/aatuh/api-toolkit/v3/upload|github.com/aatuh/api-toolkit/v3/queryparams|github.com/aatuh/api-toolkit/v3/middleware/querylimits|github.com/aatuh/api-toolkit/v3/middleware/json|github.com/aatuh/api-toolkit/v3/middleware/maxbody|github.com/aatuh/api-toolkit/v3/negotiation|github.com/aatuh/api-toolkit/v3/webhooks)
      printf '%s' "input branches: malformed input, size limits, missing fields, content type, and parse failures" ;;
    github.com/aatuh/api-toolkit/v3/endpoints/*|github.com/aatuh/api-toolkit/v3/httpx|github.com/aatuh/api-toolkit/v3/httpcache|github.com/aatuh/api-toolkit/v3/fielderrors)
      printf '%s' "HTTP output branches: status selection, headers, Problem Details, cache validators, and field errors" ;;
    github.com/aatuh/api-toolkit/v3/securityprofile|github.com/aatuh/api-toolkit/v3/routepolicy|github.com/aatuh/api-toolkit/v3/routecontracts|github.com/aatuh/api-toolkit/v3/contracttest)
      printf '%s' "contract branches: route policy, security profile composition, OpenAPI metadata, and validation failures" ;;
    github.com/aatuh/api-toolkit/v3/scheduler|github.com/aatuh/api-toolkit/v3/scheduler/migrations|github.com/aatuh/api-toolkit/v3/operations|github.com/aatuh/api-toolkit/v3/ports)
      printf '%s' "lifecycle branches: cancellation, recorder/store failures, operation states, and compatibility ports" ;;
    *)
      printf '%s' "statement coverage reported; branch risk reviewed through direct package tests before floor changes" ;;
  esac
}

coverage_row() {
  local module="$1"
  local log="$2"
  local pkg="$3"
  local api_status="$4"
  local test_status="$5"
  local env_name default_floor floor observed branch_notes

  env_name="$(coverage_floor_env "$pkg")"
  default_floor="$(coverage_default_floor "$env_name")"
  if [[ "$env_name" == "not-enforced" ]]; then
    floor="not-enforced"
  else
    floor="$(coverage_floor "$env_name" "$default_floor")"
  fi
  observed="$(package_coverage "$log" "$pkg")"
  if [[ -z "$observed" ]]; then
    observed="not-reported"
  fi
  branch_notes="$(coverage_branch_notes "$pkg")"
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$module" "$pkg" "$api_status" "$test_status" "$env_name" "$floor" "$observed" "$branch_notes"
}

stable_root_coverage_rows() {
  awk -F '\t' -v root="$root_module" '
    !/^#/ && ($2 == "stable" || $2 == "compatibility-only") && ($1 == root || index($1, root "/") == 1) {
      print $1 "\t" $2 "\t" $3
    }
  ' "$classification" |
    while IFS=$'\t' read -r import_path api_status test_status; do
      coverage_row root "$out_dir/root.log" "$import_path" "$api_status" "$test_status"
    done
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
package_summary="$out_dir/package-summary.tsv"

echo "coverage-summary: root total ${root_total}%"
echo "coverage-summary: contrib total ${contrib_total}%"

{
  printf 'module\tpackage\tapi_status\ttest_status\tfloor_env\tfloor_percent\tobserved_percent\tbranch_notes\n'
  printf 'root\t(aggregate)\taggregate\taggregate\tROOT_COVERAGE_MIN\t%s\t%s\taggregate statement coverage across root module\n' "$(coverage_floor ROOT_COVERAGE_MIN "$root_min")" "$root_total"
  printf 'contrib\t(aggregate)\taggregate\taggregate\tCONTRIB_COVERAGE_MIN\t%s\t%s\taggregate statement coverage across contrib module\n' "$(coverage_floor CONTRIB_COVERAGE_MIN "$contrib_min")" "$contrib_total"

  stable_root_coverage_rows

  coverage_row contrib "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/auditpostgres" "supported-adapter" "direct-tests"
  coverage_row contrib "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/cacheredis" "supported-adapter" "direct-tests"
  coverage_row contrib "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/idempotencyredis" "supported-adapter" "direct-tests"
  coverage_row contrib "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/objectstores3" "supported-adapter" "direct-tests"
  coverage_row contrib "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/operationpostgres" "supported-adapter" "direct-tests"
  coverage_row contrib "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/outboxpostgres" "supported-adapter" "direct-tests"
  coverage_row contrib "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/pgxpool" "supported-adapter" "direct-tests"
  coverage_row contrib "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/ratelimitredis" "supported-adapter" "direct-tests"
  coverage_row contrib "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/adapters/webhookdeliverypostgres" "supported-adapter" "direct-tests"
  coverage_row contrib "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/bootstrap" "supported-adapter" "direct-tests"
  coverage_row contrib "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/middleware/auth/oidc" "supported-adapter" "direct-tests"
  coverage_row contrib "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics" "supported-adapter" "direct-tests"
  coverage_row contrib "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/middleware/openapi" "supported-adapter" "direct-tests"
  coverage_row contrib "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/middleware/oteltrace" "supported-adapter" "direct-tests"
  coverage_row contrib "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/middleware/requestlog" "supported-adapter" "direct-tests"
  coverage_row contrib "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/webhookdelivery" "supported-adapter" "direct-tests"
} >"$package_summary"

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
- \`${package_summary}\`
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
  check_min "github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics")" "${CONTRIB_METRICS_COVERAGE_MIN:-82.0}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/middleware/openapi" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/middleware/openapi")" "${CONTRIB_OPENAPI_COVERAGE_MIN:-85.0}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/middleware/oteltrace" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/middleware/oteltrace")" "${CONTRIB_OTELTRACE_COVERAGE_MIN:-90.0}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/middleware/requestlog" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/middleware/requestlog")" "${CONTRIB_REQUESTLOG_COVERAGE_MIN:-81.0}"
  check_min "github.com/aatuh/api-toolkit/contrib/v3/webhookdelivery" "$(package_coverage "$out_dir/contrib.log" "github.com/aatuh/api-toolkit/contrib/v3/webhookdelivery")" "${CONTRIB_WEBHOOKDELIVERY_COVERAGE_MIN:-88.0}"
fi
