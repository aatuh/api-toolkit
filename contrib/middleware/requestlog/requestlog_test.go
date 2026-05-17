package requestlog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/aatuh/api-toolkit/contrib/v3/webhookdelivery"
	idempotencymw "github.com/aatuh/api-toolkit/v3/middleware/idempotency"
	timeoutmw "github.com/aatuh/api-toolkit/v3/middleware/timeout"
	"github.com/aatuh/api-toolkit/v3/routecontracts"
	"github.com/aatuh/api-toolkit/v3/routepolicy"
	"github.com/aatuh/api-toolkit/v3/specs"
)

type captureLogger struct {
	level string
	msg   string
	kv    []any
}

func (l *captureLogger) Debug(string, ...any) {}
func (l *captureLogger) Warn(msg string, kv ...any) {
	l.level = "warn"
	l.msg = msg
	l.kv = append([]any(nil), kv...)
}

func (l *captureLogger) Error(msg string, kv ...any) {
	l.level = "error"
	l.msg = msg
	l.kv = append([]any(nil), kv...)
}

func (l *captureLogger) Info(msg string, kv ...any) {
	l.level = "info"
	l.msg = msg
	l.kv = append([]any(nil), kv...)
}

func TestRequestLogRedactionDefaults(t *testing.T) {
	log := &captureLogger{}
	mw, err := New(log,
		WithRoutePattern(func(*http.Request) string { return "/demo" }),
		WithRequestHeaders(),
		WithResponseHeaders(),
		WithRedactedHeaders("X-Extra"),
	)
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "sid=secret")
		w.Header().Set("X-Resp", "ok")
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/demo", nil)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Cookie", "a=b")
	req.Header.Set("X-API-Key", "abc123")
	req.Header.Set("X-Access-Token", "ok-token")
	req.Header.Set("X_Access_Token", "underscore-access-token")
	req.Header.Set("x-auth-token", "abc")
	req.Header.Set("X_API_TOKEN", "underscore-token")
	req.Header.Set("Authorization-Token", "prefix-thing")
	req.Header.Set("X-Session-Id", "abc")
	req.Header.Set("X-Session-Token", "abc")
	req.Header.Set("X-Secret", "abc")
	req.Header.Set("X-Password", "abc")
	req.Header.Set("X-Refresh-Token", "abc")
	req.Header.Set("X-Extra", "custom-hidden")
	req.Header.Set("X-Request-ID", "req-123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fields := kvToMap(log.kv)
	reqHeaders, ok := fields[FieldRequestHeaders].(map[string]string)
	if !ok {
		t.Fatalf("expected request headers in log")
	}
	variants := []string{
		"Authorization",
		"Cookie",
		"X-Api-Key",
		"X-Access-Token",
		"X-Auth-Token",
		"X_Access_Token",
		"X_API_TOKEN",
		"Authorization-Token",
		"X-Session-Id",
		"X-Session-Token",
		"X-Secret",
		"X-Password",
		"X-Refresh-Token",
		"X-Extra",
	}
	if reqHeaders["Authorization"] != redactedValue {
		t.Fatalf("authorization not redacted: %q", reqHeaders["Authorization"])
	}
	for _, h := range variants {
		if reqHeaders[http.CanonicalHeaderKey(h)] != redactedValue {
			t.Fatalf("sensitive header not redacted: %s=%q", h, reqHeaders[http.CanonicalHeaderKey(h)])
		}
	}
	if reqHeaders["X-Extra"] != redactedValue {
		t.Fatalf("custom redacted header not redacted: %q", reqHeaders["X-Extra"])
	}
	requestIDKey := http.CanonicalHeaderKey("X-Request-ID")
	if reqHeaders[requestIDKey] != "req-123" {
		t.Fatalf("unexpected request id in headers: %q", reqHeaders[requestIDKey])
	}
	if reqHeaders["Cookie"] != redactedValue {
		t.Fatalf("cookie not redacted: %q", reqHeaders["Cookie"])
	}
	if reqHeaders[requestIDKey] != "req-123" {
		t.Fatalf("request id changed unexpectedly: %q", reqHeaders[requestIDKey])
	}

	respHeaders, ok := fields[FieldResponseHeaders].(map[string]string)
	if !ok {
		t.Fatalf("expected response headers in log")
	}
	if respHeaders["Set-Cookie"] != redactedValue {
		t.Fatalf("set-cookie not redacted: %q", respHeaders["Set-Cookie"])
	}
	if respHeaders["X-Resp"] != "ok" {
		t.Fatalf("unexpected response header: %q", respHeaders["X-Resp"])
	}
}

