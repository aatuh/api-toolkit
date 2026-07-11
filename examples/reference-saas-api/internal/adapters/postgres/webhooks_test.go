package postgres

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/adapters/webhookdeliverypostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/contracts"
	"github.com/aatuh/api-toolkit/contrib/v4/webhookdelivery"
)

func TestNewWebhookStoreRequiresSecretKey(t *testing.T) {
	if _, err := NewWebhookStore(&fakeWebhookPool{}, "short", webhookdelivery.EndpointPolicy{}); !errors.Is(err, ErrWebhookSecretKeyInvalid) {
		t.Fatalf("NewWebhookStore() error = %v, want %v", err, ErrWebhookSecretKeyInvalid)
	}
}

func TestWebhookStoreCreateEndpointEncryptsSecret(t *testing.T) {
	conn := &fakeWebhookConn{}
	store, err := NewWebhookStore(&fakeWebhookPool{conn: conn}, strings.Repeat("a", 32), webhookdelivery.EndpointPolicy{})
	if err != nil {
		t.Fatalf("NewWebhookStore() error = %v", err)
	}
	endpoint := webhookdelivery.Endpoint{
		ID:            "whend_1",
		TenantID:      "org_1",
		URL:           "https://example.com/webhooks",
		SigningSecret: []byte("webhook-secret-value"),
		Events:        []string{"widget.created"},
		CreatedAt:     time.Unix(1_700_000_000, 0).UTC(),
	}
	if err := store.CreateEndpoint(context.Background(), endpoint); err != nil {
		t.Fatalf("CreateEndpoint() error = %v", err)
	}
	if len(conn.execCalls) != 1 {
		t.Fatalf("Exec calls = %d, want 1", len(conn.execCalls))
	}
	args := conn.execCalls[0].args
	if got, _ := args[0].(string); got != "whend_1" {
		t.Fatalf("endpoint id arg = %#v", args[0])
	}
	for _, arg := range args {
		if bytes.Contains([]byte(strings.TrimSpace(toString(arg))), endpoint.SigningSecret) {
			t.Fatalf("raw webhook secret leaked into exec args: %#v", arg)
		}
	}
}

func TestWebhookStoreCreateEndpointUsesConfiguredEndpointPolicy(t *testing.T) {
	conn := &fakeWebhookConn{}
	store, err := NewWebhookStore(&fakeWebhookPool{conn: conn}, strings.Repeat("a", 32), webhookdelivery.EndpointPolicy{AllowInsecureHTTP: true})
	if err != nil {
		t.Fatalf("NewWebhookStore() error = %v", err)
	}
	err = store.CreateEndpoint(context.Background(), webhookdelivery.Endpoint{
		ID:            "whend_1",
		TenantID:      "org_1",
		URL:           "http://127.0.0.1:18081/webhooks",
		SigningSecret: []byte("webhook-secret-value"),
		Events:        []string{"widget.created"},
	})
	if err != nil {
		t.Fatalf("CreateEndpoint() error = %v", err)
	}
}

func TestWebhookStoreListDeliveriesScansTenantRows(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	conn := &fakeWebhookConn{rows: &fakeWebhookRows{values: [][]any{[]any{
		"whdel_1", "org_1", "whend_1", "evt_1", "widget.created", "https://example.com/webhooks", "pending", 0, now, 0, "", now, now,
	}}}}
	store, err := NewWebhookStore(&fakeWebhookPool{conn: conn}, strings.Repeat("a", 32), webhookdelivery.EndpointPolicy{})
	if err != nil {
		t.Fatalf("NewWebhookStore() error = %v", err)
	}
	deliveries, err := store.ListDeliveries(context.Background(), "org_1")
	if err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	if len(deliveries) != 1 || deliveries[0].ID != "whdel_1" || deliveries[0].URL == "" {
		t.Fatalf("deliveries = %#v", deliveries)
	}
}

