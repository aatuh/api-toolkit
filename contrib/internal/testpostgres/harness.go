// Package testpostgres provides isolated PostgreSQL fixtures for contrib tests.
package testpostgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	pgxpooladapter "github.com/aatuh/api-toolkit/contrib/v4/adapters/pgxpool"
	"github.com/aatuh/api-toolkit/contrib/v4/migrator"
)

const (
	// EnableEnv must be set to "1" before the harness will use a database.
	EnableEnv = "API_TOOLKIT_TEST_POSTGRES"
	// DSNEnv contains the explicit test-only PostgreSQL admin DSN.
	DSNEnv = "API_TOOLKIT_TEST_POSTGRES_DSN"

	testUser     = "api_toolkit_test"
	testPassword = "api_toolkit_test"
	adminDB      = "postgres"

	operationTimeout = 15 * time.Second
	cleanupTimeout   = 5 * time.Second
)

var databaseSequence atomic.Uint64

type config struct {
	adminDSN string
}

// Harness owns one isolated PostgreSQL database and schema for a test.
// It closes the pool and drops the database during t.Cleanup.
type Harness struct {
	t        testing.TB
	admin    *pgxpool.Pool
	pool     *pgxpool.Pool
	dsn      string
	database string
	schema   string
	major    int
	seed     string
}

// New creates an isolated database and schema for t. It fails the test when
// PostgreSQL is unavailable or the configured DSN is not the dedicated local
// or service-container test endpoint.
func New(t testing.TB) *Harness {
	t.Helper()

	cfg, err := configFromEnv()
	if err != nil {
		t.Fatalf("configure PostgreSQL test harness: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()

	admin, err := pgxpool.New(ctx, cfg.adminDSN)
	if err != nil {
		t.Fatalf("connect to PostgreSQL test service: %v", sanitizedPostgresError(err))
	}

	major, err := serverMajor(ctx, admin)
	if err != nil {
		admin.Close()
		t.Fatalf("read PostgreSQL test service version: %v", sanitizedPostgresError(err))
	}

	seed := t.Name()
	database := databaseName(seed)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+quoteIdentifier(database)); err != nil {
		admin.Close()
		t.Fatalf("create isolated PostgreSQL test database: %v", sanitizedPostgresError(err))
	}

	dsn, err := databaseDSN(cfg.adminDSN, database)
	if err != nil {
		cleanupIncompleteHarness(t, admin, database)
		t.Fatalf("configure isolated PostgreSQL test database: %v", sanitizedPostgresError(err))
	}
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		cleanupIncompleteHarness(t, admin, database)
		t.Fatalf("configure isolated PostgreSQL connection pool: %v", sanitizedPostgresError(err))
	}
	if poolConfig.ConnConfig.RuntimeParams == nil {
		poolConfig.ConnConfig.RuntimeParams = make(map[string]string)
	}
	schema := schemaName(seed)
	poolConfig.ConnConfig.RuntimeParams["search_path"] = quoteIdentifier(schema)
	poolConfig.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		cleanupIncompleteHarness(t, admin, database)
		t.Fatalf("connect to isolated PostgreSQL test database: %v", sanitizedPostgresError(err))
	}
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+quoteIdentifier(schema)); err != nil {
		pool.Close()
		cleanupIncompleteHarness(t, admin, database)
		t.Fatalf("create isolated PostgreSQL test schema: %v", sanitizedPostgresError(err))
	}

	h := &Harness{
		t:        t,
		admin:    admin,
		pool:     pool,
		dsn:      dsn,
		database: database,
		schema:   schema,
		major:    major,
		seed:     seed,
	}
	t.Cleanup(h.cleanup)
	return h
}

// Pool returns the isolated pgx pool with its search path set to the test schema.
func (h *Harness) Pool() *pgxpool.Pool { return h.pool }

// Adapter returns the isolated pool through the contrib DatabasePool adapter.
func (h *Harness) Adapter() *pgxpooladapter.Adapter {
	return &pgxpooladapter.Adapter{Pool: h.pool}
}

// DSN returns the isolated test-only database DSN. Do not log this value.
func (h *Harness) DSN() string { return h.dsn }

// Database returns the isolated database name.
func (h *Harness) Database() string { return h.database }

// Schema returns the isolated schema name.
func (h *Harness) Schema() string { return h.schema }

// MajorVersion returns the connected PostgreSQL major version.
func (h *Harness) MajorVersion() int { return h.major }

// FixtureID returns a deterministic, test-scoped identifier for label.
func (h *Harness) FixtureID(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		label = "default"
	}
	sum := sha256.Sum256([]byte(h.seed + ":" + label))
	return "fixture_" + fmt.Sprintf("%x", sum[:8])
}

// Rollback runs fn in a transaction that is always rolled back. It permits
// tests to assert transactional work without leaving durable fixture state.
func (h *Harness) Rollback(ctx context.Context, fn func(pgx.Tx) error) error {
	if fn == nil {
		return errors.New("rollback callback is required")
	}
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		// Keep request values while allowing rollback after the caller's context
		// has already been canceled.
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()
	return fn(tx)
}

