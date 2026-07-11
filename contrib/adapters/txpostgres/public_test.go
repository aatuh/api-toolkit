package txpostgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/aatuh/api-toolkit/contrib/v3/contracts"
)

func TestFromCtxReturnsTransactionWhenPresent(t *testing.T) {
	tx := &recordingTx{}
	ctx := context.WithValue(context.Background(), txKey, tx)

	got := FromCtx(ctx, &recordingPool{})
	gotTx, ok := got.(*recordingTx)
	if !ok {
		t.Fatalf("FromCtx() type = %T, want *recordingTx", got)
	}
	if gotTx != tx {
		t.Fatal("expected transaction from context to be returned directly")
	}
}

func TestFromCtxExecUsesAndReleasesConnection(t *testing.T) {
	conn := &recordingConn{execResult: recordingResult(3)}
	pool := &recordingPool{conn: conn}

	got, err := FromCtx(context.Background(), pool).Exec(context.Background(), "select 1")
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if got.RowsAffected() != 3 {
		t.Fatalf("RowsAffected() = %d, want 3", got.RowsAffected())
	}
	if pool.acquireCount != 1 {
		t.Fatalf("Acquire() calls = %d, want 1", pool.acquireCount)
	}
	if conn.execCount != 1 {
		t.Fatalf("Exec() calls = %d, want 1", conn.execCount)
	}
	if conn.releaseCount != 1 {
		t.Fatalf("Release() calls = %d, want 1", conn.releaseCount)
	}
}

func TestNewNilPoolReturnsConfiguredErrorFromWithinTx(t *testing.T) {
	called := false
	err := New(nil).WithinTx(context.Background(), func(context.Context) error {
		called = true
		return nil
	})
	if called {
		t.Fatal("expected function not to be called")
	}
	if !errors.Is(err, ErrPoolNotConfigured) {
		t.Fatalf("WithinTx() error = %v, want %v", err, ErrPoolNotConfigured)
	}
}

func TestFromCtxExecReturnsConfiguredErrorWhenPoolMissing(t *testing.T) {
	_, err := FromCtx(context.Background(), nil).Exec(context.Background(), "select 1")
	if !errors.Is(err, ErrPoolNotConfigured) {
		t.Fatalf("Exec() error = %v, want %v", err, ErrPoolNotConfigured)
	}
}

func TestFromCtxQueryReturnsConfiguredErrorWhenPoolMissing(t *testing.T) {
	_, err := FromCtx(context.Background(), nil).Query(context.Background(), "select 1")
	if !errors.Is(err, ErrPoolNotConfigured) {
		t.Fatalf("Query() error = %v, want %v", err, ErrPoolNotConfigured)
	}
}

func TestFromCtxQueryRowReturnsConfiguredErrorWhenPoolMissing(t *testing.T) {
	err := FromCtx(context.Background(), nil).QueryRow(context.Background(), "select 1").Scan(new(string))
	if !errors.Is(err, ErrPoolNotConfigured) {
		t.Fatalf("Scan() error = %v, want %v", err, ErrPoolNotConfigured)
	}
}

func TestFromCtxQueryReleasesConnectionOnClose(t *testing.T) {
	rows := &recordingRows{}
	conn := &recordingConn{queryRows: rows}

	got, err := FromCtx(context.Background(), &recordingPool{conn: conn}).Query(context.Background(), "select 1")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if conn.releaseCount != 0 {
		t.Fatalf("Release() calls before Close = %d, want 0", conn.releaseCount)
	}

	got.Close()

	if rows.closeCount != 1 {
		t.Fatalf("rows Close() calls = %d, want 1", rows.closeCount)
	}
	if conn.releaseCount != 1 {
		t.Fatalf("Release() calls after Close = %d, want 1", conn.releaseCount)
	}
}

func TestFromCtxQueryReleasesConnectionWhenQueryFails(t *testing.T) {
	conn := &recordingConn{queryErr: errors.New("boom")}

	_, err := FromCtx(context.Background(), &recordingPool{conn: conn}).Query(context.Background(), "select 1")
	if err == nil {
		t.Fatal("expected query error")
	}
	if conn.releaseCount != 1 {
		t.Fatalf("Release() calls = %d, want 1", conn.releaseCount)
	}
}

