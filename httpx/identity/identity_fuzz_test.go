package identity

import "testing"

func FuzzForwardedFor(f *testing.F) {
	seeds := []string{
		`for=192.0.2.1`,
		`for="[2001:db8::1]:443";proto=https;host=example.com`,
		`for=unknown`,
		`for="_hidden"`,
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, header string) {
		addrs, ok := forwardedForChain(header)
		if ok {
			for _, addr := range addrs {
				if !addr.IsValid() {
					t.Fatalf("expected valid address for %q", header)
				}
			}
		}
	})
}

func FuzzXForwardedFor(f *testing.F) {
	seeds := []string{
		"198.51.100.9",
		"198.51.100.9, 203.0.113.10",
		`"[2001:db8::2]"`,
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, header string) {
		addrs, ok := xForwardedForChain(header)
		if ok {
			for _, addr := range addrs {
				if !addr.IsValid() {
					t.Fatalf("expected valid address for %q", header)
				}
			}
		}
	})
}

func FuzzParseAddrValue(f *testing.F) {
	seeds := []string{
		"203.0.113.5",
		"[2001:db8::1]:443",
		"unknown",
		"",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		addr, ok := parseAddrValue(value)
		if ok && !addr.IsValid() {
			t.Fatalf("expected valid address for %q", value)
		}
	})
}
