package devheaders

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	jwtauth "github.com/aatuh/api-toolkit/v2/middleware/auth/jwt"
)

func TestConfigAndMiddlewareRemainComparable(t *testing.T) {
	if !reflect.TypeOf(Config{}).Comparable() {
		t.Fatal("Config should remain comparable for v2 source compatibility")
	}
	if !reflect.TypeOf(Middleware{}).Comparable() {
		t.Fatal("Middleware should remain comparable for v2 source compatibility")
	}
}

func TestNewRequiresUserHeaderWhenEnabled(t *testing.T) {
	if _, err := New(Config{Enabled: true}, nil); err == nil {
		t.Fatal("expected error for missing user header")
	}
}

func TestNewRequiresExplicitDangerousBypassOptInWhenEnabled(t *testing.T) {
	_, err := New(Config{
		Enabled:        true,
		UserIDHeader:   "X-Debug-User",
		TrustedProxies: "127.0.0.1/32",
	}, nil)
	if err == nil {
		t.Fatal("expected error for missing dangerous bypass opt-in")
	}
	if got, want := err.Error(), "dangerous dev bypasses must be explicitly allowed when dev auth is enabled"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestNewRequiresTrustedProxiesWhenEnabled(t *testing.T) {
	_, err := New(Config{
		Enabled:                   true,
		UserIDHeader:              "X-Debug-User",
		AllowDangerousDevBypasses: true,
	}, nil)
	if err == nil {
		t.Fatal("expected error for missing trusted proxies")
	}
	if got, want := err.Error(), "trusted proxies are required when dev auth is enabled"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestHandlerRejectsUntrustedRemote(t *testing.T) {
	mw, err := New(Config{
		Enabled:                   true,
		UserIDHeader:              "X-Debug-User",
		AllowDangerousDevBypasses: true,
		TrustedProxies:            "127.0.0.1/32",
	}, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.10:1234"
	req.Header.Set("X-Debug-User", "user-1")
	rr := httptest.NewRecorder()

	called := false
	mw.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	})).ServeHTTP(rr, req)

	if called {
		t.Fatal("expected next handler not to be called")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestHandlerAttachesSubjectForTrustedRemote(t *testing.T) {
	mw, err := New(Config{
		Enabled:                   true,
		UserIDHeader:              "X-Debug-User",
		EmailHeader:               "X-Debug-Email",
		FirstNameHeader:           "X-Debug-First-Name",
		LastNameHeader:            "X-Debug-Last-Name",
		DefaultLanguage:           "en",
		AllowDangerousDevBypasses: true,
		TrustedProxies:            "127.0.0.1/32",
	}, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Debug-User", "user-1")
	req.Header.Set("X-Debug-Email", "user@example.com")
	req.Header.Set("X-Debug-First-Name", "Dev")
	req.Header.Set("X-Debug-Last-Name", "User")
	rr := httptest.NewRecorder()

	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subj, ok := jwtauth.SubjectFromContext(r.Context())
		if !ok {
			t.Fatal("expected subject in context")
		}
		if subj.UserID != "user-1" {
			t.Fatalf("UserID = %q, want %q", subj.UserID, "user-1")
		}
		if subj.Email != "user@example.com" {
			t.Fatalf("Email = %q, want %q", subj.Email, "user@example.com")
		}
		if subj.First != "Dev" || subj.Last != "User" {
			t.Fatalf("name = %q %q, want %q %q", subj.First, subj.Last, "Dev", "User")
		}
		if subj.Language != "en" {
			t.Fatalf("Language = %q, want %q", subj.Language, "en")
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestOptionalHandlerRejectsUntrustedDebugHeaders(t *testing.T) {
	mw, err := New(Config{
		Enabled:                   true,
		UserIDHeader:              "X-Debug-User",
		AllowDangerousDevBypasses: true,
		TrustedProxies:            "127.0.0.1/32",
	}, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.10:1234"
	req.Header.Set("X-Debug-User", "user-1")
	rr := httptest.NewRecorder()

	called := false
	mw.OptionalHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	})).ServeHTTP(rr, req)

	if called {
		t.Fatal("expected next handler not to be called")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}
