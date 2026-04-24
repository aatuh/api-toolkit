package migrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestEmbeddedSchedulerMigrations(t *testing.T) {
	entries, err := fs.ReadDir(Migrations, ".")
	if err != nil {
		t.Fatalf("read embedded migrations: %v", err)
	}
	names := map[string]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		names[entry.Name()] = struct{}{}
	}
	for _, name := range []string{
		"00000100_scheduler_runs.up.sql",
		"00000100_scheduler_runs.down.sql",
	} {
		if _, ok := names[name]; !ok {
			t.Fatalf("embedded migration %s not found in %#v", name, names)
		}
		content, err := fs.ReadFile(Migrations, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.TrimSpace(string(content)) == "" {
			t.Fatalf("%s is empty", name)
		}
	}
}
