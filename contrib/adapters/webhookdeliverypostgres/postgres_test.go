package webhookdeliverypostgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/txpostgres"
	"github.com/aatuh/api-toolkit/contrib/v2/webhookdelivery"
	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestListEndpointsLoadsSubscribedTenantEndpointsAndSecrets(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	conn := &fakeDBConnection{
		rows: &fakeRows{values: [][]any{
			{"we_1", "org_1", "https://example.com/hooks", []string{"widget.created"}, false, now},
		}},
	}
	store := New(&fakeDBPool{conn: conn}, Options{
		EndpointTable: "app.webhook_endpoints",
		SecretResolver: SecretResolverFunc(func(ctx context.Context, tenantID, endpointID string) ([]byte, bool, error) {
			_ = ctx
			if tenantID == "org_1" && endpointID == "we_1" {
				return []byte("resolved-secret"), true, nil
			}
			return nil, false, nil
		}),
	})

	endpoints, err := store.ListEndpoints(context.Background(), "org_1", "widget.created")
	if err != nil {
		t.Fatalf("ListEndpoints() error = %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("endpoints len = %d, want 1", len(endpoints))
	}
	endpoint := endpoints[0]
	if endpoint.ID != "we_1" || endpoint.TenantID != "org_1" || endpoint.URL != "https://example.com/hooks" {
		t.Fatalf("endpoint = %#v", endpoint)
	}
	if string(endpoint.SigningSecret) != "resolved-secret" {
		t.Fatalf("SigningSecret = %q", endpoint.SigningSecret)
	}
	if conn.releaseCount != 1 {
		t.Fatalf("Release() calls = %d, want 1", conn.releaseCount)
	}
	if !strings.Contains(conn.querySQL, `from "app"."webhook_endpoints"`) ||
		!strings.Contains(conn.querySQL, `any(event_types)`) {
		t.Fatalf("ListEndpoints() SQL = %s", conn.querySQL)
	}
	if got := conn.queryArgs; len(got) != 3 || got[0] != "org_1" || got[1] != "widget.created" || got[2] != webhookdelivery.AnyEventType {
		t.Fatalf("ListEndpoints() args = %#v", got)
	}
}

func TestGetEndpointReturnsFalseForNoRows(t *testing.T) {
	t.Parallel()

	conn := &fakeDBConnection{row: scanFuncRow(func(...any) error { return pgx.ErrNoRows })}
	store := New(&fakeDBPool{conn: conn}, Options{SecretResolver: staticSecret("secret")})
	endpoint, ok, err := store.GetEndpoint(context.Background(), "org_1", "missing")
	if err != nil {
		t.Fatalf("GetEndpoint() error = %v", err)
	}
	if ok {
		t.Fatalf("GetEndpoint() ok = true endpoint=%#v, want false", endpoint)
	}
}

func TestGetEndpointRequiresSecretResolver(t *testing.T) {
	t.Parallel()

	conn := &fakeDBConnection{row: endpointRow("we_1", "org_1", "https://example.com/hooks", []string{"widget.created"}, false, time.Now())}
	store := New(&fakeDBPool{conn: conn}, Options{})
	_, _, err := store.GetEndpoint(context.Background(), "org_1", "we_1")
	if !errors.Is(err, ErrSecretResolverRequired) {
		t.Fatalf("GetEndpoint() error = %v, want %v", err, ErrSecretResolverRequired)
	}
}

func TestEnqueueDeliveryInsertsDeliveryAndOutboxInTransaction(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_100, 0).UTC()
	tx := &fakeDBTransaction{rowsAffected: 1}
	conn := &fakeDBConnection{tx: tx}
	store := New(&fakeDBPool{conn: conn}, Options{
		DeliveryTable: "app.webhook_deliveries",
		OutboxTable:   "app.outbox_events",
		Clock:         func() time.Time { return now },
	})
	event := webhookdelivery.Event{ID: "evt_1", TenantID: "org_1", Type: "widget.created", Payload: json.RawMessage(`{"id":"w_1"}`)}
	delivery := webhookdelivery.Delivery{
		ID:         "del_1",
		TenantID:   "org_1",
		EndpointID: "we_1",
		EventID:    "evt_1",
		EventType:  "widget.created",
		URL:        "https://example.com/hooks",
		State:      webhookdelivery.StatePending,
		NextAt:     now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	job := webhookdelivery.JobPayload{DeliveryID: "del_1", EndpointID: "we_1", Event: event}

	if err := store.EnqueueDelivery(context.Background(), delivery, job); err != nil {
		t.Fatalf("EnqueueDelivery() error = %v", err)
	}
	if conn.beginCount != 1 || tx.commitCount != 1 || tx.rollbackCount != 1 || conn.releaseCount != 1 {
		t.Fatalf("transaction counts begin=%d commit=%d rollback=%d release=%d", conn.beginCount, tx.commitCount, tx.rollbackCount, conn.releaseCount)
	}
	if len(tx.execCalls) != 2 {
		t.Fatalf("Exec() calls = %d, want 2", len(tx.execCalls))
	}
	insertDelivery := tx.execCalls[0]
	if !strings.Contains(insertDelivery.sql, `insert into "app"."webhook_deliveries"`) ||
		!strings.Contains(insertDelivery.sql, "event_id") ||
		!strings.Contains(insertDelivery.sql, "last_status_code") {
		t.Fatalf("delivery SQL = %s", insertDelivery.sql)
	}
	if got := insertDelivery.args[:6]; got[0] != "del_1" || got[1] != "org_1" || got[2] != "we_1" || got[3] != "evt_1" || got[4] != "widget.created" || string(got[5].([]byte)) != `{"id":"w_1"}` {
		t.Fatalf("delivery args = %#v", insertDelivery.args)
	}
	insertOutbox := tx.execCalls[1]
	if !strings.Contains(insertOutbox.sql, `insert into "app"."outbox_events"`) {
		t.Fatalf("outbox SQL = %s", insertOutbox.sql)
	}
	if got := insertOutbox.args; got[0] != "del_1" || got[1] != "org_1" || got[2] != OutboxEventType || got[4] != now {
		t.Fatalf("outbox args = %#v", got)
	}
	var storedJob webhookdelivery.JobPayload
	if err := json.Unmarshal(insertOutbox.args[3].([]byte), &storedJob); err != nil {
		t.Fatalf("outbox payload JSON: %v", err)
	}
	if storedJob.EndpointID != "we_1" || storedJob.Event.ID != "evt_1" {
		t.Fatalf("stored job = %#v", storedJob)
	}
}

func TestEnqueueDeliveryRejectsTenantMismatch(t *testing.T) {
	t.Parallel()

	store := New(&fakeDBPool{conn: &fakeDBConnection{}}, Options{})
	err := store.EnqueueDelivery(context.Background(), webhookdelivery.Delivery{
		ID:         "del_1",
		TenantID:   "org_1",
		EndpointID: "we_1",
		EventID:    "evt_1",
		EventType:  "widget.created",
		URL:        "https://example.com/hooks",
		State:      webhookdelivery.StatePending,
	}, webhookdelivery.JobPayload{
		DeliveryID: "del_1",
		EndpointID: "we_1",
		Event:      webhookdelivery.Event{ID: "evt_1", TenantID: "org_2", Type: "widget.created", Payload: json.RawMessage(`{"id":"w_1"}`)},
	})
	if !errors.Is(err, webhookdelivery.ErrTenantMismatch) {
		t.Fatalf("EnqueueDelivery() error = %v, want %v", err, webhookdelivery.ErrTenantMismatch)
	}
}

func TestRecordAttemptUpdatesSanitizedDeliveryState(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_200, 0).UTC()
	conn := &fakeDBConnection{rowsAffected: 1}
	store := New(&fakeDBPool{conn: conn}, Options{
		DeliveryTable: "app.webhook_deliveries",
		Clock:         func() time.Time { return now },
	})
	err := store.RecordAttempt(context.Background(), webhookdelivery.AttemptResult{
		DeliveryID:  "del_1",
		TenantID:    "org_1",
		EndpointID:  "we_1",
		EventID:     "evt_1",
		EventType:   "widget.created",
		Attempt:     2,
		StatusCode:  500,
		StatusClass: "5xx",
		Retryable:   true,
		Error:       "line one\nsecret=should-be-truncated-away",
	})
	if err != nil {
		t.Fatalf("RecordAttempt() error = %v", err)
	}
	call := conn.execCalls[0]
	if !strings.Contains(call.sql, `update "app"."webhook_deliveries"`) ||
		!strings.Contains(call.sql, "last_status_code") {
		t.Fatalf("RecordAttempt() SQL = %s", call.sql)
	}
	if got := call.args; got[0] != "del_1" || got[1] != "org_1" || got[2] != "we_1" || got[3] != 2 || got[4] != "failed" || got[5] != 500 || got[7] != now {
		t.Fatalf("RecordAttempt() args = %#v", got)
	}
	if strings.Contains(call.args[6].(string), "\n") || strings.Contains(call.args[6].(string), "secret=") {
		t.Fatalf("unsafe error persisted: %q", call.args[6])
	}
}

