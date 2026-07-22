// Package testpostgres provides safe real-PostgreSQL fixtures for contrib
// adapter contract tests. It is internal because the endpoint, credentials,
// cleanup semantics, and supported major are test infrastructure rather than
// an adopter-facing API.
package testpostgres

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// EnvironmentDSN names the test-only PostgreSQL connection setting.
	EnvironmentDSN = "API_TOOLKIT_TEST_POSTGRES_DSN"

	// SupportedMajor is the PostgreSQL major exercised by supported adapters.
	SupportedMajor = 18

	testUser     = "api_toolkit_test"
	testDatabase = "api_toolkit_test"
	testPort     = "54329"
)

var fixedTime = time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)

// Version identifies the running PostgreSQL server without exposing an
// endpoint or credentials.
type Version struct {
	Major int
	Raw   string
}

// Migration is one ordered SQL migration applied to an isolated test schema.
type Migration struct {
	Name string
	SQL  string
}

// Harness owns an ephemeral database and per-test schema. Close releases every
// connection and drops the database. Use New in tests so cleanup is automatic.
type Harness struct {
	Pool     *pgxpool.Pool
	Database string
	Schema   string
	Version  Version

	admin *pgxpool.Pool
	next  atomic.Uint64
}

// New opens a real PostgreSQL harness and registers cleanup with t. It fails
// the test when the explicitly configured local test endpoint is unavailable.
func New(t testing.TB) *Harness {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	h, err := Open(ctx)
	if err != nil {
		t.Fatalf("open real PostgreSQL test harness: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		if err := h.Close(cleanupCtx); err != nil {
			t.Errorf("clean up real PostgreSQL test harness: %v", err)
		}
	})
	return h
}

// Open creates an ephemeral database from EnvironmentDSN. The configured DSN
// must use the dedicated loopback-only test account and database.
func Open(ctx context.Context) (*Harness, error) {
	dsn, err := testDSNFromEnvironment()
	if err != nil {
		return nil, err
	}
	return open(ctx, dsn)
}

func open(ctx context.Context, dsn string) (*Harness, error) {
	baseConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("parse real PostgreSQL test endpoint")
	}
	admin, err := pgxpool.NewWithConfig(ctx, baseConfig)
	if err != nil {
		return nil, fmt.Errorf("connect to real PostgreSQL test endpoint %s", sanitizeDSN(dsn))
	}
	closeAdmin := true
	defer func() {
		if closeAdmin {
			admin.Close()
		}
	}()

	version, err := detectVersion(ctx, admin)
	if err != nil {
		return nil, err
	}
	if version.Major != SupportedMajor {
		return nil, fmt.Errorf("PostgreSQL major %d is unsupported; require %d", version.Major, SupportedMajor)
	}

	database, err := generatedIdentifier(testDatabase)
	if err != nil {
		return nil, err
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+quoteIdentifier(database)); err != nil {
		return nil, errors.New("create ephemeral PostgreSQL test database")
	}
	databaseCreated := true
	defer func() {
		if databaseCreated {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = dropDatabase(cleanupCtx, admin, database)
		}
	}()

	schema, err := generatedIdentifier("case")
	if err != nil {
		return nil, err
	}
	databaseDSN, err := withDatabase(dsn, database)
	if err != nil {
		return nil, err
	}
	poolConfig, err := pgxpool.ParseConfig(databaseDSN)
	if err != nil {
		return nil, errors.New("configure ephemeral PostgreSQL test database")
	}
	if poolConfig.ConnConfig.RuntimeParams == nil {
		poolConfig.ConnConfig.RuntimeParams = make(map[string]string)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = quoteIdentifier(schema)
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, errors.New("connect to ephemeral PostgreSQL test database")
	}
	closePool := true
	defer func() {
		if closePool {
			pool.Close()
		}
	}()
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+quoteIdentifier(schema)); err != nil {
		return nil, errors.New("create isolated PostgreSQL test schema")
	}

	closeAdmin = false
	databaseCreated = false
	closePool = false
	return &Harness{
		Pool:     pool,
		Database: database,
		Schema:   schema,
		Version:  version,
		admin:    admin,
	}, nil
}

// ApplyMigrations applies migrations in order to the harness schema.
func (h *Harness) ApplyMigrations(ctx context.Context, migrations ...Migration) error {
	if h == nil || h.Pool == nil {
		return errors.New("PostgreSQL test harness is not initialized")
	}
	for _, migration := range migrations {
		if strings.TrimSpace(migration.Name) == "" || strings.TrimSpace(migration.SQL) == "" {
			return errors.New("PostgreSQL test migration name and SQL are required")
		}
		if _, err := h.Pool.Exec(ctx, migration.SQL); err != nil {
			return fmt.Errorf("apply PostgreSQL test migration %q", migration.Name)
		}
	}
	return nil
}

