package migrator

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
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
			State:    migrationStateApplied,
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

func TestLoadMigrationsReadsEmbeddedFSMigrationsDir(t *testing.T) {
	r := &Runner{
		Opts: Options{
			EmbeddedFSs: []fs.FS{
				fstest.MapFS{
					"migrations/20240101000000_init.up.sql":   &fstest.MapFile{Data: []byte("create table widgets(id bigint);")},
					"migrations/20240101000000_init.down.sql": &fstest.MapFile{Data: []byte("drop table widgets;")},
				},
			},
		},
	}

	if err := r.loadMigrations(); err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	if got := len(r.migrations); got != 2 {
		t.Fatalf("loaded migrations = %d, want 2", got)
	}
	if up := r.find(20240101000000, "up"); up == nil {
		t.Fatal("expected embedded up migration to load from migrations/ directory")
	}
	if down := r.find(20240101000000, "down"); down == nil {
		t.Fatal("expected embedded down migration to load from migrations/ directory")
	}
}

func TestLoadMigrationsReadsEmbeddedFSRootFilesWhenNoMigrationsDir(t *testing.T) {
	r := &Runner{
		Opts: Options{
			EmbeddedFSs: []fs.FS{
				fstest.MapFS{
					"20240101000000_init.up.sql": &fstest.MapFile{Data: []byte("create table widgets(id bigint);")},
				},
			},
		},
	}

	if err := r.loadMigrations(); err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	if got := len(r.migrations); got != 1 {
		t.Fatalf("loaded migrations = %d, want 1", got)
	}
	if up := r.find(20240101000000, "up"); up == nil {
		t.Fatal("expected embedded root migration to load")
	}
}

func TestPendingUpBlocksUnresolvedMigrationStates(t *testing.T) {
	r := &Runner{
		Opts: Options{TableName: "migration_runs"},
		migrations: []*Migration{
			{
				Version:  20240101000000,
				Name:     "init",
				Dir:      "up",
				Checksum: "expected",
			},
		},
	}
	tests := []struct {
		name  string
		state string
	}{
		{name: "started", state: migrationStateStarted},
		{name: "uncertain", state: migrationStateUncertain},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applied := []appliedRow{
				{
					Version:  20240101000000,
					Name:     "init",
					Checksum: "expected",
					State:    tt.state,
				},
			}

			_, err := r.pendingUp(applied)
			var unresolved *UnresolvedMigrationStateError
			if !errors.As(err, &unresolved) {
				t.Fatalf("expected UnresolvedMigrationStateError, got %T", err)
			}
			if unresolved.State != tt.state {
				t.Fatalf("unresolved state = %q, want %q", unresolved.State, tt.state)
			}
		})
	}
}

func TestPendingUpKeepsFailedMigrationsPending(t *testing.T) {
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
			Checksum: "expected",
			Success:  false,
			State:    migrationStateFailed,
		},
	}

	pending, err := r.pendingUp(applied)
	if err != nil {
		t.Fatalf("pendingUp() error = %v", err)
	}
	if len(pending) != 1 || pending[0].Version != 20240101000000 {
		t.Fatalf("pendingUp() = %#v, want version 20240101000000 pending", pending)
	}
}

func TestApplyOneReturnsUncertainErrorWhenCommitAckFails(t *testing.T) {
	commitErr := errors.New("commit lost")
	tx := &fakeMigrationTx{commitErr: commitErr}
	var recorded []recordStateCall
	r := &Runner{
		Opts: Options{TableName: "migration_runs"},
		beginTxHook: func(context.Context, *sql.TxOptions) (migrationTx, error) {
			return tx, nil
		},
		execContextHook: func(
			_ context.Context, _ string, args ...any,
		) (sql.Result, error) {
			call := extractRecordStateCall(t, args...)
			recorded = append(recorded, call)
			return fakeSQLResult(1), nil
		},
	}
	m := &Migration{
		Version:  20240101000000,
		Name:     "init",
		SQL:      "create table widgets(id bigint);",
		Checksum: "abc123",
	}

	err := r.applyOne(context.Background(), m)
	var uncertain *UncertainMigrationError
	if !errors.As(err, &uncertain) {
		t.Fatalf("expected UncertainMigrationError, got %T", err)
	}
	if !errors.Is(err, commitErr) {
		t.Fatalf("expected wrapped commit error, got %v", err)
	}
	if tx.commitCount != 1 {
		t.Fatalf("Commit() calls = %d, want 1", tx.commitCount)
	}
	if len(recorded) != 2 {
		t.Fatalf("recordState() calls = %d, want 2", len(recorded))
	}
	assertRecordStateCall(t, recorded[0], migrationStateStarted, false)
	assertRecordStateCall(t, recorded[1], migrationStateUncertain, false)
}

