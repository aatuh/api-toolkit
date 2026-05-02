package envvar

import (
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v2/ports"
)

type adapterExtensions interface {
	TryGet(string) (string, error)
	TryGetBool(string) (bool, error)
	TryGetInt(string) (int, error)
	TryLoadEnvFiles([]string) error
	MustLoadEnvFiles([]string)
	TryBindWithPrefix(any, string) error
}

var (
	_ ports.EnvVar      = (*Adapter)(nil)
	_ adapterExtensions = (*Adapter)(nil)
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
	env := &Adapter{}
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

func TestTryGetVariantsReturnErrorsWithoutPanicking(t *testing.T) {
	t.Setenv("DEMO_BOOL", "not-a-bool")
	t.Setenv("DEMO_INT", "forty-two")
	env := &Adapter{}

	if _, err := env.TryGet("DEMO_MISSING"); err == nil {
		t.Fatal("expected missing environment variable error")
	}
	if _, err := env.TryGetBool("DEMO_BOOL"); err == nil {
		t.Fatal("expected invalid bool parse error")
	}
	if _, err := env.TryGetInt("DEMO_INT"); err == nil {
		t.Fatal("expected invalid int parse error")
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic from MustGet on invalid bool")
		}
	}()
	_ = env.MustGetBool("DEMO_BOOL")
}

func TestTryGetAndMustLoadEnvFilesDistinguishFailureModes(t *testing.T) {
	env := &Adapter{}
	badPath := t.TempDir()
	if err := env.TryLoadEnvFiles([]string{badPath}); err == nil {
		t.Fatal("expected error from TryLoadEnvFiles")
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic from MustLoadEnvFiles")
		}
	}()
	env.MustLoadEnvFiles([]string{badPath})
}

func TestTryBindReturnsParseError(t *testing.T) {
	t.Setenv("APP_COUNT", "invalid")
	type cfg struct {
		Count int `env:"COUNT"`
	}
	var c cfg
	env := &Adapter{}

	if err := env.TryBindWithPrefix(&c, "APP_"); err == nil {
		t.Fatal("expected parse error from TryBindWithPrefix")
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic from MustBindWithPrefix")
		}
	}()
	env.MustBindWithPrefix(&c, "APP_")
}
