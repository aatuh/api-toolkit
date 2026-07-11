package webhookdeliverypostgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aatuh/api-toolkit/contrib/v4/adapters/txpostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/contracts"
	"github.com/aatuh/api-toolkit/contrib/v4/webhookdelivery"
	"github.com/aatuh/api-toolkit/v4/endpoints/health"
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

func TestListEndpointsValidationAndErrorBranches(t *testing.T) {
	t.Parallel()

	if _, err := (*Store)(nil).ListEndpoints(context.Background(), "org_1", "widget.created"); !errors.Is(err, ErrStoreNotConfigured) {
		t.Fatalf("nil ListEndpoints() error = %v, want %v", err, ErrStoreNotConfigured)
	}
	store := New(&fakeDBPool{conn: &fakeDBConnection{}}, Options{})
	if _, err := store.ListEndpoints(context.Background(), "", "widget.created"); !errors.Is(err, webhookdelivery.ErrInvalidEvent) {
		t.Fatalf("ListEndpoints(empty tenant) error = %v, want %v", err, webhookdelivery.ErrInvalidEvent)
	}
	if _, err := store.ListEndpoints(context.Background(), "org_1", "*"); !errors.Is(err, webhookdelivery.ErrInvalidEvent) {
		t.Fatalf("ListEndpoints(wildcard) error = %v, want %v", err, webhookdelivery.ErrInvalidEvent)
	}
	queryErr := errors.New("query failed")
	store = New(&fakeDBPool{conn: &fakeDBConnection{queryErr: queryErr}}, Options{})
	if _, err := store.ListEndpoints(context.Background(), "org_1", "widget.created"); !errors.Is(err, queryErr) {
		t.Fatalf("ListEndpoints(query error) = %v, want %v", err, queryErr)
	}
	rowsErr := errors.New("rows failed")
	store = New(&fakeDBPool{conn: &fakeDBConnection{rows: &fakeRows{err: rowsErr}}}, Options{SecretResolver: staticSecret()})
	if _, err := store.ListEndpoints(context.Background(), "org_1", "widget.created"); !errors.Is(err, rowsErr) {
		t.Fatalf("ListEndpoints(rows error) = %v, want %v", err, rowsErr)
	}
	store = New(&fakeDBPool{conn: &fakeDBConnection{rows: &fakeRows{values: [][]any{{"we_1", "org_1", "https://example.com/hooks", []string{"widget.created"}, false, time.Now()}}}}}, Options{})
	if _, err := store.ListEndpoints(context.Background(), "org_1", "widget.created"); !errors.Is(err, ErrSecretResolverRequired) {
		t.Fatalf("ListEndpoints(secret resolver) = %v, want %v", err, ErrSecretResolverRequired)
	}
	store = New(&fakeDBPool{conn: &fakeDBConnection{rows: &fakeRows{values: [][]any{{"we_1", "org_1", "https://example.com/hooks", []string{"widget.created"}, false, time.Now()}}}}}, Options{SecretResolver: missingSecret()})
	if _, err := store.ListEndpoints(context.Background(), "org_1", "widget.created"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("ListEndpoints(secret missing) = %v, want %v", err, ErrSecretNotFound)
	}
}

func TestGetEndpointReturnsFalseForNoRows(t *testing.T) {
	t.Parallel()

	conn := &fakeDBConnection{row: scanFuncRow(func(...any) error { return pgx.ErrNoRows })}
	store := New(&fakeDBPool{conn: conn}, Options{SecretResolver: staticSecret()})
	endpoint, ok, err := store.GetEndpoint(context.Background(), "org_1", "missing")
	if err != nil {
		t.Fatalf("GetEndpoint() error = %v", err)
	}
	if ok {
		t.Fatalf("GetEndpoint() ok = true endpoint=%#v, want false", endpoint)
	}
}