func TestRequestLogPayloadRedactionHelpers(t *testing.T) {
	input := map[string]any{
		"Token":            "abc123",
		"my_secret":        "shh",
		"password":         "hunter2",
		"RequestID":        "id-123",
		"Auth":             "bearer token",
		"password_hash":    "abc",
		"x_api_key":        "xyz",
		"Correlation-ID":   "corr-1",
		"nested_secret_id": "id",
	}
	redacted := RedactPayloadFields(input)
	if redacted["Token"] != redactedValue {
		t.Fatalf("token not redacted: %q", redacted["Token"])
	}
	if redacted["my_secret"] != redactedValue {
		t.Fatalf("secret not redacted: %q", redacted["my_secret"])
	}
	if redacted["password"] != redactedValue {
		t.Fatalf("password not redacted: %q", redacted["password"])
	}
	if redacted["password_hash"] != redactedValue {
		t.Fatalf("password hash not redacted: %q", redacted["password_hash"])
	}
	if redacted["RequestID"] != "id-123" {
		t.Fatalf("non-sensitive field changed: %q", redacted["RequestID"])
	}
	if redacted["Correlation-ID"] != "corr-1" {
		t.Fatalf("non-sensitive header changed: %q", redacted["Correlation-ID"])
	}
	if redacted["x_api_key"] != redactedValue {
		t.Fatal("api key not redacted")
	}
	if redacted["Auth"] != redactedValue {
		t.Fatal("auth not redacted")
	}
	if redacted["nested_secret_id"] != redactedValue {
		t.Fatal("suffixed secret not redacted")
	}
}

func TestRequestLogIncludesBoundedRoutePolicyLabels(t *testing.T) {
	log := &captureLogger{}
	mw, err := New(log)
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	router := chi.NewRouter()
	router.Use(mw.Middleware())
	registry := routecontracts.NewRegistry(router, nil)
	operation := routepolicy.ApplyMetadata(specs.Operation{Method: http.MethodPost, Path: "/widgets"},
		routepolicy.WithAuth("ApiKeyAuth", "widgets:write"),
		routepolicy.WithTenantRequired("header"),
		routepolicy.WithIdempotencyRequired(),
		routepolicy.WithRateLimit("tenant-specific-write-standard"),
		routepolicy.WithAdminPolicy("platform-admins"),
		routepolicy.WithDeprecated(),
	)
	if err := registry.Post("/widgets", operation, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})); err != nil {
		t.Fatalf("register route: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/widgets", nil))

	fields := kvToMap(log.kv)
	for key, want := range map[string]any{
		FieldPolicyAuth:        "required",
		FieldPolicyTenant:      "required",
		FieldPolicyIdempotency: "required",
		FieldPolicyRateLimit:   "configured",
		FieldPolicyAdmin:       "required",
		FieldPolicyDeprecated:  "true",
	} {
		if got := fields[key]; got != want {
			t.Fatalf("field %s = %#v, want %#v; fields=%#v", key, got, want, fields)
		}
	}
	if fields[FieldPolicyRateLimit] == "tenant-specific-write-standard" || fields[FieldPolicyAdmin] == "platform-admins" {
		t.Fatalf("policy log leaked raw policy values: %#v", fields)
	}
}