func TestRecordAttemptReportsMissingDelivery(t *testing.T) {
	t.Parallel()

	store := New(&fakeDBPool{conn: &fakeDBConnection{rowsAffected: 0}}, Options{})
	err := store.RecordAttempt(context.Background(), webhookdelivery.AttemptResult{DeliveryID: "missing", TenantID: "org_1", EndpointID: "we_1", EventID: "evt_1", EventType: "widget.created", Attempt: 1})
	if !errors.Is(err, ErrDeliveryNotFound) {
		t.Fatalf("RecordAttempt() error = %v, want %v", err, ErrDeliveryNotFound)
	}
}

func TestReplayDeliverySchedulesTenantScopedReplay(t *testing.T) {
	t.Parallel()

	when := time.Unix(1_700_000_300, 0).UTC()
	conn := &fakeDBConnection{rowsAffected: 1}
	store := New(&fakeDBPool{conn: conn}, Options{DeliveryTable: "app.webhook_deliveries"})
	if err := store.ReplayDelivery(context.Background(), "org_1", "del_1", when); err != nil {
		t.Fatalf("ReplayDelivery() error = %v", err)
	}
	call := conn.execCalls[0]
	if !strings.Contains(call.sql, `update "app"."webhook_deliveries"`) ||
		!strings.Contains(call.sql, `state='pending'`) {
		t.Fatalf("ReplayDelivery() SQL = %s", call.sql)
	}
	if got := call.args; got[0] != "org_1" || got[1] != "del_1" || got[2] != when {
		t.Fatalf("ReplayDelivery() args = %#v", got)
	}
}

