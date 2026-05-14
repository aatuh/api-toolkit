package auditpostgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/txpostgres"
	"github.com/aatuh/api-toolkit/contrib/v2/audit"
	"github.com/aatuh/api-toolkit/v2/ports"
)

const defaultTable = "audit_events"

var (
	// ErrInvalidTable reports that a configured table name is not a safe SQL identifier.
	ErrInvalidTable = errors.New("invalid audit table")
	// ErrStoreNotConfigured reports that the store or pool was not configured.
	ErrStoreNotConfigured = errors.New("audit postgres store not configured")
)

// Options configures the Postgres audit store.
type Options struct {
	Table string
	Clock func() time.Time
}

// Store writes audit events to Postgres.
type Store struct {
	Pool  ports.DatabasePool
	table string
	clock func() time.Time
}

// New creates a Postgres-backed audit recorder.
func New(pool ports.DatabasePool, opts Options) *Store {
	table := opts.Table
	if strings.TrimSpace(table) == "" {
		table = defaultTable
	}
	clock := opts.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	return &Store{
		Pool:  pool,
		table: table,
		clock: clock,
	}
}

// Record stores an audit event.
func (s *Store) Record(ctx context.Context, event audit.Event) error {
	if s == nil || s.Pool == nil {
		return ErrStoreNotConfigured
	}
	if err := audit.ValidateEvent(event); err != nil {
		return err
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = s.clock()
	}
	table, err := quoteTable(s.table)
	if err != nil {
		return err
	}
	metadata, err := json.Marshal(audit.CloneMetadata(event.Metadata))
	if err != nil {
		return err
	}
	db := txpostgres.FromCtx(ctx, s.Pool)
	query := fmt.Sprintf(`
		insert into %s (
			id,
			organization_id,
			actor_type,
			actor_id,
			action,
			resource_type,
			resource_id,
			result,
			request_id,
			metadata,
			created_at
		)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, table) // #nosec G201 -- table is validated and quoted by quoteTable before interpolation.
	_, err = db.Exec(
		ctx,
		query,
		event.ID,
		event.TenantID,
		event.Actor.Type,
		event.Actor.ID,
		event.Action,
		event.Resource.Type,
		event.Resource.ID,
		string(event.Result),
		event.RequestID,
		metadata,
		event.OccurredAt,
	)
	return err
}

func quoteTable(name string) (string, error) {
	parts := strings.Split(name, ".")
	if len(parts) > 2 {
		return "", ErrInvalidTable
	}
	for i, part := range parts {
		if !validIdentifier(part) {
			return "", ErrInvalidTable
		}
		parts[i] = `"` + part + `"`
	}
	return strings.Join(parts, "."), nil
}

func validIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

var _ audit.Recorder = (*Store)(nil)
