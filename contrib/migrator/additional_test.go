package migrator

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNewAppliesDefaultOptionsAndDownGuard(t *testing.T) {
	r := New(nil, Options{})
	if r.Opts.TableName != defaultTable || r.Opts.LockKey != defaultLock {
		t.Fatalf("defaults = %#v", r.Opts)
	}
	if err := r.Down(context.Background()); err == nil || !strings.Contains(err.Error(), "down is disabled") {
		t.Fatalf("Down() error = %v", err)
	}
}

func TestLoadMigrationsReadsLegacyDirectoryAndIgnoresNonMigrationFiles(t *testing.T) {
	dir := t.TempDir()
	writeMigrationFile(t, dir, "20240101000000_init.up.sql", "create table widgets(id bigint);")
	writeMigrationFile(t, dir, "notes.txt", "not a migration")
	r := &Runner{Opts: Options{MigrationsDir: dir}}
	if err := r.loadMigrations(); err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	if len(r.migrations) != 1 || r.find(20240101000000, "up") == nil {
		t.Fatalf("migrations = %#v", r.migrations)
	}
}

func TestUncertainAndUnresolvedErrorMessages(t *testing.T) {
	base := errors.New("commit failed")
	uncertain := &UncertainMigrationError{Version: 1, Name: "init", Err: base}
	if !errors.Is(uncertain, base) || !strings.Contains(uncertain.Error(), defaultTable) {
		t.Fatalf("uncertain error = %v", uncertain)
	}
	unresolved := (&UnresolvedMigrationStateError{Version: 2, Name: "widgets", State: migrationStateStarted}).Error()
	if !strings.Contains(unresolved, defaultTable) || !strings.Contains(unresolved, migrationStateStarted) {
		t.Fatalf("unresolved error = %q", unresolved)
	}
}
