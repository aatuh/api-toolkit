package operationpostgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aatuh/api-toolkit/v3/httpx"
	"github.com/aatuh/api-toolkit/v3/operations"
	"github.com/aatuh/api-toolkit/v3/ports"
)

type operationResult struct {
	Message string `json:"message"`
}

func TestCreateOperationRequiresTenant(t *testing.T) {
	t.Parallel()

	store := New[operationResult](&fakeDBPool{conn: &fakeDBConnection{}}, Options{})
	err := store.CreateOperation(context.Background(), operations.Operation[operationResult]{ID: "op_1"})
	if !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("CreateOperation() error = %v, want %v", err, ErrTenantRequired)
	}
}

func TestCreateOperationInsertsTenantScopedRow(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	conn := &fakeDBConnection{}
	store := New[operationResult](&fakeDBPool{conn: conn}, Options{
		Table: "app.operations",
		Clock: func() time.Time { return now },
	})
	result := operationResult{Message: "done"}
	ctx := WithTenantID(context.Background(), "org_01")

	err := store.CreateOperation(ctx, operations.Operation[operationResult]{
		ID:     "op_1",
		State:  operations.StateSucceeded,
		Result: &result,
	})
	if err != nil {
		t.Fatalf("CreateOperation() error = %v", err)
	}
	if conn.releaseCount != 1 {
		t.Fatalf("Release() calls = %d, want 1", conn.releaseCount)
	}
	if len(conn.execCalls) != 1 {
		t.Fatalf("Exec() calls = %d, want 1", len(conn.execCalls))
	}
	call := conn.execCalls[0]
	if !strings.Contains(call.sql, `insert into "app"."operations"`) {
		t.Fatalf("Exec() SQL = %s", call.sql)
	}
	wantArgs := []any{"op_1", "org_01", "succeeded"}
	for i, want := range wantArgs {
		if got := call.args[i]; got != want {
			t.Fatalf("Exec() arg[%d] = %#v, want %#v", i, got, want)
		}
	}
	var stored operationResult
	if err := json.Unmarshal(call.args[3].([]byte), &stored); err != nil {
		t.Fatalf("result JSON: %v", err)
	}
	if stored.Message != "done" {
		t.Fatalf("stored result = %#v", stored)
	}
	if got, ok := call.args[4].([]byte); !ok || len(got) != 0 {
		t.Fatalf("problem arg = %#v, want empty []byte", call.args[4])
	}
	if got := call.args[5]; got != now {
		t.Fatalf("clock arg = %#v, want %#v", got, now)
	}
}

func TestGetOperationReturnsResultAndProblem(t *testing.T) {
	t.Parallel()

	resultJSON := []byte(`{"message":"done"}`)
	problemJSON := []byte(`{"type":"https://api-toolkit.dev/problems/conflict","title":"Conflict","detail":"already finished"}`)
	conn := &fakeDBConnection{
		row: scanFuncRow(func(dest ...any) error {
			*(dest[0].(*string)) = "failed"
			*(dest[1].(*[]byte)) = resultJSON
			*(dest[2].(*[]byte)) = problemJSON
			return nil
		}),
	}
	store := New[operationResult](&fakeDBPool{conn: conn}, Options{})
	ctx := WithTenantID(context.Background(), "org_01")

	got, ok, err := store.GetOperation(ctx, "op_1")
	if err != nil {
		t.Fatalf("GetOperation() error = %v", err)
	}
	if !ok {
		t.Fatal("GetOperation() ok = false, want true")
	}
	if got.ID != "op_1" || got.State != operations.StateFailed {
		t.Fatalf("operation = %#v", got)
	}
	if got.Result == nil || got.Result.Message != "done" {
		t.Fatalf("operation result = %#v", got.Result)
	}
	if got.Problem == nil || got.Problem.Detail != "already finished" {
		t.Fatalf("operation problem = %#v", got.Problem)
	}
	if conn.releaseCount != 1 {
		t.Fatalf("Release() calls = %d, want 1", conn.releaseCount)
	}
	if got := conn.queryRowArgs; len(got) != 2 || got[0] != "op_1" || got[1] != "org_01" {
		t.Fatalf("QueryRow() args = %#v", got)
	}
}

func TestGetOperationReturnsFalseForNoRows(t *testing.T) {
	t.Parallel()

	conn := &fakeDBConnection{row: scanFuncRow(func(...any) error { return pgx.ErrNoRows })}
	store := New[operationResult](&fakeDBPool{conn: conn}, Options{})
	ctx := WithTenantID(context.Background(), "org_01")

	_, ok, err := store.GetOperation(ctx, "missing")
	if err != nil {
		t.Fatalf("GetOperation() error = %v", err)
	}
	if ok {
		t.Fatal("GetOperation() ok = true, want false")
	}
}

func TestUpdateOperationReportsMissingRow(t *testing.T) {
	t.Parallel()

	conn := &fakeDBConnection{rowsAffected: 0}
	store := New[operationResult](&fakeDBPool{conn: conn}, Options{})
	ctx := WithTenantID(context.Background(), "org_01")
	err := store.UpdateOperation(ctx, operations.Operation[operationResult]{ID: "missing", State: operations.StateRunning})
	if !errors.Is(err, ErrOperationNotFound) {
		t.Fatalf("UpdateOperation() error = %v, want %v", err, ErrOperationNotFound)
	}
}

