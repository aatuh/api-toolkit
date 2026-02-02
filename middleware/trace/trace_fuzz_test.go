package trace

import "testing"

func FuzzParseTraceParent(f *testing.F) {
	seeds := []string{
		"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		"00-00000000000000000000000000000000-0000000000000000-00",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, header string) {
		traceID, spanID, _, ok := parseTraceParent(header)
		if ok {
			if !isValidTraceID(traceID) {
				t.Fatalf("invalid trace id for %q", header)
			}
			if !isValidSpanID(spanID) {
				t.Fatalf("invalid span id for %q", header)
			}
		}
	})
}
