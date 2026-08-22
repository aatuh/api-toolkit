package testpostgres

import (
	"context"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aatuh/api-toolkit/contrib/v4/migrator"
)

func TestParseConfigRejectsUnsafeDSNWithoutEchoingIt(t *testing.T) {
	const secret = "must-not-appear"
	cases := []struct {
		name string
		dsn  string
	}{
		{name: "missing", dsn: ""},
		{name: "remote host", dsn: "postgres://api_toolkit_test:" + secret + "@db.example.test/postgres?sslmode=disable"},
		{name: "wrong user", dsn: "postgres://production:" + secret + "@127.0.0.1/postgres?sslmode=disable"},
		{name: "wrong database", dsn: "postgres://api_toolkit_test:" + secret + "@127.0.0.1/customer?sslmode=disable"},
		{name: "unexpected parameter", dsn: "postgres://api_toolkit_test:" + secret + "@127.0.0.1/postgres?application_name=unsafe&sslmode=disable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseConfig(tc.dsn)
			if err == nil {
				t.Fatal("parseConfig() error = nil, want rejected test DSN")
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("parseConfig() error leaked DSN secret: %v", err)
			}
		})
	}
}

func TestConfigFromEnvRequiresExplicitOptIn(t *testing.T) {
	t.Setenv(EnableEnv, "")
	t.Setenv(DSNEnv, "postgres://api_toolkit_test:api_toolkit_test@127.0.0.1/postgres?sslmode=disable")
	if _, err := configFromEnv(); err == nil {
		t.Fatal("configFromEnv() error = nil, want explicit opt-in error")
	}
}

func TestRollbackRequiresCallback(t *testing.T) {
	var h Harness
	if err := h.Rollback(context.Background(), nil); err == nil {
		t.Fatal("Rollback(nil) error = nil, want validation error")
	}
}

func TestHarnessLifecycle(t *testing.T) {
	requirePostgres(t)
	h := New(t)
	ctx := context.Background()

	if h.MajorVersion() != 18 {
		t.Fatalf("PostgreSQL major version = %d, want declared test major 18", h.MajorVersion())
	}
	var currentSchema string
	if err := h.Pool().QueryRow(ctx, "SELECT current_schema()").Scan(&currentSchema); err != nil {
		t.Fatalf("query current schema: %v", err)
	}
	if currentSchema != h.Schema() {
		t.Fatalf("current schema = %q, want %q", currentSchema, h.Schema())
	}
	if _, err := h.Pool().Exec(ctx, "CREATE TABLE widgets (id TEXT PRIMARY KEY)"); err != nil {
		t.Fatalf("create test table: %v", err)
	}
	fixtureID := h.FixtureID("widget")
	if fixtureID != h.FixtureID("widget") {
		t.Fatal("FixtureID() is not deterministic")
	}
	if err := h.Rollback(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO widgets (id) VALUES ($1)", fixtureID)
		return err
	}); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	var count int
	if err := h.Pool().QueryRow(ctx, "SELECT COUNT(*) FROM widgets").Scan(&count); err != nil {
		t.Fatalf("count rolled-back rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("rows after Rollback() = %d, want 0", count)
	}
}

func TestHarnessAppliesMigrationsAndRollsBackTransactions(t *testing.T) {
	requirePostgres(t)
	h := New(t)
	ctx := context.Background()

	migrations := fstest.MapFS{
		"migrations/20260822120000_widgets.up.sql": &fstest.MapFile{Data: []byte("CREATE TABLE migrated_widgets (id TEXT PRIMARY KEY);")},
	}
	if err := h.ApplyMigrations(ctx, migrator.Options{EmbeddedFSs: []fs.FS{migrations}}); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}
	if err := h.Rollback(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO migrated_widgets (id) VALUES ($1)", h.FixtureID("migrated"))
		return err
	}); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	var count int
	if err := h.Pool().QueryRow(ctx, "SELECT COUNT(*) FROM migrated_widgets").Scan(&count); err != nil {
		t.Fatalf("count migrated rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("rows after Rollback() = %d, want 0", count)
	}
}

func TestHarnessSupportsCancellationAndConnectionInterruption(t *testing.T) {
	requirePostgres(t)
	h := New(t)
	ctx := context.Background()
	if err := h.AssertContextCancellation(ctx); err != nil {
		t.Fatalf("AssertContextCancellation() error = %v", err)
	}

	conn, err := h.Pool().Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer conn.Release()
	if err := h.InterruptConnections(ctx); err != nil {
		t.Fatalf("InterruptConnections() error = %v", err)
	}
	if _, err := conn.Exec(ctx, "SELECT 1"); err == nil {
		t.Fatal("query succeeded after connection interruption")
	}
}

func TestHarnessIsolatesParallelTests(t *testing.T) {
	requirePostgres(t)
	for _, name := range []string{"first", "second", "third"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := New(t)
			if _, err := h.Pool().Exec(context.Background(), "CREATE TABLE isolation_probe (id INTEGER PRIMARY KEY)"); err != nil {
				t.Fatalf("create isolated table: %v", err)
			}
		})
	}
}

func requirePostgres(t *testing.T) {
	t.Helper()
	if os.Getenv(EnableEnv) != "1" {
		t.Skip("set API_TOOLKIT_TEST_POSTGRES=1 through make test-postgres to run real PostgreSQL harness tests")
	}
	if os.Getenv(DSNEnv) == "" {
		t.Fatal("API_TOOLKIT_TEST_POSTGRES_DSN is required when API_TOOLKIT_TEST_POSTGRES=1")
	}
}

func TestHarnessRejectsUnavailablePostgres(t *testing.T) {
	if os.Getenv(EnableEnv) == "1" {
		t.Skip("make test-postgres must fail on a missing configured service; do not replace its configured DSN in this integration run")
	}
	t.Setenv(EnableEnv, "1")
	t.Setenv(DSNEnv, "postgres://api_toolkit_test:api_toolkit_test@127.0.0.1:1/postgres?sslmode=disable")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	cfg, err := configFromEnv()
	if err != nil {
		t.Fatalf("configFromEnv() error = %v", err)
	}
	pool, err := pgxpool.New(ctx, cfg.adminDSN)
	if err != nil {
		return
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err == nil {
		t.Fatal("ping unavailable PostgreSQL test service error = nil")
	}
}