func TestGetEndpointValidationAndSecretErrors(t *testing.T) {
	t.Parallel()

	if _, _, err := (*Store)(nil).GetEndpoint(context.Background(), "org_1", "we_1"); !errors.Is(err, ErrStoreNotConfigured) {
		t.Fatalf("nil GetEndpoint() error = %v, want %v", err, ErrStoreNotConfigured)
	}
	store := New(&fakeDBPool{conn: &fakeDBConnection{}}, Options{})
	if _, _, err := store.GetEndpoint(context.Background(), "", "we_1"); !errors.Is(err, webhookdelivery.ErrInvalidEndpoint) {
		t.Fatalf("GetEndpoint(empty tenant) error = %v, want %v", err, webhookdelivery.ErrInvalidEndpoint)
	}
	store = New(&fakeDBPool{conn: &fakeDBConnection{}}, Options{EndpointTable: "bad;table"})
	if _, _, err := store.GetEndpoint(context.Background(), "org_1", "we_1"); !errors.Is(err, ErrInvalidTable) {
		t.Fatalf("GetEndpoint(invalid table) error = %v, want %v", err, ErrInvalidTable)
	}
	rowErr := errors.New("scan failed")
	store = New(&fakeDBPool{conn: &fakeDBConnection{row: scanFuncRow(func(...any) error { return rowErr })}}, Options{SecretResolver: staticSecret()})
	if _, _, err := store.GetEndpoint(context.Background(), "org_1", "we_1"); !errors.Is(err, rowErr) {
		t.Fatalf("GetEndpoint(scan error) = %v, want %v", err, rowErr)
	}
	resolveErr := errors.New("secret backend down")
	store = New(&fakeDBPool{conn: &fakeDBConnection{row: endpointRow("https://example.com/hooks")}}, Options{
		SecretResolver: SecretResolverFunc(func(context.Context, string, string) ([]byte, bool, error) { return nil, false, resolveErr }),
	})
	if _, _, err := store.GetEndpoint(context.Background(), "org_1", "we_1"); !errors.Is(err, resolveErr) {
		t.Fatalf("GetEndpoint(secret error) = %v, want %v", err, resolveErr)
	}
	store = New(&fakeDBPool{conn: &fakeDBConnection{row: endpointRow("https://example.com/hooks")}}, Options{SecretResolver: missingSecret()})
	if _, _, err := store.GetEndpoint(context.Background(), "org_1", "we_1"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("GetEndpoint(secret missing) = %v, want %v", err, ErrSecretNotFound)
	}
}

func TestGetEndpointRequiresSecretResolver(t *testing.T) {
	t.Parallel()

	conn := &fakeDBConnection{row: endpointRow("https://example.com/hooks")}
	store := New(&fakeDBPool{conn: conn}, Options{})
	_, _, err := store.GetEndpoint(context.Background(), "org_1", "we_1")
	if !errors.Is(err, ErrSecretResolverRequired) {
		t.Fatalf("GetEndpoint() error = %v, want %v", err, ErrSecretResolverRequired)
	}
}

func TestSecretResolverFuncNilFailsClosed(t *testing.T) {
	t.Parallel()

	_, ok, err := (SecretResolverFunc)(nil).ResolveSigningSecret(context.Background(), "org_1", "we_1")
	if ok || !errors.Is(err, ErrSecretResolverRequired) {
		t.Fatalf("ResolveSigningSecret(nil) ok=%v err=%v, want %v", ok, err, ErrSecretResolverRequired)
	}
}

func TestGetEndpointUsesConfiguredEndpointPolicy(t *testing.T) {
	t.Parallel()

	conn := &fakeDBConnection{row: endpointRow("http://127.0.0.1:18081/hooks")}
	store := New(&fakeDBPool{conn: conn}, Options{
		EndpointPolicy: webhookdelivery.EndpointPolicy{AllowInsecureHTTP: true},
		SecretResolver: staticSecret(),
	})

	endpoint, ok, err := store.GetEndpoint(context.Background(), "org_1", "we_1")
	if err != nil {
		t.Fatalf("GetEndpoint() error = %v", err)
	}
	if !ok || endpoint.URL != "http://127.0.0.1:18081/hooks" {
		t.Fatalf("GetEndpoint() = endpoint=%#v ok=%v", endpoint, ok)
	}
}

