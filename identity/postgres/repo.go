package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/adapters/txpostgres"
	"github.com/aatuh/api-toolkit/identity"
	"github.com/aatuh/api-toolkit/ports"
)

// Config controls table names for identity persistence.
type Config struct {
	UsersTable string
	RolesTable string
}

// DefaultConfig returns the default table names.
func DefaultConfig() Config {
	return Config{
		UsersTable: "identity_users",
		RolesTable: "identity_user_roles",
	}
}

// Repo persists identity users in Postgres.
type Repo struct {
	Pool ports.DatabasePool
	cfg  Config
}

// NewRepo constructs a Repo with normalized config.
func NewRepo(pool ports.DatabasePool, cfg Config) *Repo {
	cfg = normalizeConfig(cfg)
	return &Repo{Pool: pool, cfg: cfg}
}

func (r *Repo) Create(ctx context.Context, u *identity.User) error {
	db := txpostgres.FromCtx(ctx, r.Pool)
	q := fmt.Sprintf(`
	insert into %s (
	  id, identity_provider, identity_subject,
	  email, first_name, last_name, preferred_language,
	  created_at, updated_at
	) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, r.cfg.UsersTable)
	_, err := db.Exec(ctx, q,
		u.ID, u.Provider, u.Subject,
		u.Email, u.FirstName, u.LastName, u.PreferredLanguage,
		u.CreatedAt, u.UpdatedAt,
	)
	if isUniqueViolation(err) {
		return identity.ErrConflict
	}
	return err
}

func (r *Repo) GetByID(ctx context.Context, id string) (*identity.User, error) {
	db := txpostgres.FromCtx(ctx, r.Pool)
	q := fmt.Sprintf(`
	select id, identity_provider, identity_subject,
	       email, first_name, last_name, preferred_language,
	       created_at, updated_at
	from %s
	where id=$1
	`, r.cfg.UsersTable)
	row := db.QueryRow(ctx, q, id)
	user, err := scanUser(row)
	if err != nil {
		if txpostgres.IsNoRows(err) {
			return nil, identity.ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *Repo) GetByIdentity(ctx context.Context, provider, subject string) (*identity.User, error) {
	db := txpostgres.FromCtx(ctx, r.Pool)
	q := fmt.Sprintf(`
	select id, identity_provider, identity_subject,
	       email, first_name, last_name, preferred_language,
	       created_at, updated_at
	from %s
	where identity_provider=$1 and identity_subject=$2
	`, r.cfg.UsersTable)
	row := db.QueryRow(ctx, q, provider, subject)
	user, err := scanUser(row)
	if err != nil {
		if txpostgres.IsNoRows(err) {
			return nil, identity.ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *Repo) Update(ctx context.Context, u *identity.User) error {
	db := txpostgres.FromCtx(ctx, r.Pool)
	q := fmt.Sprintf(`
	update %s
	set email=$2,
	    first_name=$3,
	    last_name=$4,
	    preferred_language=$5,
	    updated_at=$6
	where id=$1
	`, r.cfg.UsersTable)
	ct, err := db.Exec(ctx, q,
		u.ID, u.Email, u.FirstName, u.LastName,
		u.PreferredLanguage, u.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return identity.ErrNotFound
	}
	return nil
}

func (r *Repo) ListRoles(ctx context.Context, userID string) ([]string, error) {
	db := txpostgres.FromCtx(ctx, r.Pool)
	q := fmt.Sprintf(`
	select role
	from %s
	where user_id=$1
	order by role
	`, r.cfg.RolesTable)
	rows, err := db.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roles := []string{}
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, strings.TrimSpace(role))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *Repo) ReplaceRoles(ctx context.Context, userID string, roles []string, at time.Time) error {
	db := txpostgres.FromCtx(ctx, r.Pool)
	q := fmt.Sprintf("delete from %s where user_id=$1", r.cfg.RolesTable)
	if _, err := db.Exec(ctx, q, userID); err != nil {
		return err
	}
	if len(roles) == 0 {
		return nil
	}
	insertQ := fmt.Sprintf(`
	insert into %s (user_id, role, created_at)
	values ($1,$2,$3)
	`, r.cfg.RolesTable)
	for _, role := range roles {
		if strings.TrimSpace(role) == "" {
			continue
		}
		if _, err := db.Exec(ctx, insertQ, userID, role, at); err != nil {
			return err
		}
	}
	return nil
}

func scanUser(row interface{ Scan(dest ...any) error }) (*identity.User, error) {
	var u identity.User
	err := row.Scan(
		&u.ID,
		&u.Provider,
		&u.Subject,
		&u.Email,
		&u.FirstName,
		&u.LastName,
		&u.PreferredLanguage,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func normalizeConfig(cfg Config) Config {
	def := DefaultConfig()
	cfg.UsersTable = normalizeTable(cfg.UsersTable, def.UsersTable)
	cfg.RolesTable = normalizeTable(cfg.RolesTable, def.RolesTable)
	return cfg
}

func normalizeTable(name, def string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return def
	}
	if !isSafeName(name) {
		return def
	}
	return name
}

func isSafeName(name string) bool {
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '.':
		default:
			return false
		}
	}
	return name != ""
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if pgErr, ok := txpostgres.AsPgError(err); ok {
		return pgErr.Code == "23505"
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate") || strings.Contains(msg, "23505")
}

var _ identity.Repo = (*Repo)(nil)