func TestIdempotencyOutcomeLogHookIncludesBoundedFields(t *testing.T) {
	log := &captureLogger{}
	hook := IdempotencyOutcomeLogHook(log)

	hook(context.Background(), idempotencymw.OutcomeEvent{
		Method:    "BREW",
		Status:    http.StatusCreated,
		StoreType: "customer-acme-memory-primary",
		Outcome:   idempotencymw.OutcomeEventName("customer-acme-outcome"),
		FailOpen:  true,
	})

	if log.level != "info" || log.msg != "idempotency outcome" {
		t.Fatalf("log entry = %s %q", log.level, log.msg)
	}
	fields := kvToMap(log.kv)
	for key, want := range map[string]any{
		FieldIdempotencyMethod:      "OTHER",
		FieldIdempotencyStoreClass:  "memory",
		FieldIdempotencyOutcome:     "unknown",
		FieldIdempotencyStatusClass: "2xx",
		FieldIdempotencyFailOpen:    true,
	} {
		if got := fields[key]; got != want {
			t.Fatalf("field %s = %#v, want %#v; fields=%#v", key, got, want, fields)
		}
	}
	if fields[FieldIdempotencyStoreClass] == "customer-acme-memory-primary" || fields[FieldIdempotencyOutcome] == "customer-acme-outcome" {
		t.Fatalf("idempotency outcome log leaked raw values: %#v", fields)
	}
}

func TestHardTimeoutEventLogHookIncludesBoundedFields(t *testing.T) {
	log := &captureLogger{}
	hook := HardTimeoutEventLogHook(log)

	hook(timeoutmw.HardTimeoutEvent{
		Method:          "BREW",
		Status:          http.StatusGatewayTimeout,
		Outcome:         timeoutmw.HardTimeoutOutcome("customer-acme-timeout"),
		TimedOut:        true,
		Panicked:        true,
		CaptureOverflow: true,
	})

	if log.level != "error" || log.msg != "hard timeout event" {
		t.Fatalf("log entry = %s %q", log.level, log.msg)
	}
	fields := kvToMap(log.kv)
	for key, want := range map[string]any{
		FieldHardTimeoutMethod:          "OTHER",
		FieldHardTimeoutOutcome:         "unknown",
		FieldHardTimeoutStatusClass:     "5xx",
		FieldHardTimeoutTimedOut:        true,
		FieldHardTimeoutPanicked:        true,
		FieldHardTimeoutCaptureOverflow: true,
	} {
		if got := fields[key]; got != want {
			t.Fatalf("field %s = %#v, want %#v; fields=%#v", key, got, want, fields)
		}
	}
	if fields[FieldHardTimeoutOutcome] == "customer-acme-timeout" {
		t.Fatalf("hard-timeout log leaked raw outcome value: %#v", fields)
	}
}

func TestWebhookDeliveryLogHookIncludesBoundedFields(t *testing.T) {
	log := &captureLogger{}
	hook := WebhookDeliveryLogHook(log)

	hook.ObserveWebhookDelivery(context.Background(), webhookdelivery.DeliveryObservation{
		EventType:   "Customer Acme Widget.Created",
		Outcome:     "transport_error",
		StatusClass: "5xx",
	})

	if log.level != "warn" || log.msg != "webhook delivery event" {
		t.Fatalf("log entry = %s %q", log.level, log.msg)
	}
	fields := kvToMap(log.kv)
	for key, want := range map[string]any{
		FieldWebhookDeliveryEventType:   "customer_acme_widget.created",
		FieldWebhookDeliveryOutcome:     "transport_error",
		FieldWebhookDeliveryStatusClass: "5xx",
	} {
		if got := fields[key]; got != want {
			t.Fatalf("field %s = %#v, want %#v; fields=%#v", key, got, want, fields)
		}
	}
	for _, forbidden := range []string{"tenant", "https://", "secret", "payload"} {
		for _, value := range fields {
			if s, ok := value.(string); ok && strings.Contains(s, forbidden) {
				t.Fatalf("webhook delivery log leaked %q in fields %#v", forbidden, fields)
			}
		}
	}
}