func TestEnqueueDeliveryDefaultsAndFailureBranches(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_500, 0).UTC()
	event := webhookdelivery.Event{ID: "evt_1", TenantID: "org_1", Type: "widget.created", Payload: json.RawMessage(`{"id":"w_1"}`)}
	delivery := webhookdelivery.Delivery{
		ID:         "del_1",
		TenantID:   "org_1",
		EndpointID: "we_1",
		EventID:    "evt_1",
		EventType:  "widget.created",
		URL:        "https://example.com/hooks",
	}
	job := webhookdelivery.JobPayload{DeliveryID: "del_1", EndpointID: "we_1", Event: event}

	if err := (*Store)(nil).EnqueueDelivery(context.Background(), delivery, job); !errors.Is(err, ErrStoreNotConfigured) {
		t.Fatalf("nil EnqueueDelivery() error = %v, want %v", err, ErrStoreNotConfigured)
	}
	tx := &fakeDBTransaction{rowsAffected: 1}
	conn := &fakeDBConnection{tx: tx}
	store := New(&fakeDBPool{conn: conn}, Options{Clock: func() time.Time { return now }})
	if err := store.EnqueueDelivery(context.Background(), delivery, job); err != nil {
		t.Fatalf("EnqueueDelivery() error = %v", err)
	}
	args := tx.execCalls[0].args
	if args[6] != string(webhookdelivery.StatePending) || args[8] != now || args[10] != now {
		t.Fatalf("defaulted delivery args = %#v", args)
	}
	store = New(&fakeDBPool{conn: &fakeDBConnection{tx: &fakeDBTransaction{}}}, Options{DeliveryTable: "bad-table;drop"})
	if err := store.EnqueueDelivery(context.Background(), delivery, job); !errors.Is(err, ErrInvalidTable) {
		t.Fatalf("EnqueueDelivery(invalid table) error = %v, want %v", err, ErrInvalidTable)
	}
	store = New(&fakeDBPool{conn: &fakeDBConnection{}}, Options{})
	if err := store.EnqueueDelivery(context.Background(), delivery, job); err == nil || !strings.Contains(err.Error(), "transactions are not supported") {
		t.Fatalf("EnqueueDelivery(begin failure) error = %v", err)
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

func TestRecordAttemptStateMappingAndValidation(t *testing.T) {
	t.Parallel()

	if err := (*Store)(nil).RecordAttempt(context.Background(), webhookdelivery.AttemptResult{}); !errors.Is(err, ErrStoreNotConfigured) {
		t.Fatalf("nil RecordAttempt() error = %v, want %v", err, ErrStoreNotConfigured)
	}
	store := New(&fakeDBPool{conn: &fakeDBConnection{}}, Options{})
	if err := store.RecordAttempt(context.Background(), webhookdelivery.AttemptResult{DeliveryID: "del_1", TenantID: "org_1", EndpointID: "we_1", EventID: "evt_1", EventType: "bad event", Attempt: 1}); !errors.Is(err, webhookdelivery.ErrInvalidDelivery) {
		t.Fatalf("RecordAttempt(invalid) error = %v, want %v", err, webhookdelivery.ErrInvalidDelivery)
	}
	store = New(&fakeDBPool{conn: &fakeDBConnection{}}, Options{DeliveryTable: "bad;table"})
	if err := store.RecordAttempt(context.Background(), webhookdelivery.AttemptResult{DeliveryID: "del_1", TenantID: "org_1", EndpointID: "we_1", EventID: "evt_1", EventType: "widget.created", Attempt: 1}); !errors.Is(err, ErrInvalidTable) {
		t.Fatalf("RecordAttempt(invalid table) error = %v, want %v", err, ErrInvalidTable)
	}

	tests := []struct {
		name   string
		result webhookdelivery.AttemptResult
		state  string
	}{
		{name: "accepted", result: webhookdelivery.AttemptResult{Accepted: true, StatusCode: 204}, state: "succeeded"},
		{name: "non retryable", result: webhookdelivery.AttemptResult{Accepted: false, Retryable: false, StatusCode: 400}, state: "dead_letter"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			conn := &fakeDBConnection{rowsAffected: 1}
			store := New(&fakeDBPool{conn: conn}, Options{})
			result := tt.result
			result.DeliveryID = "del_1"
			result.TenantID = "org_1"
			result.EndpointID = "we_1"
			result.EventID = "evt_1"
			result.EventType = "widget.created"
			result.Attempt = 1
			if err := store.RecordAttempt(context.Background(), result); err != nil {
				t.Fatalf("RecordAttempt() error = %v", err)
			}
			if got := conn.execCalls[0].args[4]; got != tt.state {
				t.Fatalf("state arg = %v, want %s", got, tt.state)
			}
		})
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

func TestReplayDeliveryValidationMissingAndDefaultTime(t *testing.T) {
	t.Parallel()

	if err := (*Store)(nil).ReplayDelivery(context.Background(), "org_1", "del_1", time.Time{}); !errors.Is(err, ErrStoreNotConfigured) {
		t.Fatalf("nil ReplayDelivery() error = %v, want %v", err, ErrStoreNotConfigured)
	}
	store := New(&fakeDBPool{conn: &fakeDBConnection{}}, Options{})
	if err := store.ReplayDelivery(context.Background(), "", "del_1", time.Time{}); !errors.Is(err, webhookdelivery.ErrInvalidDelivery) {
		t.Fatalf("ReplayDelivery(empty tenant) error = %v, want %v", err, webhookdelivery.ErrInvalidDelivery)
	}
	store = New(&fakeDBPool{conn: &fakeDBConnection{}}, Options{DeliveryTable: "bad;table"})
	if err := store.ReplayDelivery(context.Background(), "org_1", "del_1", time.Time{}); !errors.Is(err, ErrInvalidTable) {
		t.Fatalf("ReplayDelivery(invalid table) error = %v, want %v", err, ErrInvalidTable)
	}
	store = New(&fakeDBPool{conn: &fakeDBConnection{rowsAffected: 0}}, Options{})
	if err := store.ReplayDelivery(context.Background(), "org_1", "missing", time.Now()); !errors.Is(err, ErrDeliveryNotFound) {
		t.Fatalf("ReplayDelivery(missing) error = %v, want %v", err, ErrDeliveryNotFound)
	}

	now := time.Unix(1_700_000_600, 0).UTC()
	conn := &fakeDBConnection{rowsAffected: 1}
	store = New(&fakeDBPool{conn: conn}, Options{Clock: func() time.Time { return now }})
	if err := store.ReplayDelivery(context.Background(), "org_1", "del_1", time.Time{}); err != nil {
		t.Fatalf("ReplayDelivery(default time) error = %v", err)
	}
	if got := conn.execCalls[0].args[2]; got != now {
		t.Fatalf("ReplayDelivery nextAt = %v, want %v", got, now)
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

	store := New(&fakeDBPool{conn: &fakeDBConnection{}}, Options{EndpointTable: "webhook_endpoints;drop", SecretResolver: staticSecret()})
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

func TestHealthChecker(t *testing.T) {
	t.Parallel()

	store := New(&fakeDBPool{}, Options{})
	result := store.HealthChecker().Check(context.Background())
	if result.Status != health.StatusHealthy || result.Message != "postgres webhook delivery healthy" {
		t.Fatalf("healthy result = %#v", result)
	}
	failed := New(&fakeDBPool{pingErr: errors.New("down")}, Options{})
	result = failed.HealthChecker().Check(context.Background())
	if result.Status != health.StatusUnhealthy || !strings.Contains(result.Message, "postgres webhook delivery ping failed") {
		t.Fatalf("unhealthy result = %#v", result)
	}
	if result := (*Store)(nil).HealthChecker().Check(context.Background()); result.Status != health.StatusUnhealthy {
		t.Fatalf("nil store result = %#v", result)
	}
}

func endpointRow(url string) contracts.DatabaseRow {
	return scanFuncRow(func(dest ...any) error {
		*(dest[0].(*string)) = "we_1"
		*(dest[1].(*string)) = "org_1"
		*(dest[2].(*string)) = url
		*(dest[3].(*[]string)) = []string{"widget.created"}
		*(dest[4].(*bool)) = false
		*(dest[5].(*time.Time)) = time.Now()
		return nil
	})
}

func staticSecret() SecretResolver {
	return SecretResolverFunc(func(context.Context, string, string) ([]byte, bool, error) {
		return []byte("secret"), true, nil
	})
}

func missingSecret() SecretResolver {
	return SecretResolverFunc(func(context.Context, string, string) ([]byte, bool, error) {
		return nil, false, nil
	})
}

type fakeDBPool struct {
	conn    contracts.DatabaseConnection
	pingErr error
}

func (p *fakeDBPool) Ping(context.Context) error { return p.pingErr }
func (p *fakeDBPool) Close()                     {}

func (p *fakeDBPool) Acquire(context.Context) (contracts.DatabaseConnection, error) {
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
	queryErr     error
	row          contracts.DatabaseRow
	rows         *fakeRows
	tx           contracts.DatabaseTransaction
	rowsAffected int64
	beginCount   int
	releaseCount int
}

func (c *fakeDBConnection) Query(_ context.Context, sql string, args ...any) (contracts.DatabaseRows, error) {
	c.querySQL = sql
	c.queryArgs = append([]any(nil), args...)
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	if c.rows != nil {
		return c.rows, nil
	}
	return &fakeRows{}, nil
}

func (c *fakeDBConnection) QueryRow(_ context.Context, sql string, args ...any) contracts.DatabaseRow {
	c.querySQL = sql
	c.queryArgs = append([]any(nil), args...)
	if c.row != nil {
		return c.row
	}
	return scanFuncRow(func(...any) error { return nil })
}

func (c *fakeDBConnection) Exec(_ context.Context, sql string, args ...any) (contracts.DatabaseResult, error) {
	c.execCalls = append(c.execCalls, execCall{sql: sql, args: append([]any(nil), args...)})
	return fakeDBResult(c.rowsAffected), nil
}

func (c *fakeDBConnection) Begin(context.Context) (contracts.DatabaseTransaction, error) {
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

func (t *fakeDBTransaction) Query(context.Context, string, ...any) (contracts.DatabaseRows, error) {
	return &fakeRows{}, nil
}

func (t *fakeDBTransaction) QueryRow(context.Context, string, ...any) contracts.DatabaseRow {
	return scanFuncRow(func(...any) error { return nil })
}

func (t *fakeDBTransaction) Exec(_ context.Context, sql string, args ...any) (contracts.DatabaseResult, error) {
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
	err    error
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
	return r.err
}

type fakeDBResult int64

func (r fakeDBResult) RowsAffected() int64 { return int64(r) }

type scanFuncRow func(...any) error

func (r scanFuncRow) Scan(dest ...any) error { return r(dest...) }
