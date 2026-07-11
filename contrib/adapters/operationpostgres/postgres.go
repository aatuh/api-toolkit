package operationpostgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/txpostgres"
	"github.com/aatuh/api-toolkit/contrib/v3/contracts"
	"github.com/aatuh/api-toolkit/v3/endpoints/health"
	"github.com/aatuh/api-toolkit/v3/operations"
)

const defaultTable = "operations"

var (
	// ErrInvalidTable reports that a configured table name is not a safe SQL identifier.
	ErrInvalidTable = errors.New("invalid operation table")
	// ErrOperationNotFound reports that an operation row was not found for the tenant.
	ErrOperationNotFound = errors.New("operation not found")
	// ErrStoreNotConfigured reports that the store or pool was not configured.
	ErrStoreNotConfigured = errors.New("operation postgres store not configured")
	// ErrTenantRequired reports that an operation write/read was attempted without tenant context.
	ErrTenantRequired = errors.New("operation tenant id is required")
)

type tenantIDKeyType struct{}

var tenantIDKey tenantIDKeyType

// WithTenantID stores a tenant ID in ctx for repository operations.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, tenantIDKey, strings.TrimSpace(tenantID))
}

// TenantIDFromContext returns the tenant ID stored by WithTenantID.
func TenantIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	tenantID, ok := ctx.Value(tenantIDKey).(string)
	tenantID = strings.TrimSpace(tenantID)
	return tenantID, ok && tenantID != ""
}

// Options configures the Postgres operation store.
type Options struct {
	Table string
	Clock func() time.Time
}

// Store persists pollable operation resources in Postgres.
type Store[T any] struct {
	Pool  contracts.DatabasePool
	table string
	clock func() time.Time
}

// New creates a Postgres-backed operation repository.
func New[T any](pool contracts.DatabasePool, opts Options) *Store[T] {
	table := strings.TrimSpace(opts.Table)
	if table == "" {
		table = defaultTable
	}
	clock := opts.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	return &Store[T]{
		Pool:  pool,
		table: table,
		clock: clock,
	}
}

// GetOperation loads an operation resource for the tenant in ctx.
func (s *Store[T]) GetOperation(ctx context.Context, id string) (operations.Operation[T], bool, error) {
	if s == nil || s.Pool == nil {
		return operations.Operation[T]{}, false, ErrStoreNotConfigured
	}
	tenantID, ok := TenantIDFromContext(ctx)
	if !ok {
		return operations.Operation[T]{}, false, ErrTenantRequired
	}
	table, err := quoteTable(s.table)
	if err != nil {
		return operations.Operation[T]{}, false, err
	}
	query := fmt.Sprintf(`select state, result, error from %s where id=$1 and organization_id=$2`, table) // #nosec G201 -- table is validated and quoted by quoteTable before interpolation.
	var (
		state   string
		result  []byte
		problem []byte
	)
	db := txpostgres.FromCtx(ctx, s.Pool)
	if err := db.QueryRow(ctx, query, strings.TrimSpace(id), tenantID).Scan(&state, &result, &problem); err != nil {
		if txpostgres.IsNoRows(err) {
			return operations.Operation[T]{}, false, nil
		}
		return operations.Operation[T]{}, false, err
	}
	operation := operations.Operation[T]{
		ID:    strings.TrimSpace(id),
		State: operations.State(state),
	}
	if err := operations.ValidateState(operation.State); err != nil {
		return operations.Operation[T]{}, false, err
	}
	if len(result) > 0 {
		var value T
		if err := json.Unmarshal(result, &value); err != nil {
			return operations.Operation[T]{}, false, err
		}
		operation.Result = &value
	}
	if len(problem) > 0 {
		if err := json.Unmarshal(problem, &operation.Problem); err != nil {
			return operations.Operation[T]{}, false, err
		}
	}
	return operation, true, nil
}

