package migrate

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresCommand(t *testing.T) {
	if os.Getenv("GO_WANT_MIGRATE_HELPER") == "1" {
		os.Args = []string{"migrate"}
		Run(Config{})
		return
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, exe, "-test.run=TestRunRequiresCommand")
	cmd.Env = append(os.Environ(), "GO_WANT_MIGRATE_HELPER=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected subprocess to exit with failure")
	}
	if !strings.Contains(string(out), "command required: up | down | status") {
		t.Fatalf("unexpected output: %s", out)
	}
}
