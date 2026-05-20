package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"example.com/reference-saas-api/internal/domain"
)

var (
	ErrWidgetStoreRequired = errors.New("postgres widget store db is required")
	ErrWidgetInvalid       = errors.New("postgres widget is invalid")
)

type WidgetDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type WidgetStore struct {
	db WidgetDB
}

func NewWidgetStore(db WidgetDB) *WidgetStore {
	return &WidgetStore{db: db}
}

func (s *WidgetStore) List(ctx context.Context, tenantID string) ([]domain.Widget, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, ErrWidgetStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrWidgetInvalid
	}
	rows, err := s.db.Query(ctx,
		"select id, organization_id, name, version, deleted_at is not null, created_at, updated_at "+
			"from widgets where organization_id=$1 and deleted_at is null order by id",
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list widgets: %w", err)
	}
	defer rows.Close()
	var out []domain.Widget
	for rows.Next() {
		widget, err := scanWidget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, widget)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list widgets rows: %w", err)
	}
	return out, nil
}

func (s *WidgetStore) Get(ctx context.Context, tenantID, id string) (domain.Widget, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.Widget{}, false, err
	}
	if s == nil || s.db == nil {
		return domain.Widget{}, false, ErrWidgetStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	id = strings.TrimSpace(id)
	if tenantID == "" || id == "" {
		return domain.Widget{}, false, ErrWidgetInvalid
	}
	row := s.db.QueryRow(ctx,
		"select id, organization_id, name, version, deleted_at is not null, created_at, updated_at "+
			"from widgets where organization_id=$1 and id=$2",
		tenantID,
		id,
	)
	widget, err := scanWidget(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Widget{}, false, nil
	}
	if err != nil {
		return domain.Widget{}, false, fmt.Errorf("get widget: %w", err)
	}
	return widget, true, nil
}

func (s *WidgetStore) Save(ctx context.Context, widget domain.Widget) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return ErrWidgetStoreRequired
	}
	widget.ID = strings.TrimSpace(widget.ID)
	widget.TenantID = strings.TrimSpace(widget.TenantID)
	widget.Name = strings.TrimSpace(widget.Name)
	if widget.ID == "" || widget.TenantID == "" || widget.Name == "" || widget.Version <= 0 || widget.CreatedAt.IsZero() || widget.UpdatedAt.IsZero() {
		return ErrWidgetInvalid
	}
	var deletedAt any
	if widget.Deleted {
		deletedAt = widget.UpdatedAt.UTC()
	}
	var savedID string
	if err := s.db.QueryRow(ctx,
		"insert into widgets (id, organization_id, name, version, deleted_at, created_at, updated_at) "+
			"values ($1, $2, $3, $4, $5, $6, $7) "+
			"on conflict (id) do update set name=excluded.name, version=excluded.version, deleted_at=excluded.deleted_at, updated_at=excluded.updated_at "+
			"returning id",
		widget.ID,
		widget.TenantID,
		widget.Name,
		widget.Version,
		deletedAt,
		widget.CreatedAt.UTC(),
		widget.UpdatedAt.UTC(),
	).Scan(&savedID); err != nil {
		return fmt.Errorf("save widget: %w", err)
	}
	return nil
}

type widgetScanner interface {
	Scan(dest ...any) error
}

func scanWidget(row widgetScanner) (domain.Widget, error) {
	var widget domain.Widget
	if err := row.Scan(&widget.ID, &widget.TenantID, &widget.Name, &widget.Version, &widget.Deleted, &widget.CreatedAt, &widget.UpdatedAt); err != nil {
		return domain.Widget{}, err
	}
	widget.CreatedAt = widget.CreatedAt.UTC()
	widget.UpdatedAt = widget.UpdatedAt.UTC()
	return widget, nil
}