func TestUpdateOperationStoresProblem(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_100, 0).UTC()
	conn := &fakeDBConnection{rowsAffected: 1}
	store := New[operationResult](&fakeDBPool{conn: conn}, Options{Clock: func() time.Time { return now }})
	ctx := WithTenantID(context.Background(), "org_01")
	problem := &httpx.Problem{Title: "Conflict", Detail: "already finished"}

	err := store.UpdateOperation(ctx, operations.Operation[operationResult]{
		ID:      "op_1",
		State:   operations.StateFailed,
		Problem: problem,
	})
	if err != nil {
		t.Fatalf("UpdateOperation() error = %v", err)
	}
	call := conn.execCalls[0]
	if !strings.Contains(call.sql, `update "operations"`) {
		t.Fatalf("Exec() SQL = %s", call.sql)
	}
	if got, ok := call.args[3].([]byte); !ok || len(got) != 0 {
		t.Fatalf("result arg = %#v, want empty []byte", call.args[3])
	}
	var stored httpx.Problem
	if err := json.Unmarshal(call.args[4].([]byte), &stored); err != nil {
		t.Fatalf("problem JSON: %v", err)
	}
	if stored.Detail != "already finished" {
		t.Fatalf("stored problem = %#v", stored)
	}
	if got := call.args[5]; got != now {
		t.Fatalf("updated_at arg = %#v, want %#v", got, now)
	}
}

func TestInvalidTableRejected(t *testing.T) {
	t.Parallel()

	store := New[operationResult](&fakeDBPool{conn: &fakeDBConnection{}}, Options{Table: "operations;drop"})
	ctx := WithTenantID(context.Background(), "org_01")
	err := store.CreateOperation(ctx, operations.Operation[operationResult]{ID: "op_1"})
	if !errors.Is(err, ErrInvalidTable) {
		t.Fatalf("CreateOperation() error = %v, want %v", err, ErrInvalidTable)
	}
}

func TestHealthChecker(t *testing.T) {
	t.Parallel()

	store := New[operationResult](&fakeDBPool{}, Options{})
	result := store.HealthChecker().Check(context.Background())
	if result.Status != ports.HealthStatusHealthy || result.Message != "postgres operations healthy" {
		t.Fatalf("healthy result = %#v", result)
	}
	failed := New[operationResult](&fakeDBPool{pingErr: errors.New("down")}, Options{})
	result = failed.HealthChecker().Check(context.Background())
	if result.Status != ports.HealthStatusUnhealthy || !strings.Contains(result.Message, "postgres operations ping failed") {
		t.Fatalf("unhealthy result = %#v", result)
	}
	if result := (*Store[operationResult])(nil).HealthChecker().Check(context.Background()); result.Status != ports.HealthStatusUnhealthy {
		t.Fatalf("nil store result = %#v", result)
	}
}

type fakeDBPool struct {
	conn         ports.DatabaseConnection
	pingErr      error
	acquireCount int
}

func (p *fakeDBPool) Ping(context.Context) error { return p.pingErr }
func (p *fakeDBPool) Close()                     {}

func (p *fakeDBPool) Acquire(context.Context) (ports.DatabaseConnection, error) {
	p.acquireCount++
	return p.conn, nil
}

type execCall struct {
	sql  string
	args []any
}

type fakeDBConnection struct {
	execCalls    []execCall
	queryRowSQL  string
	queryRowArgs []any
	row          ports.DatabaseRow
	rowsAffected int64
	releaseCount int
}

func (c *fakeDBConnection) Query(context.Context, string, ...any) (ports.DatabaseRows, error) {
	return fakeDBRows{}, nil
}

func (c *fakeDBConnection) QueryRow(_ context.Context, sql string, args ...any) ports.DatabaseRow {
	c.queryRowSQL = sql
	c.queryRowArgs = append([]any(nil), args...)
	if c.row != nil {
		return c.row
	}
	return scanFuncRow(func(...any) error { return nil })
}

func (c *fakeDBConnection) Exec(_ context.Context, sql string, args ...any) (ports.DatabaseResult, error) {
	c.execCalls = append(c.execCalls, execCall{sql: sql, args: append([]any(nil), args...)})
	return fakeDBResult(c.rowsAffected), nil
}

func (c *fakeDBConnection) Begin(context.Context) (ports.DatabaseTransaction, error) {
	return nil, errors.New("transactions are not supported by fake connection")
}

func (c *fakeDBConnection) Release() {
	c.releaseCount++
}

type fakeDBRows struct{}

func (r fakeDBRows) Next() bool        { return false }
func (r fakeDBRows) Scan(...any) error { return nil }
func (r fakeDBRows) Close()            {}
func (r fakeDBRows) Err() error        { return nil }

type fakeDBResult int64

func (r fakeDBResult) RowsAffected() int64 { return int64(r) }

type scanFuncRow func(...any) error

func (r scanFuncRow) Scan(dest ...any) error { return r(dest...) }
