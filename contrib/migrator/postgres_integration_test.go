//go:build postgres

package migrator

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/aatuh/api-toolkit/contrib/v4/internal/testpostgres"
)

func TestRunnerAppliesAndDetectsMismatchedRealPostgresMigrations(t *testing.T) {
	h := testpostgres.New(t)
	db, err := sql.Open("pgx", h.DatabaseURL())
	if err != nil {
		t.Fatalf("open real migration database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	runner := New(db, Options{
		TableName: "integration_migrations",
		EmbeddedFSs: []fs.FS{fstest.MapFS{
			"migrations/20260722000000_create_fixture.up.sql":   &fstest.MapFile{Data: []byte("CREATE TABLE migration_fixture (id text PRIMARY KEY);")},
			"migrations/20260722000000_create_fixture.down.sql": &fstest.MapFile{Data: []byte("DROP TABLE migration_fixture;")},
		}},
		AllowDangerousDown: true,
	})
	ctx := context.Background()
	if err := runner.Up(ctx); err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO migration_fixture (id) VALUES ($1)", "fixture"); err != nil {
		t.Fatalf("query migrated table: %v", err)
	}
	status, err := runner.Status(ctx)
	if err != nil || !strings.Contains(status, "20260722000000") {
		t.Fatalf("Status() = (%q, %v)", status, err)
	}
	mismatched := New(db, Options{
		TableName: "integration_migrations",
		EmbeddedFSs: []fs.FS{fstest.MapFS{
			"migrations/20260722000000_create_fixture.up.sql": &fstest.MapFile{Data: []byte("CREATE TABLE migration_fixture (id bigint PRIMARY KEY);")},
		}},
	})
	if err := mismatched.Up(ctx); err == nil {
		t.Fatal("Up() accepted a migration checksum mismatch")
	} else {
		var mismatch *ChecksumMismatchError
		if !errors.As(err, &mismatch) {
			t.Fatalf("Up() mismatch error = %T, want ChecksumMismatchError", err)
		}
	}
	if err := runner.Down(ctx); err != nil {
		t.Fatalf("Down() error = %v", err)
	}
}