func TestApplyOneReturnsUncertainErrorWhenAppliedStateCannotBeRecorded(t *testing.T) {
	recordErr := errors.New("write failed")
	tx := &fakeMigrationTx{}
	var recorded []recordStateCall
	r := &Runner{
		Opts: Options{TableName: "migration_runs"},
		beginTxHook: func(context.Context, *sql.TxOptions) (migrationTx, error) {
			return tx, nil
		},
		execContextHook: func(
			_ context.Context, _ string, args ...any,
		) (sql.Result, error) {
			call := recordStateCall{
				version:  args[0].(int64),
				name:     args[1].(string),
				checksum: args[2].(string),
				execMS:   args[3].(int),
				success:  args[4].(bool),
				state:    args[5].(string),
			}
			recorded = append(recorded, call)
			if call.state == migrationStateApplied {
				return fakeSQLResult(0), recordErr
			}
			return fakeSQLResult(1), nil
		},
	}
	m := &Migration{
		Version:  20240101000000,
		Name:     "init",
		SQL:      "create table widgets(id bigint);",
		Checksum: "abc123",
	}

	err := r.applyOne(context.Background(), m)
	var uncertain *UncertainMigrationError
	if !errors.As(err, &uncertain) {
		t.Fatalf("expected UncertainMigrationError, got %T", err)
	}
	if !errors.Is(err, recordErr) {
		t.Fatalf("expected wrapped record error, got %v", err)
	}
	if tx.commitCount != 1 {
		t.Fatalf("Commit() calls = %d, want 1", tx.commitCount)
	}
	if len(recorded) != 3 {
		t.Fatalf("recordState() calls = %d, want 3", len(recorded))
	}
	assertRecordStateCall(t, recorded[0], migrationStateStarted, false)
	assertRecordStateCall(t, recorded[1], migrationStateApplied, true)
	assertRecordStateCall(t, recorded[2], migrationStateUncertain, false)
}

func TestCommitFailureRecordsStateThatBlocksNextRun(t *testing.T) {
	commitErr := errors.New("commit lost")
	tx := &fakeMigrationTx{commitErr: commitErr}
	var recorded []recordStateCall
	m := &Migration{
		Version:  20240101000000,
		Name:     "init",
		Dir:      "up",
		SQL:      "create table widgets(id bigint);",
		Checksum: "abc123",
	}
	r := &Runner{
		Opts: Options{TableName: "migration_runs"},
		migrations: []*Migration{
			m,
		},
		beginTxHook: func(context.Context, *sql.TxOptions) (migrationTx, error) {
			return tx, nil
		},
		execContextHook: func(
			_ context.Context, _ string, args ...any,
		) (sql.Result, error) {
			recorded = append(recorded, extractRecordStateCall(t, args...))
			return fakeSQLResult(1), nil
		},
	}

	err := r.applyOne(context.Background(), m)
	var uncertain *UncertainMigrationError
	if !errors.As(err, &uncertain) {
		t.Fatalf("expected UncertainMigrationError, got %T", err)
	}
	if !errors.Is(err, commitErr) {
		t.Fatalf("expected wrapped commit error, got %v", err)
	}
	if len(recorded) != 2 {
		t.Fatalf("recordState() calls = %d, want 2", len(recorded))
	}
	assertRecordStateCall(t, recorded[0], migrationStateStarted, false)
	assertRecordStateCall(t, recorded[1], migrationStateUncertain, false)

	_, err = r.pendingUp([]appliedRow{
		{
			Version:  recorded[1].version,
			Name:     recorded[1].name,
			Checksum: recorded[1].checksum,
			Success:  recorded[1].success,
			State:    recorded[1].state,
		},
	})
	var unresolved *UnresolvedMigrationStateError
	if !errors.As(err, &unresolved) {
		t.Fatalf("expected UnresolvedMigrationStateError, got %T", err)
	}
	if unresolved.State != migrationStateUncertain {
		t.Fatalf("unresolved state = %q, want %q", unresolved.State, migrationStateUncertain)
	}
}

