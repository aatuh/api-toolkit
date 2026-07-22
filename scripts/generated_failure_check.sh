#!/usr/bin/env bash
set -euo pipefail

repo_root="${GENERATED_FAILURE_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
result_dir="${GENERATED_FAILURE_RESULT_DIR:-.ci-result/generated-failure}"
case "$result_dir" in
  ""|/*|..|../*|*/..|*/../*)
    echo "GENERATED_FAILURE_RESULT_DIR must be a repo-relative path without .. components" >&2
    exit 2
    ;;
esac

mkdir -p "$repo_root/$result_dir"
tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

api_key_service_dir="$tmpdir/full-api-key"
jwt_service_dir="$tmpdir/full-jwt"
status="failed"
status_path="$result_dir/status"
summary_path="$result_dir/summary.json"
api_key_generate_log="$result_dir/api-key-generate.log"
api_key_failure_log="$result_dir/api-key-failure-tests.log"
jwt_generate_log="$result_dir/jwt-generate.log"
jwt_failure_log="$result_dir/jwt-failure-tests.log"

json_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '%s' "$value"
}

json_string() {
  printf '"%s"' "$(json_escape "$1")"
}

json_string_array() {
  local first=true
  printf '['
  for value in "$@"; do
    if [ "$first" = true ]; then
      first=false
    else
      printf ','
    fi
    json_string "$value"
  done
  printf ']'
}

write_summary() {
  local commit timestamp toolchain
  commit="$(git -C "$repo_root" rev-parse HEAD 2>/dev/null || printf 'unknown')"
  timestamp="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  toolchain="$(go version 2>/dev/null || printf 'unknown')"

  cat >"$repo_root/$summary_path" <<JSON
{
  "status": "$(json_escape "$status")",
  "commit": "$(json_escape "$commit")",
  "timestamp": "$(json_escape "$timestamp")",
  "toolchain": "$(json_escape "$toolchain")",
  "profile": "saas-api-full",
  "auth_modes": $(json_string_array "api-key" "jwt"),
  "logs": {
    "api_key_generate": "$(json_escape "$api_key_generate_log")",
    "api_key_failure_tests": "$(json_escape "$api_key_failure_log")",
    "jwt_generate": "$(json_escape "$jwt_generate_log")",
    "jwt_failure_tests": "$(json_escape "$jwt_failure_log")"
  },
  "failure_contract": {
    "failure_cases": $(json_string_array "redis_down" "postgres_down" "expired_api_key" "bad_jwks_endpoint" "slow_downstream_timeout"),
    "generated_tests": $(json_string_array "internal/httpapi/generated_failure_test.go" "cmd/api/generated_failure_jwt_test.go"),
    "commands": $(json_string_array "go mod tidy" "go test ./internal/httpapi -run TestGeneratedFailure -count=1" "go test ./cmd/api -run TestGeneratedFailureBadJWKSURLFailsClosed -count=1")
  }
}
JSON
}

write_api_key_failure_test() {
  mkdir -p "$api_key_service_dir/internal/httpapi"
  cat >"$api_key_service_dir/internal/httpapi/generated_failure_test.go" <<'GO'
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"example.com/full-api/internal/app"
	timeoutmw "github.com/aatuh/api-toolkit/v4/middleware/timeout"
)

const (
	generatedFailureStaticAPIKey = "generated-failure-static-api-key"
	generatedFailureActorID      = "generated-failure-actor"
)

func TestGeneratedFailureReadinessFailsClosedForPostgresAndRedis(t *testing.T) {
	for _, tt := range []struct {
		name       string
		rawDetail  string
		statusName string
	}{
		{name: "postgres_down", rawDetail: "postgres unavailable", statusName: "Postgres"},
		{name: "redis_down", rawDetail: "redis unavailable", statusName: "Redis"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewRouter(RouterConfig{
				Readiness: HealthCheckFunc(func(context.Context) error { return errors.New(tt.rawDetail) }),
			})
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("%s down status = %d body=%s", tt.statusName, rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/problem+json") {
				t.Fatalf("%s down content type = %q", tt.statusName, got)
			}
			body := rec.Body.String()
			if !strings.Contains(body, "service is not ready") {
				t.Fatalf("%s down problem missing safe readiness detail: %s", tt.statusName, body)
			}
			if strings.Contains(body, tt.rawDetail) {
				t.Fatalf("%s down problem leaked dependency detail: %s", tt.statusName, body)
			}
		})
	}
}

func TestGeneratedFailureExpiredManagedAPIKeyIsRejected(t *testing.T) {
	t.Setenv("ENV", "test")
	t.Setenv("API_ACTOR_ID", "")

	tenancy := app.NewTenancyService()
	handler := NewRouter(RouterConfig{
		Tenancy: tenancy,
		APIKeys: app.NewAPIKeyService(
			"generated-failure-pepper",
			tenancy,
		),
		APIKey: generatedFailureStaticAPIKey,
	})

	orgID := generatedFailureCreateOrganization(t, handler)
	secret := generatedFailureCreateExpiredAPIKey(t, handler, orgID)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/widgets", nil)
	req.Header.Set("X-API-Key", secret)
	req.Header.Set("X-Tenant-ID", orgID)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expired API key status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("expired API key problem leaked raw secret: %s", rec.Body.String())
	}
}

func TestGeneratedFailureSlowDownstreamHardTimeoutIsBounded(t *testing.T) {
	middleware, err := timeoutmw.NewHard(timeoutmw.Options{Timeout: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("new hard timeout: %v", err)
	}
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		w.Header().Set("X-Late-Downstream", "true")
		_, _ = w.Write([]byte("late downstream response"))
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/slow-downstream?token=secret", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("slow downstream status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/problem+json") {
		t.Fatalf("slow downstream content type = %q", got)
	}
	if rec.Header().Get("X-Late-Downstream") != "" {
		t.Fatalf("late downstream header leaked: %q", rec.Header().Get("X-Late-Downstream"))
	}
	body := rec.Body.String()
	if !strings.Contains(body, "request timed out") {
		t.Fatalf("slow downstream problem missing timeout detail: %s", body)
	}
	if strings.Contains(body, "secret") || strings.Contains(body, "late downstream response") {
		t.Fatalf("slow downstream problem leaked request or late body data: %s", body)
	}
}

func generatedFailureCreateOrganization(t *testing.T, handler http.Handler) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/organizations", strings.NewReader(`{"name":"Generated Failure"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", generatedFailureStaticAPIKey)
	req.Header.Set("X-Actor-ID", generatedFailureActorID)
	req.Header.Set("Idempotency-Key", "generated-failure-organization")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create organization status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode organization body: %v", err)
	}
	if strings.TrimSpace(body.ID) == "" {
		t.Fatalf("organization response missing id: %s", rec.Body.String())
	}
	return body.ID
}

func generatedFailureCreateExpiredAPIKey(t *testing.T, handler http.Handler, orgID string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/api-keys", strings.NewReader(`{"name":"Expired","scopes":["widgets:read","widgets:write"],"expires_at":"2000-01-01T00:00:00Z"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", generatedFailureStaticAPIKey)
	req.Header.Set("X-Tenant-ID", orgID)
	req.Header.Set("X-Actor-ID", generatedFailureActorID)
	req.Header.Set("Idempotency-Key", "generated-failure-expired-api-key")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create expired API key status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode expired API key body: %v", err)
	}
	if strings.TrimSpace(body.Secret) == "" {
		t.Fatalf("expired API key response missing secret: %s", rec.Body.String())
	}
	return body.Secret
}
GO
}

write_jwt_failure_test() {
  mkdir -p "$jwt_service_dir/cmd/api"
  cat >"$jwt_service_dir/cmd/api/generated_failure_jwt_test.go" <<'GO'
package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"example.com/full-jwt/internal/httpapi"
	jwtauth "github.com/aatuh/api-toolkit/contrib/v4/middleware/auth/jwt"
	"github.com/aatuh/api-toolkit/v4/endpoints/health"
)

func TestGeneratedFailureBadJWKSURLFailsClosed(t *testing.T) {
	t.Setenv("JWT_JWKS_URL", "http://127.0.0.1:1/jwks")
	t.Setenv("JWT_ISSUER", "https://issuer.example.test/")
	t.Setenv("JWT_AUDIENCE", "saas-api-full")
	t.Setenv("JWT_ALLOWED_ALGORITHMS", "RS256")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cfg, err := httpapi.ConfigFromEnv()
	if err != nil {
		t.Fatalf("load JWT config: %v", err)
	}
	middleware, err := newJWTMiddleware(ctx, cfg)
	if err != nil {
		t.Fatalf("bad JWKS endpoint prevented middleware construction before readiness check: %v", err)
	}
	t.Cleanup(middleware.Close)

	checker := jwtauth.HealthChecker(jwtauth.Config{
		Enabled:            true,
		JWKSURL:            cfg.JWTJWKSURL,
		JWKSRefreshTimeout: 100 * time.Millisecond,
	}, nil)
	if checker == nil {
		t.Fatal("bad JWKS endpoint did not create readiness checker")
	}
	result := checker.Check(ctx)
	if result.Status == health.StatusHealthy {
		t.Fatalf("bad JWKS endpoint reported healthy: %#v", result)
	}
	if strings.Contains(result.Message, "JWT_AUDIENCE") || strings.Contains(result.Message, "JWT_ISSUER") {
		t.Fatalf("bad JWKS endpoint failed for config instead of endpoint: %#v", result)
	}
}
GO
}

run_check() {
  (
    cd "$repo_root/contrib"
    go run ./cmd/api-toolkit new service \
      --module example.com/full-api \
      --profile saas-api-full \
      --auth api-key \
      --dir "$api_key_service_dir" \
      --core-replace "$repo_root" \
      --contrib-replace "$repo_root/contrib"
  ) >"$repo_root/$api_key_generate_log" 2>&1 || return
  write_api_key_failure_test
  (
    cd "$api_key_service_dir"
    go mod tidy
    go test ./internal/httpapi -run TestGeneratedFailure -count=1
  ) >"$repo_root/$api_key_failure_log" 2>&1 || return

  (
    cd "$repo_root/contrib"
    go run ./cmd/api-toolkit new service \
      --module example.com/full-jwt \
      --profile saas-api-full \
      --auth jwt \
      --dir "$jwt_service_dir" \
      --core-replace "$repo_root" \
      --contrib-replace "$repo_root/contrib"
  ) >"$repo_root/$jwt_generate_log" 2>&1 || return
  write_jwt_failure_test
  (
    cd "$jwt_service_dir"
    go mod tidy
    go test ./cmd/api -run TestGeneratedFailureBadJWKSURLFailsClosed -count=1
  ) >"$repo_root/$jwt_failure_log" 2>&1 || return
}

if run_check; then
  status="passed"
fi

printf '%s\n' "$status" >"$repo_root/$status_path"
write_summary
printf 'generated failure check %s; summary=%s\n' "$status" "$summary_path"
if [ "$status" != "passed" ]; then
  exit 1
fi
