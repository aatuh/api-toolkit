package negotiation

import (
	"strings"
	"testing"
)

func FuzzParseAcceptAndContentType(f *testing.F) {
	for _, seed := range []string{
		"application/*;q=0.8, application/json;q=0.8, text/plain;q=0.9",
		"application/vnd.example+json",
		"*/*;q=0",
		"application/json;q=2",
		"text/plain; charset=utf-8",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, header string) {
		header = limitNegotiationFuzzString(header, 4096)
		parsed, err := ParseAccept(header)
		if err == nil {
			for i, accept := range parsed {
				if accept.MediaType == "" || accept.Type == "" || accept.Subtype == "" {
					t.Fatalf("ParseAccept returned incomplete media type from %q: %#v", header, accept)
				}
				if accept.MediaType != strings.ToLower(accept.MediaType) {
					t.Fatalf("ParseAccept returned non-normalized media type from %q: %#v", header, accept)
				}
				if accept.Q < 0 || accept.Q > 1 {
					t.Fatalf("ParseAccept returned q outside [0,1] from %q: %#v", header, accept)
				}
				if _, ok := accept.Params["q"]; ok {
					t.Fatalf("ParseAccept kept q in params from %q: %#v", header, accept)
				}
				if i > 0 && acceptSortsBefore(parsed[i], parsed[i-1]) {
					t.Fatalf("ParseAccept returned unsorted preferences from %q: %#v", header, parsed)
				}
			}
		}

		_, _ = Negotiate(header, []MediaType{"application/json", "text/plain"})
		_ = ContentTypeAllowed(header, []MediaType{"application/json", "text/plain", "application/*+json"})
	})
}

func acceptSortsBefore(candidate, previous Accept) bool {
	if candidate.Q != previous.Q {
		return candidate.Q > previous.Q
	}
	if specificity(candidate) != specificity(previous) {
		return specificity(candidate) > specificity(previous)
	}
	return candidate.Order < previous.Order
}

func limitNegotiationFuzzString(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