func TestWithLockReturnsAdvisoryLockFailure(t *testing.T) {
	lockErr := errors.New("advisory lock unavailable")
	var fnCalled bool
	var unlockCalled bool
	r := &Runner{
		Opts: Options{LockKey: 1234},
		execContextHook: func(
			_ context.Context, query string, _ ...any,
		) (sql.Result, error) {
			switch {
			case strings.Contains(query, "pg_advisory_lock"):
				return nil, lockErr
			case strings.Contains(query, "pg_advisory_unlock"):
				unlockCalled = true
				return fakeSQLResult(1), nil
			default:
				return fakeSQLResult(1), nil
			}
		},
	}

	err := r.withLock(context.Background(), func(context.Context) error {
		fnCalled = true
		return nil
	})
	if !errors.Is(err, lockErr) {
		t.Fatalf("withLock() error = %v, want %v", err, lockErr)
	}
	if fnCalled {
		t.Fatal("migration function should not run after lock failure")
	}
	if unlockCalled {
		t.Fatal("unlock should not be attempted when lock acquisition fails")
	}
}

func TestWithLockUsesConfiguredLockTimeout(t *testing.T) {
	lockErr := errors.New("lock timed out")
	timeout := 25 * time.Millisecond
	var checkedDeadline bool
	r := &Runner{
		Opts: Options{LockKey: 1234, LockTimeout: timeout},
		execContextHook: func(
			ctx context.Context, query string, _ ...any,
		) (sql.Result, error) {
			if strings.Contains(query, "pg_advisory_lock") {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("lock context missing deadline")
				}
				remaining := time.Until(deadline)
				if remaining <= 0 || remaining > timeout {
					t.Fatalf("lock timeout remaining = %s, want within %s", remaining, timeout)
				}
				checkedDeadline = true
				return nil, lockErr
			}
			return fakeSQLResult(1), nil
		},
	}

	err := r.withLock(context.Background(), func(context.Context) error {
		t.Fatal("migration function should not run after lock failure")
		return nil
	})
	if !errors.Is(err, lockErr) {
		t.Fatalf("withLock() error = %v, want %v", err, lockErr)
	}
	if !strings.Contains(err.Error(), timeout.String()) {
		t.Fatalf("withLock() error %q missing timeout %s", err.Error(), timeout)
	}
	if !checkedDeadline {
		t.Fatal("lock deadline was not checked")
	}
}

func TestWithLockDefaultTimeoutMatchesCompatibilityBehavior(t *testing.T) {
	r := &Runner{}
	if got := r.lockTimeout(); got != defaultLockTimeout {
		t.Fatalf("lockTimeout() = %s, want %s", got, defaultLockTimeout)
	}
}

