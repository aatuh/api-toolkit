package chi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	gchi "github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/aatuh/api-toolkit/v3/middleware/auth/authz"
)

type captureLogEntry struct {
	msg  string
	args []any
}

type captureLogger struct {
	mu     sync.Mutex
	debugs []captureLogEntry
}

func (l *captureLogger) Debug(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.debugs = append(l.debugs, captureLogEntry{msg: msg, args: append([]any(nil), args...)})
}

func (l *captureLogger) Info(string, ...any)  {}
func (l *captureLogger) Warn(string, ...any)  {}
func (l *captureLogger) Error(string, ...any) {}

func TestNewRoutesAndExtractors(t *testing.T) {
	router := New().(*ChiRouter)
	extractor := NewURLParamExtractor()

	router.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		if got := extractor.URLParam(r, "id"); got != "42" {
			t.Fatalf("extractor URLParam() = %q, want 42", got)
		}
		if got := URLParam(r, "id"); got != "42" {
			t.Fatalf("package URLParam() = %q, want 42", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/users/42", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestNewMiddlewareAppliesRequestIDAndRealIP(t *testing.T) {
	mw := NewMiddleware()
	var (
		seenRequestID string
		seenRemoteIP  string
	)

	handler := mw.RequestID()(mw.RealIP()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenRequestID = chimiddleware.GetReqID(r.Context())
		seenRemoteIP = r.RemoteAddr
		w.WriteHeader(http.StatusNoContent)
	})))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.50:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.10")

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if seenRequestID == "" {
		t.Fatal("expected request ID to be populated")
	}
	if seenRemoteIP != "203.0.113.10" {
		t.Fatalf("remote IP = %q, want 203.0.113.10", seenRemoteIP)
	}
}