// CreateOperation inserts an operation resource for the tenant in ctx.
func (s *Store[T]) CreateOperation(ctx context.Context, operation operations.Operation[T]) error {
	if s == nil || s.Pool == nil {
		return ErrStoreNotConfigured
	}
	tenantID, ok := TenantIDFromContext(ctx)
	if !ok {
		return ErrTenantRequired
	}
	if strings.TrimSpace(operation.ID) == "" {
		return operations.ErrInvalidState
	}
	if operation.State == "" {
		operation.State = operations.StatePending
	}
	if err := operations.ValidateState(operation.State); err != nil {
		return err
	}
	table, err := quoteTable(s.table)
	if err != nil {
		return err
	}
	result, problem, err := marshalPayloads(operation)
	if err != nil {
		return err
	}
	now := s.clock()
	query := fmt.Sprintf(`insert into %s (id, organization_id, state, result, error, created_at, updated_at) values ($1,$2,$3,$4,$5,$6,$6)`, table) // #nosec G201 -- table is validated and quoted by quoteTable before interpolation.
	db := txpostgres.FromCtx(ctx, s.Pool)
	_, err = db.Exec(ctx, query, strings.TrimSpace(operation.ID), tenantID, string(operation.State), result, problem, now)
	return err
}

// UpdateOperation updates an operation resource for the tenant in ctx.
func (s *Store[T]) UpdateOperation(ctx context.Context, operation operations.Operation[T]) error {
	if s == nil || s.Pool == nil {
		return ErrStoreNotConfigured
	}
	tenantID, ok := TenantIDFromContext(ctx)
	if !ok {
		return ErrTenantRequired
	}
	if strings.TrimSpace(operation.ID) == "" {
		return ErrOperationNotFound
	}
	if operation.State == "" {
		operation.State = operations.StatePending
	}
	if err := operations.ValidateState(operation.State); err != nil {
		return err
	}
	table, err := quoteTable(s.table)
	if err != nil {
		return err
	}
	result, problem, err := marshalPayloads(operation)
	if err != nil {
		return err
	}
	query := fmt.Sprintf(`update %s set state=$3, result=$4, error=$5, updated_at=$6 where id=$1 and organization_id=$2`, table) // #nosec G201 -- table is validated and quoted by quoteTable before interpolation.
	db := txpostgres.FromCtx(ctx, s.Pool)
	res, err := db.Exec(ctx, query, strings.TrimSpace(operation.ID), tenantID, string(operation.State), result, problem, s.clock())
	if err != nil {
		return err
	}
	if res == nil || res.RowsAffected() == 0 {
		return ErrOperationNotFound
	}
	return nil
}

// HealthChecker returns a Postgres operation dependency health checker.
func (s *Store[T]) HealthChecker() health.Checker {
	var pool contracts.DatabasePool
	if s != nil {
		pool = s.Pool
	}
	return health.NewCustomChecker(
		"postgres-operations",
		func(ctx context.Context) (health.Status, string, interface{}) {
			if pool == nil {
				return health.StatusUnhealthy, "postgres operations pool not configured", nil
			}
			if err := pool.Ping(ctx); err != nil {
				return health.StatusUnhealthy, fmt.Sprintf("postgres operations ping failed: %v", err), nil
			}
			return health.StatusHealthy, "postgres operations healthy", nil
		},
	)
}

func marshalPayloads[T any](operation operations.Operation[T]) ([]byte, []byte, error) {
	var result []byte
	if operation.Result != nil {
		data, err := json.Marshal(operation.Result)
		if err != nil {
			return nil, nil, err
		}
		result = data
	}
	var problem []byte
	if operation.Problem != nil {
		data, err := json.Marshal(operation.Problem)
		if err != nil {
			return nil, nil, err
		}
		problem = data
	}
	return result, problem, nil
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

var _ operations.Repository[json.RawMessage] = (*Store[json.RawMessage])(nil)