func TestWithLockReportsUnlockFailureWithoutMaskingPrimaryError(t *testing.T) {
	primaryErr := errors.New("migration failed")
	unlockErr := errors.New("unlock failed")
	var reported error
	var logs []string
	r := &Runner{
		Opts: Options{
			LockKey: 1234,
			Logger: func(format string, args ...any) {
				logs = append(logs, format)
			},
			UnlockFailureHandler: func(err error) {
				reported = err
			},
		},
		execContextHook: func(
			_ context.Context, query string, _ ...any,
		) (sql.Result, error) {
			switch {
			case strings.Contains(query, "pg_advisory_lock"):
				return fakeSQLResult(1), nil
			case strings.Contains(query, "pg_advisory_unlock"):
				return nil, unlockErr
			default:
				return fakeSQLResult(1), nil
			}
		},
	}

	err := r.withLock(context.Background(), func(context.Context) error {
		return primaryErr
	})
	if !errors.Is(err, primaryErr) {
		t.Fatalf("withLock() error = %v, want %v", err, primaryErr)
	}
	if !errors.Is(reported, unlockErr) {
		t.Fatalf("reported unlock error = %v, want %v", reported, unlockErr)
	}
	if len(logs) != 1 || logs[0] != "migration advisory unlock failed: %v" {
		t.Fatalf("logs = %#v", logs)
	}
}

func TestWithLockUnlockFailureHandlerPanicDoesNotMaskMigrationResult(t *testing.T) {
	primaryErr := errors.New("migration failed")
	unlockErr := errors.New("unlock failed")
	var logs []string
	r := &Runner{
		Opts: Options{
			LockKey: 1234,
			Logger: func(format string, args ...any) {
				logs = append(logs, format)
			},
			UnlockFailureHandler: func(error) {
				panic("handler failed")
			},
		},
		execContextHook: func(
			_ context.Context, query string, _ ...any,
		) (sql.Result, error) {
			switch {
			case strings.Contains(query, "pg_advisory_lock"):
				return fakeSQLResult(1), nil
			case strings.Contains(query, "pg_advisory_unlock"):
				return nil, unlockErr
			default:
				return fakeSQLResult(1), nil
			}
		},
	}

	err := r.withLock(context.Background(), func(context.Context) error {
		return primaryErr
	})
	if !errors.Is(err, primaryErr) {
		t.Fatalf("withLock() error = %v, want %v", err, primaryErr)
	}
	if len(logs) != 2 {
		t.Fatalf("logs = %#v, want unlock failure and handler panic logs", logs)
	}
}

func writeMigrationFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write migration file: %v", err)
	}
}

func extractRecordStateCall(t *testing.T, args ...any) recordStateCall {
	t.Helper()
	return recordStateCall{
		version:  args[0].(int64),
		name:     args[1].(string),
		checksum: args[2].(string),
		execMS:   args[3].(int),
		success:  args[4].(bool),
		state:    args[5].(string),
	}
}

func assertRecordStateCall(
	t *testing.T, call recordStateCall, wantState string, wantSuccess bool,
) {
	t.Helper()
	if call.state != wantState {
		t.Fatalf("recorded state = %q, want %q", call.state, wantState)
	}
	if call.success != wantSuccess {
		t.Fatalf("recorded success = %t, want %t", call.success, wantSuccess)
	}
	if call.version != 20240101000000 {
		t.Fatalf("recorded version = %d, want 20240101000000", call.version)
	}
	if call.name != "init" {
		t.Fatalf("recorded name = %q, want init", call.name)
	}
	if call.checksum != "abc123" {
		t.Fatalf("recorded checksum = %q, want abc123", call.checksum)
	}
}

type recordStateCall struct {
	version  int64
	name     string
	checksum string
	execMS   int
	success  bool
	state    string
}

type fakeMigrationTx struct {
	execSQL       []string
	commitCount   int
	rollbackCount int
	commitErr     error
}

func (t *fakeMigrationTx) ExecContext(
	_ context.Context, query string, _ ...any,
) (sql.Result, error) {
	t.execSQL = append(t.execSQL, query)
	return fakeSQLResult(1), nil
}

func (t *fakeMigrationTx) Commit() error {
	t.commitCount++
	return t.commitErr
}

func (t *fakeMigrationTx) Rollback() error {
	t.rollbackCount++
	return nil
}

type fakeSQLResult int64

func (r fakeSQLResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeSQLResult) RowsAffected() (int64, error) { return int64(r), nil }