// ApplyMigrations applies a migrator configuration to the isolated database.
// The supplied options must name a directory or embedded migration filesystem.
func (h *Harness) ApplyMigrations(ctx context.Context, opts migrator.Options) error {
	dsn, err := databaseDSNWithSearchPath(h.dsn, h.schema)
	if err != nil {
		return errors.New("configure PostgreSQL test migration connection")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return errors.New("open PostgreSQL test migration connection")
	}
	defer func() {
		_ = db.Close()
	}()
	return migrator.New(db, opts).Up(ctx)
}

// AssertContextCancellation proves that an in-flight PostgreSQL query observes
// a context deadline.
func (h *Harness) AssertContextCancellation(ctx context.Context) error {
	cancelCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	_, err := h.pool.Exec(cancelCtx, "SELECT pg_sleep(1)")
	if err == nil {
		return errors.New("PostgreSQL query ignored a canceled context")
	}
	if !errors.Is(cancelCtx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("cancel PostgreSQL query: %w", cancelCtx.Err())
	}
	return nil
}

// InterruptConnections terminates pooled connections to this test database.
// Tests can retain a connection before calling it to exercise connection-loss
// behavior without affecting another test's database.
func (h *Harness) InterruptConnections(ctx context.Context) error {
	_, err := h.admin.Exec(ctx, `
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname = $1 AND pid <> pg_backend_pid()`, h.database)
	return err
}

func (h *Harness) cleanup() {
	h.pool.Close()
	if err := cleanupDatabase(h.admin, h.database); err != nil {
		h.t.Errorf("remove isolated PostgreSQL test database: %v", sanitizedPostgresError(err))
	}
	h.admin.Close()
}

func cleanupIncompleteHarness(t testing.TB, admin *pgxpool.Pool, database string) {
	t.Helper()
	if err := cleanupDatabase(admin, database); err != nil {
		t.Logf("remove incomplete isolated PostgreSQL test database: %v", sanitizedPostgresError(err))
	}
	admin.Close()
}

func configFromEnv() (config, error) {
	if os.Getenv(EnableEnv) != "1" {
		return config{}, fmt.Errorf("%s must be set to 1", EnableEnv)
	}
	return parseConfig(os.Getenv(DSNEnv))
}

func parseConfig(dsn string) (config, error) {
	if dsn == "" {
		return config{}, fmt.Errorf("%s is required", DSNEnv)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		return config{}, errors.New("test PostgreSQL DSN is malformed")
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return config{}, errors.New("test PostgreSQL DSN must use postgres scheme")
	}
	if parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost" && parsed.Hostname() != "postgres" {
		return config{}, errors.New("test PostgreSQL DSN host must be localhost, 127.0.0.1, or postgres")
	}
	if parsed.User == nil || parsed.User.Username() != testUser {
		return config{}, errors.New("test PostgreSQL DSN must use the dedicated test user")
	}
	password, ok := parsed.User.Password()
	if !ok || password != testPassword {
		return config{}, errors.New("test PostgreSQL DSN must use the dedicated test password")
	}
	if strings.TrimPrefix(parsed.EscapedPath(), "/") != adminDB {
		return config{}, errors.New("test PostgreSQL DSN must target the postgres administration database")
	}
	query := parsed.Query()
	if query.Get("sslmode") != "disable" || len(query) != 1 {
		return config{}, errors.New("test PostgreSQL DSN must set only sslmode=disable")
	}
	if parsed.Fragment != "" {
		return config{}, errors.New("test PostgreSQL DSN must not include a fragment")
	}
	return config{adminDSN: dsn}, nil
}

func serverMajor(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var versionNumber string
	if err := pool.QueryRow(ctx, "SHOW server_version_num").Scan(&versionNumber); err != nil {
		return 0, err
	}
	version, err := strconv.Atoi(versionNumber)
	if err != nil || version < 10000 {
		return 0, errors.New("PostgreSQL test service reported an invalid version")
	}
	return version / 10000, nil
}

func databaseDSN(adminDSN, database string) (string, error) {
	parsed, err := url.Parse(adminDSN)
	if err != nil {
		return "", err
	}
	parsed.Path = "/" + database
	return parsed.String(), nil
}

func databaseDSNWithSearchPath(dsn, schema string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func databaseName(testName string) string {
	sum := sha256.Sum256([]byte(testName))
	sequence := databaseSequence.Add(1)
	return fmt.Sprintf("api_toolkit_test_%x_%d_%d", sum[:6], os.Getpid(), sequence)
}

func schemaName(testName string) string {
	sum := sha256.Sum256([]byte(testName))
	return fmt.Sprintf("api_toolkit_schema_%x", sum[:6])
}

func cleanupDatabase(admin *pgxpool.Pool, database string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	if _, err := admin.Exec(ctx, `
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname = $1 AND pid <> pg_backend_pid()`, database); err != nil {
		return err
	}
	_, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+quoteIdentifier(database))
	return err
}

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func sanitizedPostgresError(err error) error {
	if err == nil {
		return nil
	}
	return errors.New("PostgreSQL test service operation failed")
}
