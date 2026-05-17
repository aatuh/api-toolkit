package pprof

import (
	"net/http"
	"testing"

	"github.com/aatuh/api-toolkit/v3/specs"
)

type stubRouter struct {
	patterns []string
	handlers []http.HandlerFunc
}

func (s *stubRouter) Get(pattern string, _ http.HandlerFunc) {
	s.patterns = append(s.patterns, pattern)
	s.handlers = append(s.handlers, nil)
}

func TestRegisterRoutes(t *testing.T) {
	r := &stubRouter{}

	RegisterRoutes(r)

	expected := []string{
		specs.PprofIndex,
		specs.PprofCmdline,
		specs.PprofProfile,
		specs.PprofSymbol,
		specs.PprofTrace,
	}

	if len(r.patterns) != len(expected) {
		t.Fatalf("expected %d routes, got %d", len(expected), len(r.patterns))
	}
	for i := range expected {
		if r.patterns[i] != expected[i] {
			t.Errorf("route %d: expected %q, got %q", i, expected[i], r.patterns[i])
		}
	}
}

func TestRegisterAdminRoutesRequiresWrapper(t *testing.T) {
	if err := RegisterAdminRoutes(&stubRouter{}, nil); err == nil {
		t.Fatal("expected error for missing admin wrapper")
	}
}

func TestRegisterAdminRoutesWrapsHandlers(t *testing.T) {
	router := &stubRouter{}
	wrapped := 0

	err := RegisterAdminRoutes(router, func(next http.Handler) http.Handler {
		wrapped++
		return next
	})
	if err != nil {
		t.Fatalf("register admin routes: %v", err)
	}
	if wrapped != len(router.patterns) {
		t.Fatalf("wrapped handlers = %d, want %d", wrapped, len(router.patterns))
	}
}