func TestRequestLogDeepPayloadRedactionHelpers(t *testing.T) {
	input := map[string]any{
		"request_id": "id-123",
		"user": map[string]any{
			"password": "secret",
			"profile": map[string]any{
				"refresh_token": "refresh",
				"details": map[string]any{
					"token": "abc",
					"safe":  true,
				},
			},
		},
		"tokens": []any{
			map[string]any{
				"kind":  "session",
				"value": "abc",
			},
			map[string]any{
				"api_token": "xyz",
				"nested": map[string]any{
					"session_id": "s-1",
					"meta": map[string]any{
						"keep":   "visible",
						"secret": "hidden",
					},
				},
			},
		},
	}
	redacted := RedactPayloadFieldsDeep(input)
	if got := redacted["request_id"]; got != "id-123" {
		t.Fatalf("non-sensitive field changed: %#v", got)
	}

	user, ok := redacted["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user map, got %T", redacted["user"])
	}
	if user["password"] != redactedValue {
		t.Fatalf("expected nested password redacted, got %#v", user["password"])
	}
	profile, ok := user["profile"].(map[string]any)
	if !ok {
		t.Fatalf("expected user.profile map, got %T", user["profile"])
	}
	if profile["refresh_token"] != redactedValue {
		t.Fatalf("expected nested refresh_token redacted, got %#v", profile["refresh_token"])
	}
	details, ok := profile["details"].(map[string]any)
	if !ok {
		t.Fatalf("expected user.profile.details map, got %T", profile["details"])
	}
	if details["token"] != redactedValue {
		t.Fatalf("expected nested details.token redacted, got %#v", details["token"])
	}
	if got := details["safe"]; got != true {
		t.Fatalf("expected non-sensitive nested field to remain, got %#v", got)
	}

	tokens, ok := redacted["tokens"].([]any)
	if !ok {
		t.Fatalf("expected tokens slice, got %T", redacted["tokens"])
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	second, ok := tokens[1].(map[string]any)
	if !ok {
		t.Fatalf("expected second token to be map, got %T", tokens[1])
	}
	if second["api_token"] != redactedValue {
		t.Fatalf("expected nested api_token redacted, got %#v", second["api_token"])
	}
	secondNested, ok := second["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested token fields map, got %T", second["nested"])
	}
	if secondNested["session_id"] != redactedValue {
		t.Fatalf("expected session_id redacted, got %#v", secondNested["session_id"])
	}
	meta, ok := secondNested["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta map, got %T", secondNested["meta"])
	}
	if meta["secret"] != redactedValue {
		t.Fatalf("expected nested secret redacted, got %#v", meta["secret"])
	}
	if meta["keep"] != "visible" {
		t.Fatalf("expected non-sensitive nested field preserved, got %#v", meta["keep"])
	}
}

func TestRequestLogDeepPayloadRedactionSupportsTypedShapes(t *testing.T) {
	input := map[string]any{
		"profile": map[string]string{
			"request_id": "r1",
			"password":   "secret",
			"role":       "admin",
		},
		"tokens": []map[string]string{
			{
				"session_id": "session-1",
				"label":      "primary",
			},
			{
				"kind":      "api",
				"api_token": "token-123",
			},
		},
	}

	redacted := RedactPayloadFieldsDeep(input)

	profile, ok := redacted["profile"].(map[string]any)
	if !ok {
		t.Fatalf("expected profile map, got %T", redacted["profile"])
	}
	if profile["password"] != redactedValue {
		t.Fatalf("expected typed map nested password redacted, got %#v", profile["password"])
	}
	if profile["request_id"] != "r1" {
		t.Fatalf("expected non-sensitive nested typed map field to remain, got %#v", profile["request_id"])
	}

	tokens, ok := redacted["tokens"].([]any)
	if !ok {
		t.Fatalf("expected tokens slice, got %T", redacted["tokens"])
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 token entries, got %d", len(tokens))
	}
	if first, ok := tokens[0].(map[string]any); !ok {
		t.Fatalf("expected token map at first index, got %T", tokens[0])
	} else if got := first["session_id"]; got != redactedValue {
		t.Fatalf("expected typed slice first session_id redacted, got %#v", got)
	}
	if second, ok := tokens[1].(map[string]any); !ok {
		t.Fatalf("expected token map at second index, got %T", tokens[1])
	} else if got := second["api_token"]; got != redactedValue {
		t.Fatalf("expected typed slice nested api_token redacted, got %#v", got)
	} else if got := second["kind"]; got != "api" {
		t.Fatalf("expected non-sensitive nested token field to remain, got %#v", got)
	}
}

