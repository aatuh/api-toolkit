package pgxpool

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/aatuh/api-toolkit/v3/ports"
)

type stubPoolStats struct{}

func (stubPoolStats) AcquireCount() int64            { return 11 }
func (stubPoolStats) AcquireDuration() time.Duration { return 12 * time.Millisecond }
func (stubPoolStats) AcquiredConns() int32           { return 13 }
func (stubPoolStats) CanceledAcquireCount() int64    { return 14 }
func (stubPoolStats) ConstructingConns() int32       { return 15 }
func (stubPoolStats) EmptyAcquireCount() int64       { return 16 }
func (stubPoolStats) IdleConns() int32               { return 17 }
func (stubPoolStats) MaxConns() int32                { return 18 }
func (stubPoolStats) NewConnsCount() int64           { return 19 }
func (stubPoolStats) TotalConns() int32              { return 20 }

func TestSnapshotFromPoolStatsCopiesPlainValues(t *testing.T) {
	t.Parallel()

	got := snapshotFromPoolStats(stubPoolStats{})

	if got.AcquireCount != 11 {
		t.Fatalf("AcquireCount = %d", got.AcquireCount)
	}
	if got.AcquireDuration != 12*time.Millisecond {
		t.Fatalf("AcquireDuration = %v", got.AcquireDuration)
	}
	if got.AcquiredConns != 13 {
		t.Fatalf("AcquiredConns = %d", got.AcquiredConns)
	}
	if got.CanceledAcquireCount != 14 {
		t.Fatalf("CanceledAcquireCount = %d", got.CanceledAcquireCount)
	}
	if got.ConstructingConns != 15 {
		t.Fatalf("ConstructingConns = %d", got.ConstructingConns)
	}
	if got.EmptyAcquireCount != 16 {
		t.Fatalf("EmptyAcquireCount = %d", got.EmptyAcquireCount)
	}
	if got.IdleConns != 17 {
		t.Fatalf("IdleConns = %d", got.IdleConns)
	}
	if got.MaxConns != 18 {
		t.Fatalf("MaxConns = %d", got.MaxConns)
	}
	if got.NewConnsCount != 19 {
		t.Fatalf("NewConnsCount = %d", got.NewConnsCount)
	}
	if got.TotalConns != 20 {
		t.Fatalf("TotalConns = %d", got.TotalConns)
	}
}

func TestSnapshotFromPoolStatsNil(t *testing.T) {
	t.Parallel()

	if got := snapshotFromPoolStats(nil); got != (ports.DatabasePoolSnapshot{}) {
		t.Fatalf("expected zero snapshot, got %+v", got)
	}
}

func TestAdapterStatSnapshotNilAdapter(t *testing.T) {
	t.Parallel()

	var adapter *Adapter
	if got := adapter.StatSnapshot(); got != (ports.DatabasePoolSnapshot{}) {
		t.Fatalf("expected zero snapshot, got %+v", got)
	}
}

func TestNewWithContextRejectsMalformedDSN(t *testing.T) {
	t.Parallel()

	_, err := NewWithContext(context.Background(), "://bad dsn")
	if err == nil {
		t.Fatal("NewWithContext() error = nil, want malformed DSN error")
	}
}

func TestNewWithContextExposesAdapterStatsAndAcquireErrors(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	pool, err := NewWithContext(ctx, "postgres://user:pass@localhost/db?host=/tmp/api-toolkit-missing-socket&connect_timeout=1")
	if err != nil {
		t.Fatalf("NewWithContext() error = %v", err)
	}
	t.Cleanup(pool.Close)

	snapshotter, ok := pool.(ports.DatabasePoolSnapshotProvider)
	if !ok {
		t.Fatal("pool does not expose snapshot stats")
	}
	if stats := snapshotter.StatSnapshot(); stats.MaxConns == 0 {
		t.Fatalf("StatSnapshot().MaxConns = %d, want non-zero stats", stats.MaxConns)
	}
	if _, err := pool.Acquire(ctx); err == nil {
		t.Fatal("Acquire() error = nil, want backend connection error")
	}
}

