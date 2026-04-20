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
	if cfg.AllowDangerousDevBypasses {
		t.Fatal("expected dangerous dev bypasses to default disabled")
	}
	if len(cfg.TrustedProxies) != 2 {
		t.Fatalf("TrustedProxies len = %d, want %d", len(cfg.TrustedProxies), 2)
	}
	if cfg.TrustedProxies[0] != "127.0.0.1/32" || cfg.TrustedProxies[1] != "::1/128" {
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
	if len(cfg.TrustedProxies) != 2 {
		t.Fatalf("TrustedProxies len = %d, want %d", len(cfg.TrustedProxies), 2)
	}
	if cfg.TrustedProxies[0] != "10.0.0.0/8" || cfg.TrustedProxies[1] != "192.168.0.0/16" {
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
