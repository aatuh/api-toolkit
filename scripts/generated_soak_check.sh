#!/usr/bin/env bash
set -euo pipefail

repo_root="${GENERATED_SOAK_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
result_dir="${GENERATED_SOAK_RESULT_DIR:-.ci-result/generated-soak}"
duration_seconds="${GENERATED_SOAK_DURATION_SECONDS:-300}"
integration_cycles="${GENERATED_SOAK_INTEGRATION_CYCLES:-3}"
race_workers="${GENERATED_SOAK_RACE_WORKERS:-8}"

case "$result_dir" in
  ""|/*|..|../*|*/..|*/../*)
    echo "GENERATED_SOAK_RESULT_DIR must be a repo-relative path without .. components" >&2
    exit 2
    ;;
esac

require_int_range() {
  local name="$1"
  local value="$2"
  local min="$3"
  local max="$4"
  case "$value" in
    ""|*[!0-9]*)
      printf '%s must be an integer between %s and %s\n' "$name" "$min" "$max" >&2
      exit 2
      ;;
  esac
  if [ "$value" -lt "$min" ] || [ "$value" -gt "$max" ]; then
    printf '%s must be an integer between %s and %s\n' "$name" "$min" "$max" >&2
    exit 2
  fi
}

require_int_range GENERATED_SOAK_DURATION_SECONDS "$duration_seconds" 1 7200
require_int_range GENERATED_SOAK_INTEGRATION_CYCLES "$integration_cycles" 1 50
require_int_range GENERATED_SOAK_RACE_WORKERS "$race_workers" 1 256

mkdir -p "$repo_root/$result_dir"
tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

service_dir="$tmpdir/full-api"
status="failed"
status_path="$result_dir/status"
summary_path="$result_dir/summary.json"
generate_log="$result_dir/generate.log"
race_log="$result_dir/race-soak.log"
build_log="$result_dir/build-and-contracts.log"

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

integration_logs_json() {
  local first=true
  local cycle
  printf '['
  for cycle in $(seq 1 "$integration_cycles"); do
    if [ "$first" = true ]; then
      first=false
    else
      printf ','
    fi
    json_string "$result_dir/integration-cycle-$cycle.log"
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
  "auth": "api-key",
  "duration_seconds": $duration_seconds,
  "integration_cycles": $integration_cycles,
  "race_workers": $race_workers,
  "logs": {
    "generate": "$(json_escape "$generate_log")",
    "race_soak": "$(json_escape "$race_log")",
    "build_and_contracts": "$(json_escape "$build_log")",
    "integration_cycles": $(integration_logs_json)
  },
  "soak_contract": {
    "generated_checks": $(json_string_array "go mod tidy" "make build" "make contracts-lint" "make contracts-diff" "make openapi-check" "make client-check"),
    "race_and_leak_checks": $(json_string_array "go test -race ./internal/httpapi -run TestGeneratedFullProfileSoakNoGoroutineGrowth" "runtime.NumGoroutine before/after threshold" "GENERATED_SOAK_TEST_DURATION"),
    "connection_leak_checks": $(json_string_array "repeated make integration-check" "Postgres/Redis Docker runtime restart cycles")
  }
}
JSON
}

write_generated_soak_test() {
  mkdir -p "$service_dir/internal/httpapi"
  cat >"$service_dir/internal/httpapi/generated_soak_test.go" <<'GO'
package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ratelimitmw "github.com/aatuh/api-toolkit/v3/middleware/ratelimit"
)

const (
	generatedSoakAuthHeader  = "generated-soak-auth-header"
	generatedSoakAdminHeader = "generated-soak-admin-header"
	generatedSoakActorID     = "generated-soak-actor"
)

func TestGeneratedFullProfileSoakNoGoroutineGrowth(t *testing.T) {
	t.Setenv("ENV", "test")
	t.Setenv("API_ACTOR_ID", "")

	duration := generatedSoakDuration(t)
	workers := generatedSoakWorkers(t)
	handler := generatedSoakRouter(t)
	tenantID := generatedSoakTenant(t, handler)

	before := runtime.NumGoroutine()
	deadline := time.Now().Add(duration)
	var requests int64
	var unexpected int64
	var firstFailure string
	var firstFailureMu sync.Mutex
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(deadline) {
				index := atomic.AddInt64(&requests, 1)
				name, status, want := generatedSoakRequest(handler, tenantID, index)
				if status != want {
					atomic.AddInt64(&unexpected, 1)
					firstFailureMu.Lock()
					if firstFailure == "" {
						firstFailure = fmt.Sprintf("%s status=%d want=%d", name, status, want)
					}
					firstFailureMu.Unlock()
				}
				time.Sleep(25 * time.Millisecond)
			}
		}()
	}
	wg.Wait()
	runtime.GC()
	time.Sleep(250 * time.Millisecond)
	after := runtime.NumGoroutine()

	if got := atomic.LoadInt64(&requests); got == 0 {
		t.Fatal("soak issued no requests")
	}
	if got := atomic.LoadInt64(&unexpected); got != 0 {
		t.Fatalf("soak saw %d unexpected statuses; first=%s", got, firstFailure)
	}
	if after > before+20 {
		t.Fatalf("goroutine count grew from %d to %d", before, after)
	}
}

func generatedSoakDuration(t *testing.T) time.Duration {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv("GENERATED_SOAK_TEST_DURATION"))
	if raw == "" {
		return 30 * time.Second
	}
	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 || duration > 2*time.Hour {
		t.Fatalf("GENERATED_SOAK_TEST_DURATION=%q is invalid", raw)
	}
	return duration
}

func generatedSoakWorkers(t *testing.T) int {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv("GENERATED_SOAK_RACE_WORKERS"))
	if raw == "" {
		return 8
	}
	workers, err := strconv.Atoi(raw)
	if err != nil || workers < 1 || workers > 256 {
		t.Fatalf("GENERATED_SOAK_RACE_WORKERS=%q is invalid", raw)
	}
	return workers
}

func generatedSoakRouter(t *testing.T) http.Handler {
	t.Helper()
	limiter, err := ratelimitmw.New(ratelimitmw.Options{
		Capacity:     1000000,
		RefillRate:   1000000,
		RetryAfter:   time.Second,
		Key:          func(*http.Request) string { return "generated-full-profile-soak" },
		HeaderConfig: ratelimitmw.DefaultHeaderConfig(),
	})
	if err != nil {
		t.Fatalf("new rate limiter: %v", err)
	}
	return NewRouter(RouterConfig{
		APIKey:    generatedSoakAuthHeader,
		AdminKey:  generatedSoakAdminHeader,
		RateLimit: limiter,
	})
}

func generatedSoakTenant(t *testing.T, handler http.Handler) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/organizations", strings.NewReader(`{"name":"Generated Soak"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", generatedSoakAuthHeader)
	req.Header.Set("X-Actor-ID", generatedSoakActorID)
	req.Header.Set("Idempotency-Key", "generated-soak-organization")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed organization status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode seed organization response: %v", err)
	}
	if strings.TrimSpace(body.ID) == "" {
		t.Fatalf("seed organization response missing id: %s", rec.Body.String())
	}
	return body.ID
}

func generatedSoakRequest(handler http.Handler, tenantID string, index int64) (string, int, int) {
	switch {
	case index%10 == 0:
		return generatedSoakDo(handler, http.MethodGet, "/widgets", "", "", "", "", http.StatusUnauthorized)
	case index%4 == 0:
		return generatedSoakDo(handler, http.MethodGet, "/readyz", "", "", "", "", http.StatusOK)
	case index%3 == 0:
		return generatedSoakDo(handler, http.MethodPost, "/widgets", fmt.Sprintf(`{"name":"generated-soak-%06d"}`, index), tenantID, generatedSoakActorID, fmt.Sprintf("generated-soak-widget-%06d", index), http.StatusCreated)
	default:
		return generatedSoakDo(handler, http.MethodGet, "/widgets", "", tenantID, generatedSoakActorID, "", http.StatusOK)
	}
}

func generatedSoakDo(handler http.Handler, method, path, body, tenantID, actorID, idempotencyKey string, want int) (string, int, int) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
	req.RemoteAddr = "127.0.0.1:12345"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if tenantID != "" || actorID != "" || idempotencyKey != "" {
		req.Header.Set("X-API-Key", generatedSoakAuthHeader)
	}
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	if actorID != "" {
		req.Header.Set("X-Actor-ID", actorID)
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return method + " " + path, rec.Code, want
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
      --dir "$service_dir" \
      --core-replace "$repo_root" \
      --contrib-replace "$repo_root/contrib"
  ) >"$repo_root/$generate_log" 2>&1 || return

  write_generated_soak_test

  (
    cd "$service_dir"
    go mod tidy
    make build
    make contracts-lint
    make contracts-diff
    make openapi-check
    make client-check
  ) >"$repo_root/$build_log" 2>&1 || return

  (
    cd "$service_dir"
    GENERATED_SOAK_TEST_DURATION="${duration_seconds}s" \
      GENERATED_SOAK_RACE_WORKERS="$race_workers" \
      go test -race ./internal/httpapi -run TestGeneratedFullProfileSoakNoGoroutineGrowth -count=1
  ) >"$repo_root/$race_log" 2>&1 || return

  local cycle
  for cycle in $(seq 1 "$integration_cycles"); do
    (
      cd "$service_dir"
      make integration-check
    ) >"$repo_root/$result_dir/integration-cycle-$cycle.log" 2>&1 || return
  done
}

if run_check; then
  status="passed"
fi

printf '%s\n' "$status" >"$repo_root/$status_path"
write_summary
printf 'generated soak check %s; summary=%s\n' "$status" "$summary_path"
if [ "$status" != "passed" ]; then
  exit 1
fi