func TestInvalidTableRejected(t *testing.T) {
	t.Parallel()

	store := New(&fakeDBPool{conn: &fakeDBConnection{}}, Options{EndpointTable: "webhook_endpoints;drop", SecretResolver: staticSecret("secret")})
	_, err := store.ListEndpoints(context.Background(), "org_1", "widget.created")
	if !errors.Is(err, ErrInvalidTable) {
		t.Fatalf("ListEndpoints() error = %v, want %v", err, ErrInvalidTable)
	}
}

func TestStoreUsesTransactionFromContextForRecordAttempt(t *testing.T) {
	t.Parallel()

	tx := &fakeDBTransaction{rowsAffected: 1}
	conn := &fakeDBConnection{tx: tx}
	pool := &fakeDBPool{conn: conn}
	store := New(pool, Options{})
	manager := txpostgres.New(pool)

	err := manager.WithinTx(context.Background(), func(ctx context.Context) error {
		return store.RecordAttempt(ctx, webhookdelivery.AttemptResult{DeliveryID: "del_1", TenantID: "org_1", EndpointID: "we_1", EventID: "evt_1", EventType: "widget.created", Attempt: 1, Accepted: true, StatusCode: 204})
	})
	if err != nil {
		t.Fatalf("WithinTx() error = %v", err)
	}
	if len(conn.execCalls) != 0 {
		t.Fatalf("pooled Exec() calls = %d, want 0", len(conn.execCalls))
	}
	if len(tx.execCalls) != 1 {
		t.Fatalf("tx Exec() calls = %d, want 1", len(tx.execCalls))
	}
}

