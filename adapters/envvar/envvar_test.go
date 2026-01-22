package envvar

import (
	"testing"
	"time"
)

func TestBindStruct(t *testing.T) {
	t.Setenv("APP_NAME", "demo")
	t.Setenv("APP_ENABLED", "true")
	t.Setenv("APP_TIMEOUT", "2s")

	type cfg struct {
		Name    string        `env:"NAME"`
		Enabled bool          `env:"ENABLED"`
		Timeout time.Duration `env:"TIMEOUT"`
		Count   int           `env:"COUNT" default:"5"`
	}
	var c cfg
	env := New()
	if err := env.BindWithPrefix(&c, "APP_"); err != nil {
		t.Fatalf("bind failed: %v", err)
	}
	if c.Name != "demo" {
		t.Fatalf("expected name demo, got %q", c.Name)
	}
	if !c.Enabled {
		t.Fatalf("expected enabled true")
	}
	if c.Timeout != 2*time.Second {
		t.Fatalf("expected timeout 2s, got %s", c.Timeout)
	}
	if c.Count != 5 {
		t.Fatalf("expected count default 5, got %d", c.Count)
	}
}
