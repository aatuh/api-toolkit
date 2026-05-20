package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"example.com/reference-saas-api/internal/app"
)

var (
	ErrObjectStoreRequired = errors.New("postgres object store db is required")
	ErrObjectInvalid       = errors.New("postgres object metadata is invalid")
)

type ObjectDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type ObjectStore struct {
	db ObjectDB
}

func NewObjectStore(db ObjectDB) *ObjectStore {
	return &ObjectStore{db: db}
}

func (s *ObjectStore) SaveObjectMetadata(ctx context.Context, object app.Object) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return ErrObjectStoreRequired
	}
	object.TenantID = strings.TrimSpace(object.TenantID)
	object.Key = strings.TrimSpace(object.Key)
	object.ContentType = strings.TrimSpace(object.ContentType)
	if object.TenantID == "" || object.Key == "" || object.ContentType == "" || object.Size < 0 || object.CreatedAt.IsZero() || object.UpdatedAt.IsZero() {
		return ErrObjectInvalid
	}
	var savedKey string
	if err := s.db.QueryRow(ctx,
		"insert into objects (organization_id, key, content_type, size, created_at, updated_at) "+
			"values ($1, $2, $3, $4, $5, $6) "+
			"on conflict (organization_id, key) do update set content_type=excluded.content_type, size=excluded.size, updated_at=excluded.updated_at "+
			"returning key",
		object.TenantID,
		object.Key,
		object.ContentType,
		object.Size,
		object.CreatedAt.UTC(),
		object.UpdatedAt.UTC(),
	).Scan(&savedKey); err != nil {
		return fmt.Errorf("save object metadata: %w", err)
	}
	return nil
}

func (s *ObjectStore) GetObjectMetadata(ctx context.Context, tenantID, key string) (app.Object, bool, error) {
	if err := ctx.Err(); err != nil {
		return app.Object{}, false, err
	}
	if s == nil || s.db == nil {
		return app.Object{}, false, ErrObjectStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	key = strings.TrimSpace(key)
	if tenantID == "" || key == "" {
		return app.Object{}, false, ErrObjectInvalid
	}
	row := s.db.QueryRow(ctx,
		"select organization_id, key, content_type, size, created_at, updated_at from objects where organization_id=$1 and key=$2",
		tenantID,
		key,
	)
	object, err := scanObjectMetadata(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Object{}, false, nil
	}
	if err != nil {
		return app.Object{}, false, fmt.Errorf("get object metadata: %w", err)
	}
	return object, true, nil
}

func (s *ObjectStore) ListObjectMetadata(ctx context.Context, tenantID string) ([]app.Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, ErrObjectStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrObjectInvalid
	}
	rows, err := s.db.Query(ctx,
		"select organization_id, key, content_type, size, created_at, updated_at from objects where organization_id=$1 order by key",
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list object metadata: %w", err)
	}
	defer rows.Close()
	var out []app.Object
	for rows.Next() {
		object, err := scanObjectMetadata(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list object metadata rows: %w", err)
	}
	return out, nil
}

func (s *ObjectStore) DeleteObjectMetadata(ctx context.Context, tenantID, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if s == nil || s.db == nil {
		return false, ErrObjectStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	key = strings.TrimSpace(key)
	if tenantID == "" || key == "" {
		return false, ErrObjectInvalid
	}
	tag, err := s.db.Exec(ctx, "delete from objects where organization_id=$1 and key=$2", tenantID, key)
	if err != nil {
		return false, fmt.Errorf("delete object metadata: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

type objectScanner interface {
	Scan(dest ...any) error
}

func scanObjectMetadata(row objectScanner) (app.Object, error) {
	var object app.Object
	if err := row.Scan(&object.TenantID, &object.Key, &object.ContentType, &object.Size, &object.CreatedAt, &object.UpdatedAt); err != nil {
		return app.Object{}, err
	}
	object.CreatedAt = object.CreatedAt.UTC()
	object.UpdatedAt = object.UpdatedAt.UTC()
	return object, nil
}
