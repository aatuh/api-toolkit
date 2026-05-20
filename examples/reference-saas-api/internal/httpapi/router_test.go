package httpapi

import (
	"bytes"
	"context"

	"encoding/json"
	"errors"

	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"example.com/reference-saas-api/internal/app"
	metricsmw "github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics"
)

func TestReadinessAndOpenAPI(t *testing.T) {
	handler := newTestRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("live status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ready status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/docs/openapi.json", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("openapi status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"operationId\": \"createWidget\"") {
		t.Fatalf("openapi missing createWidget operation: %s", rec.Body.String())
	}
}

func TestReadinessReportsDependencyFailure(t *testing.T) {
	handler := NewRouter(RouterConfig{
		Readiness: HealthCheckFunc(func(context.Context) error { return errors.New("postgres unavailable") }),
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready failure status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "postgres unavailable") {
		t.Fatalf("ready failure leaked dependency error: %s", rec.Body.String())
	}
}

func TestOpenAPIValidationRejectsInvalidRequests(t *testing.T) {
	configureTestAuthEnv(t)
	validator, err := NewOpenAPIValidationMiddleware(Config{OpenAPIRequestValidation: true})
	if err != nil {
		t.Fatalf("new openapi validator: %v", err)
	}
	tenancy := app.NewTenancyService()
	handler := NewRouter(RouterConfig{Widgets: app.NewWidgetService(), Tenancy: tenancy, APIKeys: app.NewAPIKeyService("test-pepper", tenancy), OpenAPIValidation: validator, APIKey: "test-key"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader(`{"name":"validated"}`))
	req.Header.Set("Content-Type", "text/plain")
	authorizeTestRequest(t, req, "org_validation")
	req.Header.Set("Idempotency-Key", "openapi-validation")
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusCreated {
		t.Fatalf("expected OpenAPI request validation failure, got status %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "unsupported content type") {
		t.Fatalf("validation response body = %s", rec.Body.String())
	}
}

func TestAdminMetricsRecordsHTTPRequestsWithoutSecrets(t *testing.T) {
	handler, adminHandler := newTestRouterWithMetrics(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader("{\"name\":\"metric-widget\"}"))
	authorizeTestRequestAs(t, req, "org_metric_secret", "actor_metric_secret", "widgets:write")
	req.Header.Set("Idempotency-Key", "idem_metric_secret")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create widget status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("X-Admin-Key", "test-admin-key")
	adminHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"http_requests_total", "http_request_duration_seconds", `route="POST /widgets"`, `status="201"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body missing %q:\n%s", want, body)
		}
	}
	for _, secret := range []string{"org_metric_secret", "actor_metric_secret", "idem_metric_secret", "test-key", "test-admin-key"} {
		if strings.Contains(body, secret) {
			t.Fatalf("metrics body leaked secret %q:\n%s", secret, body)
		}
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	adminHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated metrics status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminPprofRequiresAdminAndServesProfiles(t *testing.T) {
	publicHandler := newTestRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	publicHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("public pprof status = %d body=%s", rec.Code, rec.Body.String())
	}

	adminHandler := NewAdminRouter(RouterConfig{AdminKey: "test-admin-key"})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	adminHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated admin pprof status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	req.Header.Set("X-Admin-Key", "test-admin-key")
	adminHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Types of profiles available") {
		t.Fatalf("admin pprof index status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/debug/pprof/cmdline", nil)
	req.Header.Set("X-Admin-Key", "test-admin-key")
	adminHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin pprof cmdline status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateWidgetRequiresAuth(t *testing.T) {
	handler := newTestRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader("{\"name\":\"alpha\"}"))
	req.Header.Set("X-Tenant-ID", "org_1")
	req.Header.Set("Idempotency-Key", "idem_1")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/problem+json") {
		t.Fatalf("auth failure content type = %q", got)
	}
}

func TestProtectedRoutesAreRateLimited(t *testing.T) {
	handler := newTestRouter(t)
	limited := false
	for i := 0; i < 30; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/organizations", nil)
		authorizeTestRequestAs(t, req, "", "owner_1", "organizations:read")
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			limited = true
			if rec.Header().Get("Retry-After") == "" || !strings.Contains(rec.Body.String(), "rate limit exceeded") {
				t.Fatalf("rate limit response headers/body missing: headers=%v body=%s", rec.Header(), rec.Body.String())
			}
			break
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("protected request status = %d body=%s", rec.Code, rec.Body.String())
		}
	}
	if !limited {
		t.Fatal("expected protected route to return 429 after repeated requests")
	}
}

func TestCreateWidgetValidatesBody(t *testing.T) {
	handler := newTestRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader("{\"name\":\"\"}"))
	authorizeTestRequest(t, req, "org_1")
	req.Header.Set("X-Tenant-ID", "org_1")
	req.Header.Set("Idempotency-Key", "idem_1")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("validation status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateWidgetReplaysIdempotencyKey(t *testing.T) {
	handler := newTestRouter(t)
	first := createWidget(t, handler, "idem_1")
	second := createWidget(t, handler, "idem_1")
	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d body=%s", first.Code, first.Body.String())
	}
	if second.Code != http.StatusCreated {
		t.Fatalf("second replay status = %d body=%s", second.Code, second.Body.String())
	}
	if second.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("second replay header = %q", second.Header().Get("Idempotency-Replayed"))
	}
	var firstBody, secondBody map[string]any
	if err := json.Unmarshal(first.Body.Bytes(), &firstBody); err != nil {
		t.Fatalf("decode first body: %v", err)
	}
	if err := json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil {
		t.Fatalf("decode second body: %v", err)
	}
	if firstBody["id"] != secondBody["id"] {
		t.Fatalf("idempotent replay changed id: first=%v second=%v", firstBody["id"], secondBody["id"])
	}
	if first.Header().Get("ETag") == "" || second.Header().Get("ETag") == "" {
		t.Fatalf("missing ETag on create/replay")
	}
}

func TestUpdateWidgetRequiresMatchingETag(t *testing.T) {
	handler := newTestRouter(t)
	created := createWidget(t, handler, "idem_create")
	var body map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode created body: %v", err)
	}
	id, _ := body["id"].(string)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/widgets/"+id, strings.NewReader("{\"name\":\"beta\"}"))
	authorizeTestRequest(t, req, "org_1")
	req.Header.Set("X-Tenant-ID", "org_1")
	req.Header.Set("Idempotency-Key", "idem_update")
	req.Header.Set("If-Match", "\"999\"")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("conflict status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWidgetImportReturnsPollableOperation(t *testing.T) {
	handler := newTestRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/widgets/imports", strings.NewReader(`{"items":[{"name":"bulk-a"},{"name":"bulk-b"}]}`))
	authorizeTestRequestAs(t, req, "org_1", "user_123", "widgets:write")
	req.Header.Set("Idempotency-Key", "import-1")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("import status = %d body=%s", rec.Code, rec.Body.String())
	}
	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, "/operations/") || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("operation headers Location=%q Retry-After=%q", location, rec.Header().Get("Retry-After"))
	}
	var accepted map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &accepted); err != nil {
		t.Fatalf("decode accepted body: %v", err)
	}
	operationID, _ := accepted["id"].(string)
	if operationID == "" || accepted["state"] != "pending" {
		t.Fatalf("accepted body = %#v", accepted)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, location, nil)
	authorizeTestRequestAs(t, req, "org_1", "user_123", "operations:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), operationID) || !strings.Contains(rec.Body.String(), "pending") {
		t.Fatalf("operation poll status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOrganizationInvitationFlow(t *testing.T) {
	handler := newTestRouter(t)
	orgID := createOrganization(t, handler, "owner_1", "Acme")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/organizations", nil)
	authorizeTestRequestAs(t, req, "", "owner_1", "organizations:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), orgID) {
		t.Fatalf("list organizations status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/organizations/"+orgID+"/members", nil)
	authorizeTestRequestAs(t, req, orgID, "owner_1", "members:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "owner_1") {
		t.Fatalf("list members status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/invitations", strings.NewReader(`{"email":"other@example.com","role":"member"}`))
	authorizeTestRequestAs(t, req, orgID, "stranger_1", "invitations:write")
	req.Header.Set("Idempotency-Key", "invite-stranger")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("stranger invite status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/invitations", strings.NewReader(`{"email":"Member@Example.com","role":"member"}`))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "invitations:write")
	req.Header.Set("Idempotency-Key", "invite-member")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite status = %d body=%s", rec.Code, rec.Body.String())
	}
	var inviteBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &inviteBody); err != nil {
		t.Fatalf("decode invite body: %v", err)
	}
	token, _ := inviteBody["token"].(string)
	if token == "" {
		t.Fatalf("invite body missing token: %#v", inviteBody)
	}
	invitation, _ := inviteBody["invitation"].(map[string]any)
	invitationID, _ := invitation["id"].(string)
	if invitationID == "" || invitation["email"] != "member@example.com" {
		t.Fatalf("invitation body = %#v", inviteBody)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/invitations/"+invitationID+"/accept", strings.NewReader(`{"token":"wrong-token"}`))
	authorizeTestRequestAs(t, req, orgID, "member_1", "invitations:accept")
	req.Header.Set("Idempotency-Key", "accept-wrong")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("wrong token accept status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), token) {
		t.Fatalf("wrong-token problem leaked invitation token: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/invitations/"+invitationID+"/accept", strings.NewReader("{\"token\":\""+token+"\"}"))
	authorizeTestRequestAs(t, req, orgID, "member_1", "invitations:accept")
	req.Header.Set("Idempotency-Key", "accept-member")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "member_1") {
		t.Fatalf("accept status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/invitations/"+invitationID+"/accept", strings.NewReader("{\"token\":\""+token+"\"}"))
	authorizeTestRequestAs(t, req, orgID, "member_2", "invitations:accept")
	req.Header.Set("Idempotency-Key", "accept-replay")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("replay accept status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/api-keys", strings.NewReader(`{"name":"CI","scopes":["widgets:read","widgets:write"],"expires_at":"2030-01-01T00:00:00Z"}`))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "api-keys:write")
	req.Header.Set("Idempotency-Key", "create-api-key")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create api key status = %d body=%s", rec.Code, rec.Body.String())
	}
	var apiKeyBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &apiKeyBody); err != nil {
		t.Fatalf("decode api key body: %v", err)
	}
	secret, _ := apiKeyBody["secret"].(string)
	apiKey, _ := apiKeyBody["api_key"].(map[string]any)
	apiKeyID, _ := apiKey["id"].(string)
	if secret == "" || apiKeyID == "" || apiKey["prefix"] == "" {
		t.Fatalf("api key body = %#v", apiKeyBody)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader(`{"name":"managed-key-widget"}`))
	req.Header.Set("X-API-Key", secret)
	req.Header.Set("X-Tenant-ID", orgID)
	req.Header.Set("Idempotency-Key", "managed-key-widget")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("managed api key widget create status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader(`{"name":"wrong-tenant"}`))
	req.Header.Set("X-API-Key", secret)
	req.Header.Set("X-Tenant-ID", orgID+"-other")
	req.Header.Set("Idempotency-Key", "managed-key-wrong-tenant")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("managed api key tenant mismatch status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("tenant mismatch problem leaked api key secret: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/api-keys", strings.NewReader(`{"name":"Read Only","scopes":["widgets:read"]}`))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "api-keys:write")
	req.Header.Set("Idempotency-Key", "create-read-only-api-key")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create read-only api key status = %d body=%s", rec.Code, rec.Body.String())
	}
	var readOnlyBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &readOnlyBody); err != nil {
		t.Fatalf("decode read-only api key body: %v", err)
	}
	readOnlySecret, _ := readOnlyBody["secret"].(string)
	if readOnlySecret == "" {
		t.Fatalf("read-only api key response missing secret: %#v", readOnlyBody)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader(`{"name":"missing-scope"}`))
	req.Header.Set("X-API-Key", readOnlySecret)
	req.Header.Set("X-Tenant-ID", orgID)
	req.Header.Set("Idempotency-Key", "managed-key-missing-scope")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("managed api key missing scope status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), readOnlySecret) {
		t.Fatalf("scope failure problem leaked api key secret: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/organizations/"+orgID+"/api-keys", nil)
	authorizeTestRequestAs(t, req, orgID, "owner_1", "api-keys:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), apiKeyID) {
		t.Fatalf("list api keys status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("list api keys leaked secret: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/organizations/"+orgID+"/api-keys/"+apiKeyID, nil)
	authorizeTestRequestAs(t, req, orgID, "owner_1", "api-keys:write")
	req.Header.Set("Idempotency-Key", "revoke-api-key")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke api key status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/widgets", nil)
	req.Header.Set("X-API-Key", secret)
	req.Header.Set("X-Tenant-ID", orgID)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked managed api key status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("revoked api key problem leaked secret: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/api-keys", strings.NewReader(`{"name":"Member","scopes":["widgets:read"]}`))
	authorizeTestRequestAs(t, req, orgID, "member_1", "api-keys:write")
	req.Header.Set("Idempotency-Key", "member-api-key")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member api key create status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/invitations", strings.NewReader(`{"email":"second@example.com","role":"viewer"}`))
	authorizeTestRequestAs(t, req, orgID, "member_1", "invitations:write")
	req.Header.Set("Idempotency-Key", "invite-second")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member invite status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWriteRoutesRecordAuditWithoutSecrets(t *testing.T) {
	handler, auditLog := newTestRouterWithAudit(t)
	orgID := createOrganization(t, handler, "owner_1", "Acme")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/api-keys", strings.NewReader(`{"name":"CI","scopes":["widgets:read","widgets:write"]}`))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "api-keys:write")
	req.Header.Set("Idempotency-Key", "audit-api-key")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create api key status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode api key body: %v", err)
	}
	secret, _ := body["secret"].(string)
	if secret == "" {
		t.Fatalf("api key response missing secret: %#v", body)
	}

	events, err := auditLog.Events(context.Background())
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("marshal audit events: %v", err)
	}
	if !strings.Contains(string(encoded), "organization.create") || !strings.Contains(string(encoded), "api_key.create") {
		t.Fatalf("audit events missing expected actions: %s", encoded)
	}
	if strings.Contains(string(encoded), secret) {
		t.Fatalf("audit events leaked api key secret: %s", encoded)
	}
}

func TestWebhookEndpointDeliveryAndReplayFlow(t *testing.T) {
	handler := newTestRouter(t)
	orgID := createOrganization(t, handler, "owner_1", "Acme")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/webhook-events", nil)
	authorizeTestRequestAs(t, req, "", "owner_1", "webhooks:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "widget.created") {
		t.Fatalf("webhook event catalog status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("first webhook event catalog cache header = %q", rec.Header().Get("X-Cache"))
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/webhook-events", nil)
	authorizeTestRequestAs(t, req, "", "owner_1", "webhooks:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("cached webhook event catalog status = %d cache=%q body=%s", rec.Code, rec.Header().Get("X-Cache"), rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/webhook-endpoints", strings.NewReader(`{"url":"https://example.com/webhooks/widgets","events":["widget.created"]}`))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "webhooks:write")
	req.Header.Set("Idempotency-Key", "create-webhook")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create webhook endpoint status = %d body=%s", rec.Code, rec.Body.String())
	}
	var endpointBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &endpointBody); err != nil {
		t.Fatalf("decode endpoint body: %v", err)
	}
	secret, _ := endpointBody["secret"].(string)
	endpoint, _ := endpointBody["endpoint"].(map[string]any)
	endpointID, _ := endpoint["id"].(string)
	if secret == "" || endpointID == "" {
		t.Fatalf("endpoint response = %#v", endpointBody)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/organizations/"+orgID+"/webhook-endpoints", nil)
	authorizeTestRequestAs(t, req, orgID, "owner_1", "webhooks:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), endpointID) {
		t.Fatalf("list webhook endpoints status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("list webhook endpoints leaked signing secret: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader(`{"name":"emits-webhook"}`))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "widgets:write")
	req.Header.Set("Idempotency-Key", "webhook-widget")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create widget for webhook status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/organizations/"+orgID+"/webhook-deliveries", nil)
	authorizeTestRequestAs(t, req, orgID, "owner_1", "webhooks:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "widget.created") || !strings.Contains(rec.Body.String(), endpointID) {
		t.Fatalf("list webhook deliveries status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("list webhook deliveries leaked signing secret: %s", rec.Body.String())
	}
	var deliveryBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &deliveryBody); err != nil {
		t.Fatalf("decode delivery list: %v", err)
	}
	items, _ := deliveryBody["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("delivery items = %#v", deliveryBody)
	}
	first, _ := items[0].(map[string]any)
	deliveryID, _ := first["id"].(string)
	if deliveryID == "" {
		t.Fatalf("delivery item missing id: %#v", first)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/webhook-deliveries/"+deliveryID+"/replay", strings.NewReader(`{}`))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "webhooks:write")
	req.Header.Set("Idempotency-Key", "replay-webhook")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), deliveryID) {
		t.Fatalf("replay webhook delivery status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("replay webhook delivery leaked signing secret: %s", rec.Body.String())
	}
}

func TestObjectStorageFlowRejectsUnsafeInputsAndDoesNotLeakPayload(t *testing.T) {
	handler := newTestRouter(t)
	orgID := createOrganization(t, handler, "owner_1", "Acme")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/objects", strings.NewReader(`{"key":"readme.txt","content_type":"text/plain","content_base64":"aGVsbG8="}`))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "objects:write")
	req.Header.Set("Idempotency-Key", "put-object")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), "readme.txt") {
		t.Fatalf("put object status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "aGVsbG8=") {
		t.Fatalf("put object response leaked payload: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/organizations/"+orgID+"/objects", nil)
	authorizeTestRequestAs(t, req, orgID, "owner_1", "objects:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "readme.txt") {
		t.Fatalf("list objects status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "aGVsbG8=") {
		t.Fatalf("list objects leaked payload: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/organizations/"+orgID+"/objects/readme.txt", nil)
	authorizeTestRequestAs(t, req, orgID, "owner_1", "objects:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "\"content_base64\":\"aGVsbG8=\"") {
		t.Fatalf("get object status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/objects", strings.NewReader(`{"key":"../secret","content_type":"text/plain","content_base64":"ZG8tbm90LWxlYWs="}`))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "objects:write")
	req.Header.Set("Idempotency-Key", "put-unsafe-object")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unsafe object status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "ZG8tbm90LWxlYWs=") || strings.Contains(rec.Body.String(), "do-not-leak") {
		t.Fatalf("unsafe object problem leaked payload: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/organizations/"+orgID+"/objects/readme.txt", nil)
	authorizeTestRequestAs(t, req, orgID, "owner_1", "objects:write")
	req.Header.Set("Idempotency-Key", "delete-object")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete object status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOpenAPIGolden(t *testing.T) {
	got, err := OpenAPIDocument()
	if err != nil {
		t.Fatalf("render openapi: %v", err)
	}
	goldenPath := filepath.Join("..", "..", "testdata", "openapi.golden.json")
	if os.Getenv("UPDATE_OPENAPI") == "1" {
		if err := os.WriteFile(goldenPath, got, 0o600); err != nil {
			t.Fatalf("update golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("openapi golden drift\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func createWidget(t *testing.T, handler http.Handler, idem string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader("{\"name\":\"alpha\"}"))
	authorizeTestRequest(t, req, "org_1")
	req.Header.Set("Idempotency-Key", idem)
	handler.ServeHTTP(rec, req)
	return rec
}

func createOrganization(t *testing.T, handler http.Handler, actorID, name string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/organizations", strings.NewReader("{\"name\":\""+name+"\"}"))
	authorizeTestRequestAs(t, req, "", actorID, "organizations:write")
	req.Header.Set("Idempotency-Key", "create-org-"+actorID)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create organization status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode organization body: %v", err)
	}
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatalf("organization body missing id: %#v", body)
	}
	return id
}

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	handler, _ := newTestRouterWithAudit(t)
	return handler
}

func newTestRouterWithAudit(t *testing.T) (http.Handler, *app.AuditService) {
	t.Helper()
	configureTestAuthEnv(t)
	tenancy := app.NewTenancyService()
	auditLog := app.NewAuditService()
	handler := NewRouter(RouterConfig{Widgets: app.NewWidgetService(), Tenancy: tenancy, APIKeys: app.NewAPIKeyService("test-pepper", tenancy), Audit: auditLog, APIKey: "test-key"})
	return handler, auditLog
}

func newTestRouterWithMetrics(t *testing.T) (http.Handler, http.Handler) {
	t.Helper()
	configureTestAuthEnv(t)
	recorder, err := metricsmw.NewPrometheusRecorderChecked(nil, nil)
	if err != nil {
		t.Fatalf("new metrics recorder: %v", err)
	}
	middleware, err := NewMetricsMiddleware(recorder)
	if err != nil {
		t.Fatalf("new metrics middleware: %v", err)
	}
	tenancy := app.NewTenancyService()
	cfg := RouterConfig{
		Widgets:        app.NewWidgetService(),
		Tenancy:        tenancy,
		APIKeys:        app.NewAPIKeyService("test-pepper", tenancy),
		Audit:          app.NewAuditService(),
		Metrics:        middleware,
		MetricsHandler: metricsmw.PrometheusHandler(),
		AdminKey:       "test-admin-key",
		APIKey:         "test-key",
	}
	return NewRouter(cfg), NewAdminRouter(cfg)
}

func configureTestAuthEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ENV", "test")
	t.Setenv("API_ACTOR_ID", "")
}

func authorizeTestRequest(t *testing.T, req *http.Request, tenantID string) {
	t.Helper()
	authorizeTestRequestAs(t, req, tenantID, "user_123", "widgets:write")
}

func authorizeTestRequestAs(t *testing.T, req *http.Request, tenantID, actorID string, scopes ...string) {
	t.Helper()
	if len(scopes) == 0 {
		scopes = []string{"widgets:write"}
	}
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	if actorID != "" {
		req.Header.Set("X-Actor-ID", actorID)
	}
	req.Header.Set("X-API-Key", "test-key")
}
