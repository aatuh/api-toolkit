package pprof

import (
	"net/http"
	"testing"

	"github.com/aatuh/api-toolkit/specs"
)

type stubRouter struct {
	patterns []string
}

func (s *stubRouter) Get(pattern string, _ http.HandlerFunc) {
	s.patterns = append(s.patterns, pattern)
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
