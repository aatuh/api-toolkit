package version

import (
	"net/http"
	"testing"

	"github.com/aatuh/api-toolkit/v4/specs"
)

type stubRouteRegistrar struct {
	patterns []string
}

func (s *stubRouteRegistrar) Get(pattern string, _ http.HandlerFunc) {
	s.patterns = append(s.patterns, pattern)
}

func TestRegisterRoutesToUsesMinimalRegistrar(t *testing.T) {
	router := &stubRouteRegistrar{}
	handler := NewHandler(Config{})

	handler.RegisterRoutesTo(router)

	if len(router.patterns) != 1 {
		t.Fatalf("expected 1 route, got %d", len(router.patterns))
	}
	if router.patterns[0] != specs.Version {
		t.Fatalf("route = %q", router.patterns[0])
	}
}
