//go:build postgres

package cliintegration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v4/internal/testpostgres"
)

func TestGeneratedFullProfileMigratesAgainstRealPostgres(t *testing.T) {
	h := testpostgres.New(t)
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	serviceDir := filepath.Join(t.TempDir(), "service")
	output, err := runCLI(context.Background(), append([]string{
		"new", "service",
		"--module", "example.com/postgres-integration",
		"--profile", "saas-api-full",
		"--auth", "api-key",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	})...)
	if err != nil {
		t.Fatalf("generate full profile: %v\n%s", err, output)
	}
	tidy := exec.CommandContext(context.Background(), "go", "mod", "tidy")
	tidy.Dir = serviceDir
	tidy.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
	if output, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("tidy generated profile: %v\n%s", err, output)
	}
	migrate := exec.CommandContext(context.Background(), "go", "run", "./cmd/migrate", "up")
	migrate.Dir = serviceDir
	migrate.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local", "DATABASE_URL="+h.DatabaseURL())
	if output, err := migrate.CombinedOutput(); err != nil {
		t.Fatalf("run generated persistence migrations: %v\n%s", err, output)
	}
	var tableCount int
	if err := h.Pool.QueryRow(context.Background(), "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name IN ('organizations', 'widgets', 'outbox_events')").Scan(&tableCount); err != nil || tableCount != 3 {
		t.Fatalf("generated persistence tables = (%d, %v), want 3", tableCount, err)
	}
}
