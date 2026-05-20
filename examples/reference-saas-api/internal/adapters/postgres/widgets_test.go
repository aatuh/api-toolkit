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

	"example.com/reference-saas-api/internal/domain"
)

func TestWidgetStoreSaveUsesUpsert(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	db := &fakeWidgetDB{row: fakeWidgetRow{values: []any{"wgt_1"}}}
	store := NewWidgetStore(db)
	err := store.Save(context.Background(), domain.Widget{ID: "wgt_1", TenantID: "org_1", Name: "First", Version: 1, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if !strings.Contains(db.lastRowSQL, "insert into widgets") || !strings.Contains(db.lastRowSQL, "on conflict") {
		t.Fatalf("Save() SQL = %q", db.lastRowSQL)
	}
	if got := db.lastRowArgs[4]; got != nil {
		t.Fatalf("Save() active deleted_at arg = %#v, want nil", got)
	}
}

func TestWidgetStoreListAndGetScanWidgets(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	db := &fakeWidgetDB{
		rows: &fakeWidgetRows{rows: [][]any{[]any{"wgt_1", "org_1", "First", int64(2), false, now, now}}},
		row:  fakeWidgetRow{values: []any{"wgt_1", "org_1", "First", int64(2), false, now, now}},
	}
	store := NewWidgetStore(db)
	widgets, err := store.List(context.Background(), "org_1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(widgets) != 1 || widgets[0].ID != "wgt_1" || widgets[0].Version != 2 {
		t.Fatalf("List() = %#v", widgets)
	}
	got, ok, err := store.Get(context.Background(), "org_1", "wgt_1")
	if err != nil || !ok || got.ID != "wgt_1" {
		t.Fatalf("Get() widget=%#v ok=%v err=%v", got, ok, err)
	}
}

func TestWidgetStoreGetNotFound(t *testing.T) {
	store := NewWidgetStore(&fakeWidgetDB{row: fakeWidgetRow{err: pgx.ErrNoRows}})
	got, ok, err := store.Get(context.Background(), "org_1", "missing")
	if err != nil || ok || got.ID != "" {
		t.Fatalf("Get() widget=%#v ok=%v err=%v", got, ok, err)
	}
}

func TestWidgetStoreRequiresDBAndValidWidget(t *testing.T) {
	if err := (*WidgetStore)(nil).Save(context.Background(), domain.Widget{}); !errors.Is(err, ErrWidgetStoreRequired) {
		t.Fatalf("nil Save() error = %v, want %v", err, ErrWidgetStoreRequired)
	}
	if err := NewWidgetStore(&fakeWidgetDB{}).Save(context.Background(), domain.Widget{}); !errors.Is(err, ErrWidgetInvalid) {
		t.Fatalf("invalid Save() error = %v, want %v", err, ErrWidgetInvalid)
	}
}

type fakeWidgetDB struct {
	rows          pgx.Rows
	row           pgx.Row
	queryErr      error
	lastQuerySQL  string
	lastQueryArgs []any
	lastRowSQL    string
	lastRowArgs   []any
}

func (f *fakeWidgetDB) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.lastQuerySQL = sql
	f.lastQueryArgs = append([]any(nil), args...)
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	if f.rows == nil {
		return &fakeWidgetRows{}, nil
	}
	return f.rows, nil
}

func (f *fakeWidgetDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	f.lastRowSQL = sql
	f.lastRowArgs = append([]any(nil), args...)
	if f.row == nil {
		return fakeWidgetRow{err: pgx.ErrNoRows}
	}
	return f.row
}

type fakeWidgetRows struct {
	rows [][]any
	idx  int
	err  error
}

func (r *fakeWidgetRows) Close()                                       {}
func (r *fakeWidgetRows) Err() error                                   { return r.err }
func (r *fakeWidgetRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeWidgetRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeWidgetRows) Values() ([]any, error)                       { return r.rows[r.idx-1], nil }
func (r *fakeWidgetRows) RawValues() [][]byte                          { return nil }
func (r *fakeWidgetRows) Conn() *pgx.Conn                              { return nil }

func (r *fakeWidgetRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeWidgetRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("Scan called without current row")
	}
	return scanFakeWidgetValues(r.rows[r.idx-1], dest...)
}

type fakeWidgetRow struct {
	values []any
	err    error
}

func (r fakeWidgetRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return scanFakeWidgetValues(r.values, dest...)
}

func scanFakeWidgetValues(values []any, dest ...any) error {
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
		case *bool:
			value, ok := values[i].(bool)
			if !ok {
				return fmt.Errorf("value %d is %T, want bool", i, values[i])
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