// Begin starts a transaction and registers a rollback cleanup. Explicit commits
// are allowed; the cleanup then becomes a harmless no-op.
func (h *Harness) Begin(t testing.TB, ctx context.Context) (pgx.Tx, error) {
	t.Helper()
	if h == nil || h.Pool == nil {
		return nil, errors.New("PostgreSQL test harness is not initialized")
	}
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		return nil, errors.New("begin PostgreSQL test transaction")
	}
	t.Cleanup(func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	})
	return tx, nil
}

// NextText returns deterministic fixture data scoped to this harness.
func (h *Harness) NextText(prefix string) string {
	if h == nil {
		return prefix + "-000"
	}
	return fmt.Sprintf("%s-%03d", prefix, h.next.Add(1))
}

// FixedTime returns the stable timestamp used by real-adapter fixtures.
func (h *Harness) FixedTime() time.Time { return fixedTime }

// CanceledContext returns an already-canceled context for cancellation tests.
func (h *Harness) CanceledContext(t testing.TB) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// TerminateConnection interrupts a checked-out connection through the admin
// pool, allowing adapters to exercise real connection-loss behavior.
func (h *Harness) TerminateConnection(ctx context.Context, conn *pgxpool.Conn) error {
	if h == nil || h.admin == nil || conn == nil {
		return errors.New("PostgreSQL test connection is required")
	}
	var pid int32
	if err := conn.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
		return errors.New("identify PostgreSQL test connection")
	}
	var terminated bool
	if err := h.admin.QueryRow(ctx, "SELECT pg_terminate_backend($1)", pid).Scan(&terminated); err != nil {
		return errors.New("interrupt PostgreSQL test connection")
	}
	if !terminated {
		return errors.New("PostgreSQL test connection was not interrupted")
	}
	return nil
}

// Close closes pools and drops the ephemeral database. It is safe to call once.
func (h *Harness) Close(ctx context.Context) error {
	if h == nil {
		return nil
	}
	if h.Pool != nil {
		h.Pool.Close()
		h.Pool = nil
	}
	if h.admin == nil {
		return nil
	}
	defer func() {
		h.admin.Close()
		h.admin = nil
	}()
	return dropDatabase(ctx, h.admin, h.Database)
}

func testDSNFromEnvironment() (string, error) {
	dsn := os.Getenv(EnvironmentDSN)
	if dsn == "" {
		return "", fmt.Errorf("%s is required; make test-postgres supplies a safe loopback default", EnvironmentDSN)
	}
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return "", errors.New("PostgreSQL test DSN must be a postgres URL")
	}
	if !isLoopbackHost(u.Hostname()) || u.Port() != testPort {
		return "", errors.New("PostgreSQL test DSN must use loopback host and port 54329")
	}
	if u.User == nil || u.User.Username() != testUser {
		return "", errors.New("PostgreSQL test DSN must use the dedicated test user")
	}
	if strings.TrimPrefix(u.Path, "/") != testDatabase {
		return "", errors.New("PostgreSQL test DSN must use the dedicated test database")
	}
	if u.Query().Get("sslmode") != "disable" {
		return "", errors.New("PostgreSQL test DSN must explicitly disable TLS for the local test endpoint")
	}
	return dsn, nil
}

func detectVersion(ctx context.Context, pool *pgxpool.Pool) (Version, error) {
	var raw string
	if err := pool.QueryRow(ctx, "SHOW server_version_num").Scan(&raw); err != nil {
		return Version{}, errors.New("detect PostgreSQL server version")
	}
	number, err := strconv.Atoi(raw)
	if err != nil || number < 10000 {
		return Version{}, errors.New("parse PostgreSQL server version")
	}
	return Version{Major: number / 10000, Raw: raw}, nil
}

func withDatabase(dsn, database string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", errors.New("parse PostgreSQL test DSN")
	}
	u.Path = "/" + database
	return u.String(), nil
}

func dropDatabase(ctx context.Context, admin *pgxpool.Pool, database string) error {
	if admin == nil || database == "" {
		return nil
	}
	if _, err := admin.Exec(ctx, "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()", database); err != nil {
		return errors.New("terminate ephemeral PostgreSQL test connections")
	}
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+quoteIdentifier(database)); err != nil {
		return errors.New("drop ephemeral PostgreSQL test database")
	}
	return nil
}

func generatedIdentifier(prefix string) (string, error) {
	var entropy [8]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", errors.New("generate PostgreSQL test identifier")
	}
	return prefix + "_" + fmt.Sprintf("%x", entropy), nil
}

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func sanitizeDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "PostgreSQL test endpoint"
	}
	return u.Scheme + "://" + u.Host + u.EscapedPath()
}
