package devheaders

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	jwtauth "github.com/aatuh/api-toolkit/v3/middleware/auth/jwt"
)

func TestAliasesExposeComparableDevHeaderSurface(t *testing.T) {
	if !reflect.TypeOf(Config{}).Comparable() {
		t.Fatal("integration Config alias should expose the comparable devheaders Config surface")
	}
	if !reflect.TypeOf((*Middleware)(nil)).Elem().Comparable() {
		t.Fatal("integration Middleware alias should expose the comparable devheaders Middleware surface")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg := LoadConfig(nil)

	if cfg.Enabled {
		t.Fatal("expected dev headers fallback to default disabled")
	}
	if cfg.UserIDHeader != "X-Debug-User" {
		t.Fatalf("UserIDHeader = %q, want %q", cfg.UserIDHeader, "X-Debug-User")
	}
	if cfg.DefaultLanguage != "fi" {
		t.Fatalf("DefaultLanguage = %q, want %q", cfg.DefaultLanguage, "fi")
	}
	if cfg.AllowDangerousDevBypasses {
		t.Fatal("expected dangerous dev bypasses to default disabled")
	}
	if cfg.TrustedProxies != "127.0.0.1/32,::1/128" {
		t.Fatalf("TrustedProxies = %#v, want loopback defaults", cfg.TrustedProxies)
	}
}

func TestLoadConfigEnvOverrides(t *testing.T) {
	t.Setenv("DEV_AUTH_ALLOW_DANGEROUS_DEV_BYPASSES", "true")
	t.Setenv("DEV_AUTH_TRUSTED_PROXIES", "10.0.0.0/8, 192.168.0.0/16")

	cfg := LoadConfig(nil)

	if !cfg.AllowDangerousDevBypasses {
		t.Fatal("expected dangerous dev bypasses enabled from env")
	}
	if cfg.TrustedProxies != "10.0.0.0/8, 192.168.0.0/16" {
		t.Fatalf("TrustedProxies = %#v, want configured values", cfg.TrustedProxies)
	}
}

func TestNewDisabledConfig(t *testing.T) {
	mw, err := New(Config{Enabled: false}, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if mw == nil {
		t.Fatal("expected middleware instance")
	}
}

func TestNewEnabledConfigRequiresDangerousBypassOptIn(t *testing.T) {
	_, err := New(Config{
		Enabled:        true,
		UserIDHeader:   "X-Debug-User",
		TrustedProxies: "127.0.0.1/32",
	}, nil)
	if err == nil {
		t.Fatal("expected enabled dev headers to require dangerous bypass opt-in")
	}
}

func TestHandlerPropagatesConfiguredHeaders(t *testing.T) {
	mw, err := New(Config{
		Enabled:                   true,
		UserIDHeader:              "X-User",
		EmailHeader:               "X-Email",
		FirstNameHeader:           "X-First",
		LastNameHeader:            "X-Last",
		DefaultLanguage:           "sv",
		AllowDangerousDevBypasses: true,
		TrustedProxies:            "127.0.0.1/32",
	}, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subj, ok := jwtauth.SubjectFromContext(r.Context())
		if !ok {
			t.Fatal("expected subject in context")
		}
		if subj.UserID != "user-1" || subj.Email != "user@example.com" ||
			subj.First != "Ada" || subj.Last != "Lovelace" || subj.Language != "sv" {
			t.Fatalf("subject = %#v, want configured header values", subj)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-User", " user-1 ")
	req.Header.Set("X-Email", " user@example.com ")
	req.Header.Set("X-First", "Ada")
	req.Header.Set("X-Last", "Lovelace")
	rr := httptest.NewRecorder()

	mw.Handler(next).ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}
