package postgres

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"example.com/reference-saas-api/internal/domain"
)

func TestAPIKeyStoreCreateStoresDecodedHash(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	hash := hex.EncodeToString([]byte("12345678901234567890123456789012"))
	db := &fakeAPIKeyDB{execTag: pgconn.NewCommandTag("INSERT 0 1")}
	store := NewAPIKeyStore(db)
	err := store.CreateAPIKey(context.Background(), domain.APIKey{ID: "key_1", OrganizationID: "org_1", Name: "CI", Prefix: "atk_prefix", Scopes: []string{"widgets:read"}, CreatedAt: now}, hash)
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if !strings.Contains(db.lastExecSQL, "insert into api_keys") {
		t.Fatalf("CreateAPIKey() SQL = %q", db.lastExecSQL)
	}
	hashArg, ok := db.lastExecArgs[4].([]byte)
	if !ok || string(hashArg) != "12345678901234567890123456789012" {
		t.Fatalf("hash arg = %#v", db.lastExecArgs[4])
	}
	if strings.Contains(fmt.Sprint(db.lastExecArgs...), "raw-secret") {
		t.Fatalf("exec args leaked raw secret: %#v", db.lastExecArgs)
	}
}

func TestAPIKeyStoreListAndGetByHashScanKeys(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	values := []any{"key_1", "org_1", "CI", "atk_prefix", []string{"widgets:read"}, pgtype.Timestamptz{}, pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{}, now}
	db := &fakeAPIKeyDB{
		rows: &fakeAPIKeyRows{rows: [][]any{values}},
		row:  fakeAPIKeyRow{values: values},
	}
	store := NewAPIKeyStore(db)
	keys, err := store.ListAPIKeys(context.Background(), "org_1")
	if err != nil {
		t.Fatalf("ListAPIKeys() error = %v", err)
	}
	if len(keys) != 1 || keys[0].ID != "key_1" || keys[0].LastUsedAt == nil {
		t.Fatalf("ListAPIKeys() = %#v", keys)
	}
	hash := hex.EncodeToString([]byte("12345678901234567890123456789012"))
	key, ok, err := store.GetAPIKeyByHash(context.Background(), hash)
	if err != nil || !ok || key.ID != "key_1" {
		t.Fatalf("GetAPIKeyByHash() key=%#v ok=%v err=%v", key, ok, err)
	}
}

func TestAPIKeyStoreRevokeAndTouchUseSafeUpdates(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	db := &fakeAPIKeyDB{execTag: pgconn.NewCommandTag("UPDATE 1")}
	store := NewAPIKeyStore(db)
	ok, err := store.RevokeAPIKey(context.Background(), "org_1", "key_1", now)
	if err != nil || !ok {
		t.Fatalf("RevokeAPIKey() ok=%v err=%v", ok, err)
	}
	if !strings.Contains(db.lastExecSQL, "revoked_at") {
		t.Fatalf("RevokeAPIKey() SQL = %q", db.lastExecSQL)
	}
	if err := store.TouchAPIKey(context.Background(), "key_1", now); err != nil {
		t.Fatalf("TouchAPIKey() error = %v", err)
	}
	if !strings.Contains(db.lastExecSQL, "last_used_at") {
		t.Fatalf("TouchAPIKey() SQL = %q", db.lastExecSQL)
	}
}

func TestAPIKeyStoreRequiresDBAndValidHash(t *testing.T) {
	if err := (*APIKeyStore)(nil).CreateAPIKey(context.Background(), domain.APIKey{}, strings.Repeat("0", 64)); !errors.Is(err, ErrAPIKeyStoreRequired) {
		t.Fatalf("nil CreateAPIKey() error = %v, want %v", err, ErrAPIKeyStoreRequired)
	}
	store := NewAPIKeyStore(&fakeAPIKeyDB{})
	if err := store.CreateAPIKey(context.Background(), domain.APIKey{ID: "key_1", OrganizationID: "org_1", Name: "CI", Prefix: "atk", Scopes: []string{"widgets:read"}, CreatedAt: time.Now()}, "not-hex"); !errors.Is(err, ErrAPIKeyInvalid) {
		t.Fatalf("invalid hash CreateAPIKey() error = %v, want %v", err, ErrAPIKeyInvalid)
	}
}

type fakeAPIKeyDB struct {
	rows          pgx.Rows
	row           pgx.Row
	execTag       pgconn.CommandTag
	execErr       error
	lastQuerySQL  string
	lastQueryArgs []any
	lastRowSQL    string
	lastRowArgs   []any
	lastExecSQL   string
	lastExecArgs  []any
}

func (f *fakeAPIKeyDB) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.lastQuerySQL = sql
	f.lastQueryArgs = append([]any(nil), args...)
	if f.rows == nil {
		return &fakeAPIKeyRows{}, nil
	}
	return f.rows, nil
}

func (f *fakeAPIKeyDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	f.lastRowSQL = sql
	f.lastRowArgs = append([]any(nil), args...)
	if f.row == nil {
		return fakeAPIKeyRow{err: pgx.ErrNoRows}
	}
	return f.row
}

func (f *fakeAPIKeyDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.lastExecSQL = sql
	f.lastExecArgs = append([]any(nil), args...)
	return f.execTag, f.execErr
}

type fakeAPIKeyRows struct {
	rows [][]any
	idx  int
	err  error
}

func (r *fakeAPIKeyRows) Close()                                       {}
func (r *fakeAPIKeyRows) Err() error                                   { return r.err }
func (r *fakeAPIKeyRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeAPIKeyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeAPIKeyRows) Values() ([]any, error)                       { return r.rows[r.idx-1], nil }
func (r *fakeAPIKeyRows) RawValues() [][]byte                          { return nil }
func (r *fakeAPIKeyRows) Conn() *pgx.Conn                              { return nil }

func (r *fakeAPIKeyRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeAPIKeyRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("Scan called without current row")
	}
	return scanFakeAPIKeyValues(r.rows[r.idx-1], dest...)
}

type fakeAPIKeyRow struct {
	values []any
	err    error
}

func (r fakeAPIKeyRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return scanFakeAPIKeyValues(r.values, dest...)
}

func scanFakeAPIKeyValues(values []any, dest ...any) error {
	if len(values) != len(dest) {
		return fmt.Errorf("value count %d does not match destination count %d", len(values), len(dest))
	}
	for i := range values {
		switch d := dest[i].(type) {
		case *string:
			value, ok := values[i].(string)
			if !ok {
				return fmt.Errorf("value %d is %T, want string", i, values[i])
			}
			*d = value
		case *[]string:
			value, ok := values[i].([]string)
			if !ok {
				return fmt.Errorf("value %d is %T, want []string", i, values[i])
			}
			*d = append([]string(nil), value...)
		case *pgtype.Timestamptz:
			value, ok := values[i].(pgtype.Timestamptz)
			if !ok {
				return fmt.Errorf("value %d is %T, want pgtype.Timestamptz", i, values[i])
			}
			*d = value
		case *time.Time:
			value, ok := values[i].(time.Time)
			if !ok {
				return fmt.Errorf("value %d is %T, want time.Time", i, values[i])
			}
			*d = value
		default:
			return fmt.Errorf("unsupported destination %T", dest[i])
		}
	}
	return nil
}