func TestRequestLogFieldConventions(t *testing.T) {
	log := &captureLogger{}
	mw, err := New(log, WithRoutePattern(func(*http.Request) string { return "/test" }))
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	traceID := oteltrace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	spanID := oteltrace.SpanID{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}
	sc := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: oteltrace.FlagsSampled,
	})

	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/test", nil)
	req.Header.Set("X-Request-ID", "req-xyz")
	req = req.WithContext(oteltrace.ContextWithSpanContext(req.Context(), sc))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fields := kvToMap(log.kv)
	if fields[FieldRequestID] != "req-xyz" {
		t.Fatalf("request_id = %v", fields[FieldRequestID])
	}
	if fields[FieldTraceID] != traceID.String() {
		t.Fatalf("trace_id = %v", fields[FieldTraceID])
	}
	if fields[FieldSpanID] != spanID.String() {
		t.Fatalf("span_id = %v", fields[FieldSpanID])
	}
	if fields[FieldRoute] != "/test" {
		t.Fatalf("route = %v", fields[FieldRoute])
	}
	if fields[FieldStatus] != http.StatusOK {
		t.Fatalf("status = %v", fields[FieldStatus])
	}
	if _, ok := fields[FieldLatencyMS]; !ok {
		t.Fatalf("missing latency field")
	}
}

func TestRequestLogDefaultsClock(t *testing.T) {
	mw, err := New(nil)
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	if mw.clock == nil {
		t.Fatal("expected default clock")
	}
}

func TestRequestLogDoesNotAttachStackForHandled5xxByDefault(t *testing.T) {
	log := &captureLogger{}
	mw, err := New(log, WithRoutePattern(func(*http.Request) string { return "/boom" }))
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/boom", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if log.level != "error" {
		t.Fatalf("level = %q", log.level)
	}
	fields := kvToMap(log.kv)
	if _, ok := fields[FieldStack]; ok {
		t.Fatal("did not expect stack field")
	}
}

func TestRequestLogCanAttachStackForHandled5xxWhenEnabled(t *testing.T) {
	log := &captureLogger{}
	mw, err := New(log,
		WithRoutePattern(func(*http.Request) string { return "/boom" }),
		With5xxStackLogging(true),
	)
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/boom", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fields := kvToMap(log.kv)
	if _, ok := fields[FieldStack]; !ok {
		t.Fatal("expected stack field")
	}
}

func TestRequestLogEmitsErrorForRecoveredPanicBeforeCommit(t *testing.T) {
	log := &captureLogger{}
	mw, err := New(log, WithRoutePattern(func(*http.Request) string { return "/panic" }))
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Middleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/panic", nil)
	defer func() {
		if got := recover(); got != "boom" {
			t.Fatalf("expected panic boom, got %v", got)
		}
		if log.level != "error" {
			t.Fatalf("level = %q", log.level)
		}
		fields := kvToMap(log.kv)
		if fields[FieldStatus] != http.StatusInternalServerError {
			t.Fatalf("status = %v", fields[FieldStatus])
		}
		if fields[FieldPanicRecovered] != true {
			t.Fatalf("panic_recovered = %v", fields[FieldPanicRecovered])
		}
	}()
	handler.ServeHTTP(rec, req)
}

func TestRequestLogEmitsErrorForRecoveredPanicAfterCommit(t *testing.T) {
	log := &captureLogger{}
	mw, err := New(log,
		WithRoutePattern(func(*http.Request) string { return "/panic-after-commit" }),
		With5xxStackLogging(true),
	)
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
		panic("boom-after-commit")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/panic-after-commit", nil)
	defer func() {
		if got := recover(); got != "boom-after-commit" {
			t.Fatalf("expected panic boom-after-commit, got %v", got)
		}
		if log.level != "error" {
			t.Fatalf("level = %q", log.level)
		}
		fields := kvToMap(log.kv)
		if fields[FieldStatus] != http.StatusInternalServerError {
			t.Fatalf("status = %v", fields[FieldStatus])
		}
		if fields[FieldPanicRecovered] != true {
			t.Fatalf("panic_recovered = %v", fields[FieldPanicRecovered])
		}
		if fields[FieldCommittedStatus] != http.StatusCreated {
			t.Fatalf("committed_status = %v", fields[FieldCommittedStatus])
		}
		if _, ok := fields[FieldStack]; !ok {
			t.Fatal("expected stack field for recovered panic when 5xx stack logging is enabled")
		}
	}()
	handler.ServeHTTP(rec, req)
}

func kvToMap(kv []any) map[string]any {
	out := make(map[string]any)
	for i := 0; i+1 < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok || key == "" {
			continue
		}
		out[key] = kv[i+1]
	}
	return out
}