func TestFromCtxQueryRowReleasesConnectionAfterScan(t *testing.T) {
	row := &recordingRow{}
	conn := &recordingConn{queryRow: row}

	got := FromCtx(context.Background(), &recordingPool{conn: conn}).QueryRow(context.Background(), "select 1")
	if conn.releaseCount != 0 {
		t.Fatalf("Release() calls before Scan = %d, want 0", conn.releaseCount)
	}

	if err := got.Scan(new(string)); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if row.scanCount != 1 {
		t.Fatalf("row Scan() calls = %d, want 1", row.scanCount)
	}
	if conn.releaseCount != 1 {
		t.Fatalf("Release() calls after Scan = %d, want 1", conn.releaseCount)
	}
}

func TestConvenienceHelpers(t *testing.T) {
	if !IsNoRows(pgx.ErrNoRows) {
		t.Fatal("expected pgx.ErrNoRows to be recognized")
	}
	if IsNoRows(errors.New("other")) {
		t.Fatal("did not expect unrelated error to be recognized")
	}

	want := &pgconn.PgError{Code: "23505"}
	got, ok := AsPgError(want)
	if !ok {
		t.Fatal("expected pg error cast to succeed")
	}
	if got != want {
		t.Fatalf("AsPgError() = %v, want %v", got, want)
	}
}

type recordingPool struct {
	conn         contracts.DatabaseConnection
	acquireErr   error
	acquireCount int
}

func (p *recordingPool) Ping(context.Context) error { return nil }
func (p *recordingPool) Close()                     {}

func (p *recordingPool) Acquire(context.Context) (contracts.DatabaseConnection, error) {
	p.acquireCount++
	if p.acquireErr != nil {
		return nil, p.acquireErr
	}
	return p.conn, nil
}

type recordingConn struct {
	queryRows     contracts.DatabaseRows
	queryRow      contracts.DatabaseRow
	queryErr      error
	execResult    contracts.DatabaseResult
	execErr       error
	execCount     int
	queryCount    int
	queryRowCount int
	releaseCount  int
}

func (c *recordingConn) Query(context.Context, string, ...any) (contracts.DatabaseRows, error) {
	c.queryCount++
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	return c.queryRows, nil
}

func (c *recordingConn) QueryRow(context.Context, string, ...any) contracts.DatabaseRow {
	c.queryRowCount++
	return c.queryRow
}

func (c *recordingConn) Exec(context.Context, string, ...any) (contracts.DatabaseResult, error) {
	c.execCount++
	return c.execResult, c.execErr
}

func (c *recordingConn) Begin(context.Context) (contracts.DatabaseTransaction, error) {
	return &recordingTx{}, nil
}

func (c *recordingConn) Release() {
	c.releaseCount++
}

type recordingTx struct{}

func (*recordingTx) Query(context.Context, string, ...any) (contracts.DatabaseRows, error) {
	return &recordingRows{}, nil
}

func (*recordingTx) QueryRow(context.Context, string, ...any) contracts.DatabaseRow {
	return &recordingRow{}
}

func (*recordingTx) Exec(context.Context, string, ...any) (contracts.DatabaseResult, error) {
	return recordingResult(0), nil
}

func (*recordingTx) Commit(context.Context) error   { return nil }
func (*recordingTx) Rollback(context.Context) error { return nil }

type recordingRows struct {
	closeCount int
}

func (*recordingRows) Next() bool        { return false }
func (*recordingRows) Scan(...any) error { return nil }
func (r *recordingRows) Close()          { r.closeCount++ }
func (*recordingRows) Err() error        { return nil }

type recordingRow struct {
	scanCount int
}

func (r *recordingRow) Scan(...any) error {
	r.scanCount++
	return nil
}

type recordingResult int64

func (r recordingResult) RowsAffected() int64 { return int64(r) }