func TestWebhookStoreReplayRequeuesOutbox(t *testing.T) {
	now := time.Unix(1_700_000_100, 0).UTC()
	conn := &fakeWebhookConn{row: fakeWebhookRowFunc(func(dest ...any) error {
		*(dest[0].(*string)) = "whend_1"
		*(dest[1].(*string)) = "evt_1"
		*(dest[2].(*string)) = "widget.created"
		*(dest[3].(*[]byte)) = []byte(`{"id":"wgt_1"}`)
		return nil
	})}
	tx := &fakeWebhookTx{rowsAffected: 1}
	conn.tx = tx
	store, err := NewWebhookStore(&fakeWebhookPool{conn: conn}, strings.Repeat("a", 32), webhookdelivery.EndpointPolicy{})
	if err != nil {
		t.Fatalf("NewWebhookStore() error = %v", err)
	}
	store.now = func() time.Time { return now }
	if err := store.ReplayDelivery(context.Background(), "org_1", "whdel_1", now); err != nil {
		t.Fatalf("ReplayDelivery() error = %v", err)
	}
	if len(tx.execCalls) != 2 {
		t.Fatalf("tx exec calls = %d, want 2", len(tx.execCalls))
	}
	if !strings.Contains(tx.execCalls[1].sql, "on conflict (id) do update") || tx.execCalls[1].args[2] != webhookdeliverypostgres.OutboxEventType {
		t.Fatalf("outbox replay call = %#v", tx.execCalls[1])
	}
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

type fakeWebhookPool struct {
	conn *fakeWebhookConn
}

func (p *fakeWebhookPool) Ping(context.Context) error { return nil }
func (p *fakeWebhookPool) Close()                     {}

func (p *fakeWebhookPool) Acquire(context.Context) (contracts.DatabaseConnection, error) {
	if p.conn == nil {
		p.conn = &fakeWebhookConn{}
	}
	return p.conn, nil
}

type fakeWebhookExecCall struct {
	sql  string
	args []any
}

type fakeWebhookConn struct {
	execCalls    []fakeWebhookExecCall
	row          contracts.DatabaseRow
	rows         contracts.DatabaseRows
	tx           contracts.DatabaseTransaction
	rowsAffected int64
}

func (c *fakeWebhookConn) Query(_ context.Context, _ string, _ ...any) (contracts.DatabaseRows, error) {
	if c.rows != nil {
		return c.rows, nil
	}
	return &fakeWebhookRows{}, nil
}

func (c *fakeWebhookConn) QueryRow(_ context.Context, _ string, _ ...any) contracts.DatabaseRow {
	if c.row != nil {
		return c.row
	}
	return fakeWebhookRowFunc(func(...any) error { return nil })
}

func (c *fakeWebhookConn) Exec(_ context.Context, sql string, args ...any) (contracts.DatabaseResult, error) {
	c.execCalls = append(c.execCalls, fakeWebhookExecCall{sql: sql, args: append([]any(nil), args...)})
	return fakeWebhookResult(c.rowsAffected), nil
}

func (c *fakeWebhookConn) Begin(context.Context) (contracts.DatabaseTransaction, error) {
	if c.tx == nil {
		return nil, errors.New("transaction not configured")
	}
	return c.tx, nil
}

func (c *fakeWebhookConn) Release() {}

type fakeWebhookTx struct {
	execCalls    []fakeWebhookExecCall
	rowsAffected int64
}

func (t *fakeWebhookTx) Query(context.Context, string, ...any) (contracts.DatabaseRows, error) {
	return &fakeWebhookRows{}, nil
}

func (t *fakeWebhookTx) QueryRow(context.Context, string, ...any) contracts.DatabaseRow {
	return fakeWebhookRowFunc(func(...any) error { return nil })
}

func (t *fakeWebhookTx) Exec(_ context.Context, sql string, args ...any) (contracts.DatabaseResult, error) {
	t.execCalls = append(t.execCalls, fakeWebhookExecCall{sql: sql, args: append([]any(nil), args...)})
	return fakeWebhookResult(t.rowsAffected), nil
}

func (t *fakeWebhookTx) Commit(context.Context) error   { return nil }
func (t *fakeWebhookTx) Rollback(context.Context) error { return nil }

type fakeWebhookRows struct {
	values [][]any
	index  int
}

func (r *fakeWebhookRows) Next() bool {
	return r.index < len(r.values)
}

func (r *fakeWebhookRows) Scan(dest ...any) error {
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
		case *int:
			*d = row[i].(int)
		case *time.Time:
			*d = row[i].(time.Time)
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}

func (r *fakeWebhookRows) Close()     {}
func (r *fakeWebhookRows) Err() error { return nil }

type fakeWebhookResult int64

func (r fakeWebhookResult) RowsAffected() int64 {
	if r == 0 {
		return 1
	}
	return int64(r)
}

type fakeWebhookRowFunc func(...any) error

func (r fakeWebhookRowFunc) Scan(dest ...any) error { return r(dest...) }