func TestNewMiddlewareRecoverer(t *testing.T) {
	mw := NewMiddleware()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)

	mw.Recoverer()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestValidateRequireRoleMiddlewareRoutesFromChiSupportsAnyAndSpecificMethods(t *testing.T) {
	adminMw, err := authz.NewRequireRoleMiddlewareChecked("admin", func(_ context.Context) []string { return []string{"admin"} })
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	opsMw, err := authz.NewRequireRoleMiddlewareChecked("ops", func(_ context.Context) []string { return []string{"ops"} })
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	router := New().(*ChiRouter)
	router.Get("/admin", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	router.Method(http.MethodPost, "/billing", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	router.Handle("/wide", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	err = ValidateRequireRoleMiddlewareRoutes(router.Mux, func(method, route string, _ http.Handler) *authz.RequireRoleMiddleware {
		switch route {
		case "/admin":
			return adminMw
		case "/billing":
			return adminMw
		case "/wide":
			if method == "ANY" {
				return opsMw
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected validation to pass: %v", err)
	}
}

func TestValidateRequireRoleMiddlewareRoutesFromChiFailsForMalformedAndMissingCoverage(t *testing.T) {
	malformedMw := &authz.RequireRoleMiddleware{}
	router := New().(*ChiRouter)
	router.Method(http.MethodGet, "/billing", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	router.Handle("/wide", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	err := ValidateRequireRoleMiddlewareRoutesStrict(router.Mux, func(method, route string, _ http.Handler) *authz.RequireRoleMiddleware {
		if route == "/billing" && method == http.MethodGet {
			return malformedMw
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "GET /billing") {
		t.Fatalf("expected malformed route context, got %v", err)
	}
	if !strings.Contains(err.Error(), "ANY /wide") {
		t.Fatalf("expected missing route context for ANY, got %v", err)
	}
	if !strings.Contains(err.Error(), "middleware is nil") {
		t.Fatalf("expected missing middleware context, got %v", err)
	}
}

func TestValidateRequireRoleMiddlewareRoutesFromChiSkipsPublicRoutesByDefault(t *testing.T) {
	adminMw, err := authz.NewRequireRoleMiddlewareChecked("admin", func(_ context.Context) []string { return []string{"admin"} })
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	router := New().(*ChiRouter)
	router.Get("/public", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	router.Method(http.MethodPost, "/billing", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	router.Handle("/wide", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	err = ValidateRequireRoleMiddlewareRoutes(router.Mux, func(method, route string, _ http.Handler) *authz.RequireRoleMiddleware {
		if route == "/billing" && method == http.MethodPost {
			return adminMw
		}
		if route == "/wide" {
			return adminMw
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected public route skip semantics to pass mixed scan: %v", err)
	}
}

func TestValidateRequireRoleMiddlewareRoutesFromChiSupportsExplicitSkipForMixedRoutes(t *testing.T) {
	adminMw, err := authz.NewRequireRoleMiddlewareChecked("admin", func(_ context.Context) []string { return []string{"admin"} })
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	router := New().(*ChiRouter)
	router.Get("/public", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	router.Method(http.MethodGet, "/protected", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	err = ValidateRequireRoleMiddlewareRoutesWithCoverage(router.Mux, func(method, route string, _ http.Handler) MiddlewareSpecResolution {
		switch route {
		case "/public":
			return MiddlewareSpecResolution{SkipFromValidation: true}
		case "/protected":
			if method == http.MethodGet {
				return MiddlewareSpecResolution{Middleware: adminMw}
			}
		}
		return MiddlewareSpecResolution{Middleware: nil, SkipFromValidation: false}
	})
	if err != nil {
		t.Fatalf("expected explicit skip and protected route validation to pass: %v", err)
	}
}

func TestValidateRequireRoleMiddlewareRoutesFromChiSupportsExplicitSkipAndFailsForProtectedMissingCoverage(t *testing.T) {
	router := New().(*ChiRouter)
	router.Get("/public", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	router.Method(http.MethodGet, "/protected", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	err := ValidateRequireRoleMiddlewareRoutesWithCoverage(router.Mux, func(method, route string, _ http.Handler) MiddlewareSpecResolution {
		switch route {
		case "/public":
			return MiddlewareSpecResolution{SkipFromValidation: true}
		case "/protected":
			if method != http.MethodGet {
				return MiddlewareSpecResolution{}
			}
		}
		return MiddlewareSpecResolution{}
	})
	if err == nil {
		t.Fatal("expected validation to fail when protected route has no middleware")
	}
	if !strings.Contains(err.Error(), "GET /protected") {
		t.Fatalf("expected protected route context in error, got %v", err)
	}
}

func TestValidateRequireRoleMiddlewareRoutesAutoDefaultsToPermissiveInNonStrictEnvs(t *testing.T) {
	adminMw, err := authz.NewRequireRoleMiddlewareChecked("admin", func(_ context.Context) []string { return []string{"admin"} })
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("ENV", "development")
	t.Setenv("API_TOOLKIT_AUTHZ_BOOTSTRAP_STRICT", "")

	router := New().(*ChiRouter)
	router.Get("/admin", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	router.Get("/billing", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	err = ValidateRequireRoleMiddlewareRoutesAuto(router.Mux, func(method, route string, _ http.Handler) *authz.RequireRoleMiddleware {
		if route == "/admin" && method == http.MethodGet {
			return adminMw
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected permissive default to skip missing coverage for mixed routes: %v", err)
	}
}

func TestValidateRequireRoleMiddlewareRoutesAutoIsStrictInCIMode(t *testing.T) {
	adminMw, err := authz.NewRequireRoleMiddlewareChecked("admin", func(_ context.Context) []string { return []string{"admin"} })
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	t.Setenv("CI", "true")
	t.Setenv("API_TOOLKIT_AUTHZ_BOOTSTRAP_STRICT", "")

	router := New().(*ChiRouter)
	router.Get("/admin", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	router.Get("/billing", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	err = ValidateRequireRoleMiddlewareRoutesAuto(router.Mux, func(method, route string, _ http.Handler) *authz.RequireRoleMiddleware {
		if route == "/admin" && method == http.MethodGet {
			return adminMw
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected strict CI bootstrap policy to fail on missing protected coverage")
	}
	if !strings.Contains(err.Error(), "GET /billing") {
		t.Fatalf("expected mixed-route strict failure to retain route context, got %v", err)
	}
}

func TestValidateRequireRoleMiddlewareRoutesAutoHonorsStrictAndPermissiveOverrides(t *testing.T) {
	adminMw, err := authz.NewRequireRoleMiddlewareChecked("admin", func(_ context.Context) []string { return []string{"admin"} })
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	t.Setenv("CI", "true")
	t.Setenv("API_TOOLKIT_AUTHZ_BOOTSTRAP_STRICT", "strict")

	router := New().(*ChiRouter)
	router.Get("/admin", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	router.Get("/billing", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	err = ValidateRequireRoleMiddlewareRoutesAuto(router.Mux, func(method, route string, _ http.Handler) *authz.RequireRoleMiddleware {
		if route == "/admin" && method == http.MethodGet {
			return adminMw
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected strict override to fail in CI when coverage is missing")
	}

	t.Setenv("API_TOOLKIT_AUTHZ_BOOTSTRAP_STRICT", "permissive")
	err = ValidateRequireRoleMiddlewareRoutesAuto(router.Mux, func(method, route string, _ http.Handler) *authz.RequireRoleMiddleware {
		if route == "/admin" && method == http.MethodGet {
			return adminMw
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected permissive override to keep mixed-route behavior: %v", err)
	}
}

func TestValidateRequireRoleMiddlewareRoutesWithCoverageLogsValidationIntent(t *testing.T) {
	adminMw, err := authz.NewRequireRoleMiddlewareChecked("admin", func(_ context.Context) []string { return []string{"admin"} })
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	router := New().(*ChiRouter)
	router.Get("/public", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	router.Get("/admin", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	router.Handle("/wide", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	logger := &captureLogger{}
	err = ValidateRequireRoleMiddlewareRoutesWithCoverageAndLogger(router.Mux, func(method, route string, _ http.Handler) MiddlewareSpecResolution {
		switch route {
		case "/public":
			return MiddlewareSpecResolution{SkipFromValidation: true}
		case "/admin":
			if method == http.MethodGet {
				return MiddlewareSpecResolution{Middleware: adminMw}
			}
		}
		return MiddlewareSpecResolution{}
	}, logger)
	if err == nil {
		t.Fatal("expected mixed protected route to fail when explicit coverage is missing")
	}
	if !strings.Contains(err.Error(), "ANY /wide") {
		t.Fatalf("expected missing route in error context, got %v", err)
	}

	seenIntents := map[string]bool{}
	for _, debug := range logger.debugs {
		if debug.msg != "authz route validation intent" {
			continue
		}
		for i := 0; i+1 < len(debug.args); i += 2 {
			key, ok := debug.args[i].(string)
			if !ok {
				continue
			}
			if key == "intent" {
				if intent, ok := debug.args[i+1].(string); ok {
					seenIntents[intent] = true
				}
			}
		}
	}
	if !seenIntents["skip"] || !seenIntents["validate:missing"] {
		t.Fatalf("expected startup intent log for skip and missing-validate: %#v", seenIntents)
	}
}

func TestValidateRequireRoleMiddlewareRoutesFromChiPreservesNestedAndParameterizedPatterns(t *testing.T) {
	router := New().(*ChiRouter)
	router.Route("/api", func(r gchi.Router) {
		r.Route("/v1", func(r gchi.Router) {
			r.Get("/{tenant}/accounts/{id}", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
			r.Method(http.MethodPost, "/{tenant}/accounts/{id}", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		})
		r.Route("/", func(r gchi.Router) {
			r.Get("/", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		})
	})
	router.Get("/wildcard/{id}/detail/*", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	router.Get("/slash/", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	seen := map[string]bool{}
	err := ValidateRequireRoleMiddlewareRoutesWithCoverage(router.Mux, func(_, route string, _ http.Handler) MiddlewareSpecResolution {
		seen[route] = true
		return MiddlewareSpecResolution{SkipFromValidation: true}
	})
	if err != nil {
		t.Fatalf("expected mixed route patterns to pass with exact route assembly: %v", err)
	}
	if !seen["/api/v1/{tenant}/accounts/{id}"] {
		t.Fatalf("expected nested parameterized route to be reported")
	}
	if !seen["/wildcard/{id}/detail/*"] {
		t.Fatalf("expected wildcard route to be reported")
	}
	if !seen["/api/"] {
		t.Fatalf("expected nested mount root route to be preserved")
	}
	if !seen["/slash/"] {
		t.Fatalf("expected slash-root route form to be preserved")
	}
}