func endpointRow(id, tenantID, url string, events []string, disabled bool, createdAt time.Time) ports.DatabaseRow {
	return scanFuncRow(func(dest ...any) error {
		*(dest[0].(*string)) = id
		*(dest[1].(*string)) = tenantID
		*(dest[2].(*string)) = url
		*(dest[3].(*[]string)) = append([]string(nil), events...)
		*(dest[4].(*bool)) = disabled
		*(dest[5].(*time.Time)) = createdAt
		return nil
	})
}

func staticSecret(secret string) SecretResolver {
	return SecretResolverFunc(func(context.Context, string, string) ([]byte, bool, error) {
		return []byte(secret), true, nil
	})
}

type fakeDBPool struct {
	conn ports.DatabaseConnection
}

func (p *fakeDBPool) Ping(context.Context) error { return nil }
func (p *fakeDBPool) Close()                     {}
func (p *fakeDBPool) Stat() ports.DatabaseStats  { return nil }

func (p *fakeDBPool) Acquire(context.Context) (ports.DatabaseConnection, error) {
	return p.conn, nil
}

type execCall struct {
	sql  string
	args []any
}

type fakeDBConnection struct {
	execCalls    []execCall
	querySQL     string
	queryArgs    []any
	row          ports.DatabaseRow
	rows         *fakeRows
	tx           ports.DatabaseTransaction
	rowsAffected int64
	beginCount   int
	releaseCount int
}

func (c *fakeDBConnection) Query(_ context.Context, sql string, args ...any) (ports.DatabaseRows, error) {
	c.querySQL = sql
	c.queryArgs = append([]any(nil), args...)
	if c.rows != nil {
		return c.rows, nil
	}
	return &fakeRows{}, nil
}

func (c *fakeDBConnection) QueryRow(_ context.Context, sql string, args ...any) ports.DatabaseRow {
	c.querySQL = sql
	c.queryArgs = append([]any(nil), args...)
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
	c.beginCount++
	if c.tx == nil {
		return nil, errors.New("transactions are not supported by fake connection")
	}
	return c.tx, nil
}

func (c *fakeDBConnection) Release() {
	c.releaseCount++
}

type fakeDBTransaction struct {
	execCalls     []execCall
	rowsAffected  int64
	commitCount   int
	rollbackCount int
}

func (t *fakeDBTransaction) Query(context.Context, string, ...any) (ports.DatabaseRows, error) {
	return &fakeRows{}, nil
}

func (t *fakeDBTransaction) QueryRow(context.Context, string, ...any) ports.DatabaseRow {
	return scanFuncRow(func(...any) error { return nil })
}

func (t *fakeDBTransaction) Exec(_ context.Context, sql string, args ...any) (ports.DatabaseResult, error) {
	t.execCalls = append(t.execCalls, execCall{sql: sql, args: append([]any(nil), args...)})
	return fakeDBResult(t.rowsAffected), nil
}

func (t *fakeDBTransaction) Commit(context.Context) error {
	t.commitCount++
	return nil
}

func (t *fakeDBTransaction) Rollback(context.Context) error {
	t.rollbackCount++
	return nil
}

type fakeRows struct {
	values [][]any
	index  int
	closed bool
}

func (r *fakeRows) Next() bool {
	return r.index < len(r.values)
}

func (r *fakeRows) Scan(dest ...any) error {
	row := r.values[r.index]
	r.index++
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = row[i].(string)
		case *[]string:
			*d = append([]string(nil), row[i].([]string)...)
		case *bool:
			*d = row[i].(bool)
		case *time.Time:
			*d = row[i].(time.Time)
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}

func (r *fakeRows) Close() {
	r.closed = true
}

func (r *fakeRows) Err() error {
	return nil
}

type fakeDBResult int64

func (r fakeDBResult) RowsAffected() int64 { return int64(r) }

type scanFuncRow func(...any) error

func (r scanFuncRow) Scan(dest ...any) error { return r(dest...) }
