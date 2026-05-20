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

func TestTenancyStoreCreateOrganizationUsesSingleStatement(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	db := &fakeTenancyDB{execTag: pgconn.NewCommandTag("INSERT 0 1")}
	store := NewTenancyStore(db)
	err := store.CreateOrganization(context.Background(),
		domain.Organization{ID: "org_1", Name: "Acme", CreatedAt: now, UpdatedAt: now},
		domain.Membership{OrganizationID: "org_1", UserID: "owner_1", Role: domain.RoleOwner, CreatedAt: now},
	)
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	if !strings.Contains(db.lastExecSQL, "insert into organizations") || !strings.Contains(db.lastExecSQL, "insert into memberships") {
		t.Fatalf("CreateOrganization() SQL = %q", db.lastExecSQL)
	}
}

func TestTenancyStoreInvitationStoresHashBytesAndAccepts(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	hash := hex.EncodeToString([]byte("12345678901234567890123456789012"))
	db := &fakeTenancyDB{execTag: pgconn.NewCommandTag("INSERT 0 1")}
	store := NewTenancyStore(db)
	err := store.CreateInvitation(context.Background(), domain.Invitation{ID: "inv_1", OrganizationID: "org_1", Email: "member@example.com", Role: domain.RoleMember, ExpiresAt: now.Add(time.Hour), CreatedAt: now}, hash)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}
	hashArg, ok := db.lastExecArgs[4].([]byte)
	if !ok || string(hashArg) != "12345678901234567890123456789012" {
		t.Fatalf("token hash arg = %#v", db.lastExecArgs[4])
	}
	if strings.Contains(fmt.Sprint(db.lastExecArgs...), "raw-token") {
		t.Fatalf("exec args leaked invitation token: %#v", db.lastExecArgs)
	}

	db.row = fakeTenancyRow{values: []any{"org_1", "member_1", "member", now}}
	member, ok, err := store.AcceptInvitation(context.Background(), "inv_1", "member_1", now)
	if err != nil || !ok || member.Role != domain.RoleMember {
		t.Fatalf("AcceptInvitation() member=%#v ok=%v err=%v", member, ok, err)
	}
	if !strings.Contains(db.lastRowSQL, "accepted_at is null") {
		t.Fatalf("AcceptInvitation() SQL = %q", db.lastRowSQL)
	}
}

func TestTenancyStoreListsAndChecksRoles(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	db := &fakeTenancyDB{
		rows: &fakeTenancyRows{rows: [][]any{[]any{"org_1", "Acme", now, now}}},
		row:  fakeTenancyRow{values: []any{"admin"}},
	}
	store := NewTenancyStore(db)
	orgs, err := store.ListOrganizations(context.Background(), "owner_1")
	if err != nil {
		t.Fatalf("ListOrganizations() error = %v", err)
	}
	if len(orgs) != 1 || orgs[0].ID != "org_1" {
		t.Fatalf("ListOrganizations() = %#v", orgs)
	}
	ok, err := store.HasRole(context.Background(), "org_1", "admin_1", domain.RoleMember)
	if err != nil || !ok {
		t.Fatalf("HasRole() ok=%v err=%v", ok, err)
	}
	ok, err = store.HasRole(context.Background(), "org_1", "admin_1", domain.RoleOwner)
	if err != nil || ok {
		t.Fatalf("HasRole(owner) ok=%v err=%v", ok, err)
	}
}

func TestTenancyStoreGetInvitationScansTokenHash(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	tokenHash := []byte("12345678901234567890123456789012")
	db := &fakeTenancyDB{row: fakeTenancyRow{values: []any{"inv_1", "org_1", "member@example.com", "member", tokenHash, now.Add(time.Hour), pgtype.Timestamptz{}, now}}}
	store := NewTenancyStore(db)
	invitation, hash, ok, err := store.GetInvitation(context.Background(), "inv_1")
	if err != nil || !ok || invitation.ID != "inv_1" || hash != hex.EncodeToString(tokenHash) {
		t.Fatalf("GetInvitation() invitation=%#v hash=%q ok=%v err=%v", invitation, hash, ok, err)
	}
}

func TestTenancyStoreRequiresDBAndValidHash(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	err := (*TenancyStore)(nil).CreateInvitation(context.Background(), domain.Invitation{ID: "inv_1", OrganizationID: "org_1", Email: "member@example.com", Role: domain.RoleMember, ExpiresAt: now.Add(time.Hour), CreatedAt: now}, strings.Repeat("0", 64))
	if !errors.Is(err, ErrTenancyStoreRequired) {
		t.Fatalf("nil CreateInvitation() error = %v, want %v", err, ErrTenancyStoreRequired)
	}
	store := NewTenancyStore(&fakeTenancyDB{})
	err = store.CreateInvitation(context.Background(), domain.Invitation{ID: "inv_1", OrganizationID: "org_1", Email: "member@example.com", Role: domain.RoleMember, ExpiresAt: now.Add(time.Hour), CreatedAt: now}, "not-hex")
	if !errors.Is(err, ErrTenancyInvalid) {
		t.Fatalf("invalid hash CreateInvitation() error = %v, want %v", err, ErrTenancyInvalid)
	}
}

type fakeTenancyDB struct {
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

func (f *fakeTenancyDB) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.lastQuerySQL = sql
	f.lastQueryArgs = append([]any(nil), args...)
	if f.rows == nil {
		return &fakeTenancyRows{}, nil
	}
	return f.rows, nil
}

func (f *fakeTenancyDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	f.lastRowSQL = sql
	f.lastRowArgs = append([]any(nil), args...)
	if f.row == nil {
		return fakeTenancyRow{err: pgx.ErrNoRows}
	}
	return f.row
}

func (f *fakeTenancyDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.lastExecSQL = sql
	f.lastExecArgs = append([]any(nil), args...)
	return f.execTag, f.execErr
}

type fakeTenancyRows struct {
	rows [][]any
	idx  int
	err  error
}

func (r *fakeTenancyRows) Close()                                       {}
func (r *fakeTenancyRows) Err() error                                   { return r.err }
func (r *fakeTenancyRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeTenancyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeTenancyRows) Values() ([]any, error)                       { return r.rows[r.idx-1], nil }
func (r *fakeTenancyRows) RawValues() [][]byte                          { return nil }
func (r *fakeTenancyRows) Conn() *pgx.Conn                              { return nil }

func (r *fakeTenancyRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeTenancyRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("Scan called without current row")
	}
	return scanFakeTenancyValues(r.rows[r.idx-1], dest...)
}

type fakeTenancyRow struct {
	values []any
	err    error
}

func (r fakeTenancyRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return scanFakeTenancyValues(r.values, dest...)
}

func scanFakeTenancyValues(values []any, dest ...any) error {
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
		case *[]byte:
			value, ok := values[i].([]byte)
			if !ok {
				return fmt.Errorf("value %d is %T, want []byte", i, values[i])
			}
			*d = append([]byte(nil), value...)
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
