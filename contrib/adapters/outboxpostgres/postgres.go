package outboxpostgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/aatuh/api-toolkit/contrib/v4/adapters/txpostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/async"
	"github.com/aatuh/api-toolkit/contrib/v4/contracts"
	"github.com/aatuh/api-toolkit/v4/endpoints/health"
)

const (
	defaultTable         = "outbox_events"
	defaultLeaseOwner    = "api-toolkit"
	defaultLeaseDuration = 30 * time.Second
	defaultRetryDelay    = time.Second
	defaultMaxRetryDelay = time.Minute
	defaultMaxAttempts   = 10
)

var (
	// ErrEventNotFound reports that an outbox event was not found for the current lease owner.
	ErrEventNotFound = errors.New("outbox event not found")
	// ErrInvalidEvent reports that an outbox event is missing required fields.
	ErrInvalidEvent = errors.New("invalid outbox event")
	// ErrInvalidTable reports that a configured table name is not a safe SQL identifier.
	ErrInvalidTable = errors.New("invalid outbox table")
	// ErrStoreNotConfigured reports that the store or pool was not configured.
	ErrStoreNotConfigured = errors.New("outbox postgres store not configured")
)

// Event is an event to enqueue into the transactional outbox.
type Event struct {
	ID       string
	TenantID string
	Type     string
	Payload  []byte
	NextAt   time.Time
}

// Options configures the Postgres outbox store.
type Options struct {
	Table         string
	Clock         func() time.Time
	LeaseOwner    string
	LeaseDuration time.Duration
	RetryDelay    time.Duration
	MaxRetryDelay time.Duration
	MaxAttempts   int
}

// Store persists and leases outbox events in Postgres.
type Store struct {
	Pool          contracts.DatabasePool
	table         string
	clock         func() time.Time
	leaseOwner    string
	leaseDuration time.Duration
	retryDelay    time.Duration
	maxRetryDelay time.Duration
	maxAttempts   int
}

// New creates a Postgres-backed outbox store.
func New(pool contracts.DatabasePool, opts Options) *Store {
	table := strings.TrimSpace(opts.Table)
	if table == "" {
		table = defaultTable
	}
	clock := opts.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	leaseOwner := strings.TrimSpace(opts.LeaseOwner)
	if leaseOwner == "" {
		leaseOwner = defaultLeaseOwner
	}
	leaseDuration := opts.LeaseDuration
	if leaseDuration <= 0 {
		leaseDuration = defaultLeaseDuration
	}
	retryDelay := opts.RetryDelay
	if retryDelay <= 0 {
		retryDelay = defaultRetryDelay
	}
	maxRetryDelay := opts.MaxRetryDelay
	if maxRetryDelay <= 0 {
		maxRetryDelay = defaultMaxRetryDelay
	}
	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}
	return &Store{
		Pool:          pool,
		table:         table,
		clock:         clock,
		leaseOwner:    leaseOwner,
		leaseDuration: leaseDuration,
		retryDelay:    retryDelay,
		maxRetryDelay: maxRetryDelay,
		maxAttempts:   maxAttempts,
	}
}

// Enqueue inserts an event into the outbox.
func (s *Store) Enqueue(ctx context.Context, event Event) error {
	if s == nil || s.Pool == nil {
		return ErrStoreNotConfigured
	}
	if err := validateEvent(event); err != nil {
		return err
	}
	table, err := quoteTable(s.table)
	if err != nil {
		return err
	}
	nextAt := event.NextAt
	if nextAt.IsZero() {
		nextAt = s.clock()
	}
	query := fmt.Sprintf(`insert into %s (id, organization_id, event_type, payload, state, next_at, created_at) values ($1,$2,$3,$4,'pending',$5,$6)`, table) // #nosec G201 -- table is validated and quoted by quoteTable before interpolation.
	db := txpostgres.FromCtx(ctx, s.Pool)
	_, err = db.Exec(ctx, query, strings.TrimSpace(event.ID), strings.TrimSpace(event.TenantID), strings.TrimSpace(event.Type), event.Payload, nextAt, s.clock())
	return err
}

