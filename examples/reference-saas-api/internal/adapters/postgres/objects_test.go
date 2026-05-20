package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"example.com/reference-saas-api/internal/app"
)

func TestObjectStoreSaveUsesUpsertWithoutPayload(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	db := &fakeObjectDB{row: fakeObjectRow{values: []any{"readme.txt"}}}
	store := NewObjectStore(db)
	err := store.SaveObjectMetadata(context.Background(), app.Object{TenantID: "org_1", Key: "readme.txt", ContentType: "text/plain", Size: 5, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("SaveObjectMetadata() error = %v", err)
	}
	if !strings.Contains(db.lastRowSQL, "insert into objects") || !strings.Contains(db.lastRowSQL, "on conflict") {
		t.Fatalf("SaveObjectMetadata() SQL = %q", db.lastRowSQL)
	}
	if strings.Contains(fmt.Sprint(db.lastRowArgs...), "hello") {
		t.Fatalf("metadata args leaked object payload: %#v", db.lastRowArgs)
	}
}

func TestObjectStoreListGetAndDelete(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	values := []any{"org_1", "readme.txt", "text/plain", int64(5), now, now}
	db := &fakeObjectDB{
		rows:    &fakeObjectRows{rows: [][]any{values}},
		row:     fakeObjectRow{values: values},
		execTag: pgconn.NewCommandTag("DELETE 1"),
	}
	store := NewObjectStore(db)
	objects, err := store.ListObjectMetadata(context.Background(), "org_1")
	if err != nil {
		t.Fatalf("ListObjectMetadata() error = %v", err)
	}
	if len(objects) != 1 || objects[0].Key != "readme.txt" || objects[0].Size != 5 {
		t.Fatalf("ListObjectMetadata() = %#v", objects)
	}
	got, ok, err := store.GetObjectMetadata(context.Background(), "org_1", "readme.txt")
	if err != nil || !ok || got.ContentType != "text/plain" {
		t.Fatalf("GetObjectMetadata() object=%#v ok=%v err=%v", got, ok, err)
	}
	deleted, err := store.DeleteObjectMetadata(context.Background(), "org_1", "readme.txt")
	if err != nil || !deleted {
		t.Fatalf("DeleteObjectMetadata() deleted=%v err=%v", deleted, err)
	}
	if strings.Contains(fmt.Sprint(db.lastExecArgs...), "hello") {
		t.Fatalf("delete args leaked object payload: %#v", db.lastExecArgs)
	}
}

func TestObjectStoreGetNotFound(t *testing.T) {
	store := NewObjectStore(&fakeObjectDB{row: fakeObjectRow{err: pgx.ErrNoRows}})
	got, ok, err := store.GetObjectMetadata(context.Background(), "org_1", "missing.txt")
	if err != nil || ok || got.Key != "" {
		t.Fatalf("GetObjectMetadata() object=%#v ok=%v err=%v", got, ok, err)
	}
}

func TestObjectStoreRequiresDBAndValidObject(t *testing.T) {
	if err := (*ObjectStore)(nil).SaveObjectMetadata(context.Background(), app.Object{}); !errors.Is(err, ErrObjectStoreRequired) {
		t.Fatalf("nil SaveObjectMetadata() error = %v, want %v", err, ErrObjectStoreRequired)
	}
	if err := NewObjectStore(&fakeObjectDB{}).SaveObjectMetadata(context.Background(), app.Object{}); !errors.Is(err, ErrObjectInvalid) {
		t.Fatalf("invalid SaveObjectMetadata() error = %v, want %v", err, ErrObjectInvalid)
	}
}

type fakeObjectDB struct {
	rows          pgx.Rows
	row           pgx.Row
	queryErr      error
	execTag       pgconn.CommandTag
	execErr       error
	lastQuerySQL  string
	lastQueryArgs []any
	lastRowSQL    string
	lastRowArgs   []any
	lastExecSQL   string
	lastExecArgs  []any
}

func (f *fakeObjectDB) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.lastQuerySQL = sql
	f.lastQueryArgs = append([]any(nil), args...)
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	if f.rows == nil {
		return &fakeObjectRows{}, nil
	}
	return f.rows, nil
}

func (f *fakeObjectDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	f.lastRowSQL = sql
	f.lastRowArgs = append([]any(nil), args...)
	if f.row == nil {
		return fakeObjectRow{err: pgx.ErrNoRows}
	}
	return f.row
}

func (f *fakeObjectDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.lastExecSQL = sql
	f.lastExecArgs = append([]any(nil), args...)
	return f.execTag, f.execErr
}

type fakeObjectRows struct {
	rows [][]any
	idx  int
	err  error
}

func (r *fakeObjectRows) Close()                                       {}
func (r *fakeObjectRows) Err() error                                   { return r.err }
func (r *fakeObjectRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeObjectRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeObjectRows) Values() ([]any, error)                       { return r.rows[r.idx-1], nil }
func (r *fakeObjectRows) RawValues() [][]byte                          { return nil }
func (r *fakeObjectRows) Conn() *pgx.Conn                              { return nil }

func (r *fakeObjectRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeObjectRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("Scan called without current row")
	}
	return scanFakeObjectValues(r.rows[r.idx-1], dest...)
}

type fakeObjectRow struct {
	values []any
	err    error
}

func (r fakeObjectRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return scanFakeObjectValues(r.values, dest...)
}

func scanFakeObjectValues(values []any, dest ...any) error {
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
		case *int64:
			value, ok := values[i].(int64)
			if !ok {
				return fmt.Errorf("value %d is %T, want int64", i, values[i])
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
