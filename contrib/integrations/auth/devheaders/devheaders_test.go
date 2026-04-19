package devheaders

import "testing"

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