// Lease leases due outbox events for async worker execution.
func (s *Store) Lease(ctx context.Context, limit int) ([]async.Job, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrStoreNotConfigured
	}
	if limit <= 0 {
		return nil, nil
	}
	table, err := quoteTable(s.table)
	if err != nil {
		return nil, err
	}
	now := s.clock()
	leaseUntil := now.Add(s.leaseDuration)
	query := fmt.Sprintf(`
		update %s
		set state='leased', lease_owner=$1, lease_expires_at=$2
		where id in (
			select id
			from %s
			where next_at <= $3
			  and (
				state='pending'
				or state='failed'
				or (state='leased' and lease_expires_at < $3)
			  )
			order by next_at, created_at
			limit $4
			for update skip locked
		)
		returning id, organization_id, event_type, payload, retry_count
	`, table, table) // #nosec G201 -- table is validated and quoted by quoteTable before interpolation.
	db := txpostgres.FromCtx(ctx, s.Pool)
	rows, err := db.Query(ctx, query, s.leaseOwner, leaseUntil, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []async.Job
	for rows.Next() {
		var job async.Job
		if err := rows.Scan(&job.ID, &job.TenantID, &job.Kind, &job.Payload, &job.Attempts); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

// Complete marks a leased event as succeeded.
func (s *Store) Complete(ctx context.Context, id string) error {
	if s == nil || s.Pool == nil {
		return ErrStoreNotConfigured
	}
	table, err := quoteTable(s.table)
	if err != nil {
		return err
	}
	query := fmt.Sprintf(`update %s set state='succeeded', lease_owner=null, lease_expires_at=null where id=$1 and lease_owner=$2`, table) // #nosec G201 -- table is validated and quoted by quoteTable before interpolation.
	db := txpostgres.FromCtx(ctx, s.Pool)
	res, err := db.Exec(ctx, query, strings.TrimSpace(id), s.leaseOwner)
	if err != nil {
		return err
	}
	if res == nil || res.RowsAffected() == 0 {
		return ErrEventNotFound
	}
	return nil
}

// Fail records a retryable failure or dead-letters the event after MaxAttempts.
func (s *Store) Fail(ctx context.Context, id string, _ string) error {
	if s == nil || s.Pool == nil {
		return ErrStoreNotConfigured
	}
	table, err := quoteTable(s.table)
	if err != nil {
		return err
	}
	now := s.clock()
	query := fmt.Sprintf(`
		update %s
		set state=case when retry_count + 1 >= $4 then 'dead_letter' else 'failed' end,
			retry_count=retry_count + 1,
			next_at=$3::timestamptz + (least($5::double precision * power(2, retry_count), $6::double precision) * interval '1 second'),
			lease_owner=null,
			lease_expires_at=null
		where id=$1 and lease_owner=$2
	`, table) // #nosec G201 -- table is validated and quoted by quoteTable before interpolation.
	db := txpostgres.FromCtx(ctx, s.Pool)
	res, err := db.Exec(
		ctx,
		query,
		strings.TrimSpace(id),
		s.leaseOwner,
		now,
		s.maxAttempts,
		s.retryDelay.Seconds(),
		s.maxRetryDelay.Seconds(),
	)
	if err != nil {
		return err
	}
	if res == nil || res.RowsAffected() == 0 {
		return ErrEventNotFound
	}
	return nil
}

// HealthChecker returns a Postgres outbox dependency health checker.
func (s *Store) HealthChecker() health.Checker {
	var pool contracts.DatabasePool
	if s != nil {
		pool = s.Pool
	}
	return health.NewCustomChecker(
		"postgres-outbox",
		func(ctx context.Context) (health.Status, string, interface{}) {
			if pool == nil {
				return health.StatusUnhealthy, "postgres outbox pool not configured", nil
			}
			if err := pool.Ping(ctx); err != nil {
				return health.StatusUnhealthy, fmt.Sprintf("postgres outbox ping failed: %v", err), nil
			}
			return health.StatusHealthy, "postgres outbox healthy", nil
		},
	)
}

func validateEvent(event Event) error {
	if strings.TrimSpace(event.ID) == "" ||
		strings.TrimSpace(event.TenantID) == "" ||
		strings.TrimSpace(event.Type) == "" ||
		len(event.Payload) == 0 {
		return ErrInvalidEvent
	}
	return nil
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

var _ async.Store = (*Store)(nil)