func TestTransactionWrapsUnderlyingPgxTx(t *testing.T) {
	t.Parallel()

	rows := &fakePgxRows{value: "query-value"}
	row := fakePgxRow{value: "row-value"}
	tx := &fakePgxTx{
		rows: rows,
		row:  row,
		tag:  pgconn.NewCommandTag("UPDATE 3"),
	}
	wrapped := &Transaction{Tx: tx}
	ctx := context.Background()

	result, err := wrapped.Exec(ctx, "update widgets set seen=true where id=$1", 7)
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if got := result.RowsAffected(); got != 3 {
		t.Fatalf("RowsAffected() = %d, want 3", got)
	}
	if tx.execSQL != "update widgets set seen=true where id=$1" {
		t.Fatalf("Exec() SQL = %q", tx.execSQL)
	}

	dbRows, err := wrapped.Query(ctx, "select status from widgets where id=$1", 7)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if !dbRows.Next() {
		t.Fatal("Query() rows.Next() = false, want true")
	}
	var gotQuery string
	if err := dbRows.Scan(&gotQuery); err != nil {
		t.Fatalf("rows.Scan() error = %v", err)
	}
	if gotQuery != "query-value" {
		t.Fatalf("rows.Scan() = %q, want %q", gotQuery, "query-value")
	}
	dbRows.Close()
	if tx.querySQL != "select status from widgets where id=$1" {
		t.Fatalf("Query() SQL = %q", tx.querySQL)
	}

	dbRow := wrapped.QueryRow(ctx, "select name from widgets where id=$1", 7)
	var gotRow string
	if err := dbRow.Scan(&gotRow); err != nil {
		t.Fatalf("QueryRow().Scan() error = %v", err)
	}
	if gotRow != "row-value" {
		t.Fatalf("QueryRow().Scan() = %q, want %q", gotRow, "row-value")
	}
	if tx.queryRowSQL != "select name from widgets where id=$1" {
		t.Fatalf("QueryRow() SQL = %q", tx.queryRowSQL)
	}
}

type fakePgxTx struct {
	execSQL     string
	querySQL    string
	queryRowSQL string
	rows        pgx.Rows
	row         pgx.Row
	tag         pgconn.CommandTag
	err         error
}

func (t *fakePgxTx) Begin(context.Context) (pgx.Tx, error) { return t, nil }
func (t *fakePgxTx) Commit(context.Context) error          { return nil }
func (t *fakePgxTx) Rollback(context.Context) error        { return nil }
func (t *fakePgxTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *fakePgxTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (t *fakePgxTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (t *fakePgxTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return &pgconn.StatementDescription{}, nil
}
func (t *fakePgxTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	t.execSQL = sql
	if t.err != nil {
		return pgconn.CommandTag{}, t.err
	}
	return t.tag, nil
}
func (t *fakePgxTx) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	t.querySQL = sql
	if t.err != nil {
		return nil, t.err
	}
	return t.rows, nil
}
func (t *fakePgxTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	t.queryRowSQL = sql
	return t.row
}
func (t *fakePgxTx) Conn() *pgx.Conn { return nil }

type fakePgxRows struct {
	value    string
	returned bool
}

func (r *fakePgxRows) Close()                                       {}
func (r *fakePgxRows) Err() error                                   { return nil }
func (r *fakePgxRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("SELECT 1") }
func (r *fakePgxRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakePgxRows) Next() bool {
	if r.returned {
		return false
	}
	r.returned = true
	return true
}
func (r *fakePgxRows) Scan(dest ...any) error {
	ptr := dest[0].(*string)
	*ptr = r.value
	return nil
}
func (r *fakePgxRows) Values() ([]any, error) { return []any{r.value}, nil }
func (r *fakePgxRows) RawValues() [][]byte    { return nil }
func (r *fakePgxRows) Conn() *pgx.Conn        { return nil }

type fakePgxRow struct {
	value string
}

func (r fakePgxRow) Scan(dest ...any) error {
	ptr := dest[0].(*string)
	*ptr = r.value
	return nil
}
