package migrator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPendingUpChecksumMismatch(t *testing.T) {
	r := &Runner{
		migrations: []*Migration{
			{
				Version:  20240101000000,
				Name:     "init",
				Dir:      "up",
				Checksum: "expected",
			},
		},
	}
	applied := []appliedRow{
		{
			Version:  20240101000000,
			Name:     "init",
			Checksum: "actual",
			Success:  true,
		},
	}
	_, err := r.pendingUp(applied)
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	var mismatch *ChecksumMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected ChecksumMismatchError, got %T", err)
	}
	if mismatch.Version != 20240101000000 || mismatch.Name != "init" {
		t.Fatalf("unexpected mismatch details: %#v", mismatch)
	}
}

func TestLoadMigrationsRejectsDuplicateVersionDirectionAcrossDirectories(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	writeMigrationFile(t, dir1, "20240101000000_init.up.sql", "create table one();")
	writeMigrationFile(t, dir2, "20240101000000_init.up.sql", "create table two();")

	r := &Runner{
		Opts: Options{
			MigrationsDirs: []string{dir1, dir2},
		},
	}

	err := r.loadMigrations()
	if err == nil {
		t.Fatal("expected duplicate migration error")
	}
	if !strings.Contains(err.Error(), "duplicate migration") {
		t.Fatalf("expected duplicate migration error, got %v", err)
	}
}

func writeMigrationFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write migration file: %v", err)
	}
}
