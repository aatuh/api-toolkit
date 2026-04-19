package migrator

import (
	"context"
	"database/sql"
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
