package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/aatuh/api-toolkit/v2/ports"
)

const legacyInFlightRecoveryUnknownKeyValue = "[redacted]"

// LegacyInFlightRecoveryOutcome captures whether a legacy tokenless recovery path
// was exercised in the adapter.
type LegacyInFlightRecoveryOutcome string

const (
	// LegacyInFlightRecoveryRecovered indicates tokenless legacy in-flight state
	// was intentionally migrated away.
	LegacyInFlightRecoveryRecovered LegacyInFlightRecoveryOutcome = "legacy_in_flight_recovered"
	// LegacyInFlightRecoveryTokenMismatch indicates a token was supplied for a
	// tokenless legacy in-flight record and release was rejected.
	LegacyInFlightRecoveryTokenMismatch LegacyInFlightRecoveryOutcome = "legacy_in_flight_token_mismatch"
)

// LegacyInFlightRecoveryEvent carries structured context for legacy token recovery.
type LegacyInFlightRecoveryEvent struct {
	// Key is hashed by default. It contains the raw key only when
	// MemoryStoreOptions.LegacyInFlightRecoveryRawKey is explicitly enabled.
	Key string
	// KeyHash always contains the hashed key value when a key is available.
	KeyHash string
	// RawKey is populated only when LegacyInFlightRecoveryRawKey is explicitly enabled.
	RawKey  string
	Outcome LegacyInFlightRecoveryOutcome
}

// LegacyInFlightRecoveryHandler is invoked when a tokenless legacy recovery branch
// is encountered. Keep handlers fast and non-blocking.
type LegacyInFlightRecoveryHandler func(context.Context, LegacyInFlightRecoveryEvent)

// MemoryStoreOptions configures legacy recovery telemetry for memory store usage.
type MemoryStoreOptions struct {
	OnLegacyInFlightRecovery LegacyInFlightRecoveryHandler
	// LegacyInFlightRecoveryRawKey exposes raw idempotency keys in recovery events.
	// Defaults to false so adapter telemetry receives hashed keys.
	LegacyInFlightRecoveryRawKey bool
}

// MemoryStore is an in-memory idempotency store intended for development/testing.
type MemoryStore struct {
	mu                       sync.Mutex
	data                     map[string]memoryEntry
	now                      func() time.Time
	onLegacyInFlightRecovery LegacyInFlightRecoveryHandler
	legacyRecoveryRawKey     bool
}

var _ ports.ReleasableIdempotencyStore = (*MemoryStore)(nil)
var _ ports.IdempotencyReservationReleaser = (*MemoryStore)(nil)

type memoryEntry struct {
	record    ports.IdempotencyRecord
	expiresAt time.Time
}

// NewMemoryStore constructs an in-memory idempotency store.
func NewMemoryStore() *MemoryStore {
	return NewMemoryStoreWithOptions(MemoryStoreOptions{})
}

// NewMemoryStoreWithOptions constructs an in-memory idempotency store with optional
// observability hooks.
func NewMemoryStoreWithOptions(opts MemoryStoreOptions) *MemoryStore {
	return &MemoryStore{
		data:                     make(map[string]memoryEntry),
		now:                      time.Now,
		onLegacyInFlightRecovery: opts.OnLegacyInFlightRecovery,
		legacyRecoveryRawKey:     opts.LegacyInFlightRecoveryRawKey,
	}
}

// Get returns an idempotency record if present and not expired.
func (m *MemoryStore) Get(ctx context.Context, key string) (ports.IdempotencyRecord, bool, error) {
	if m == nil {
		return ports.IdempotencyRecord{}, false, nil
	}
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.data[key]
	if !ok {
		return ports.IdempotencyRecord{}, false, nil
	}
	if m.isExpired(entry) {
		delete(m.data, key)
		return ports.IdempotencyRecord{}, false, nil
	}
	return cloneRecord(entry.record), true, nil
}

// TryBegin reserves a key for an in-flight request.
func (m *MemoryStore) TryBegin(ctx context.Context, key string, record ports.IdempotencyRecord, ttl time.Duration) (bool, error) {
	if m == nil {
		return false, nil
	}
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.data[key]; ok && !m.isExpired(entry) {
		return false, nil
	}
	m.data[key] = memoryEntry{
		record:    cloneRecord(record),
		expiresAt: m.now().Add(ttl),
	}
	return true, nil
}

// Save persists a completed idempotency record.
func (m *MemoryStore) Save(ctx context.Context, key string, record ports.IdempotencyRecord, ttl time.Duration) error {
	if m == nil {
		return nil
	}
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = memoryEntry{
		record:    cloneRecord(record),
		expiresAt: m.now().Add(ttl),
	}
	return nil
}

// Release removes an in-flight idempotency reservation by key.
//
// This method preserves the v2 compatibility contract. New middleware uses
// ReleaseReservation when available so tokened reservations are not released by
// unrelated requests.
func (m *MemoryStore) Release(ctx context.Context, key string) error {
	return m.release(ctx, key, "", false)
}

// ReleaseReservation removes a stored idempotency reservation when token matches.
func (m *MemoryStore) ReleaseReservation(ctx context.Context, key, token string) error {
	return m.release(ctx, key, token, true)
}

func (m *MemoryStore) release(ctx context.Context, key, token string, requireToken bool) error {
	_ = ctx
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.data[key]
	if !ok {
		return nil
	}
	if entry.record.State != ports.IdempotencyStateInFlight {
		return nil
	}
	if !requireToken {
		delete(m.data, key)
		return nil
	}
	if entry.record.ReservationToken == "" {
		// Compatibility path for older records that were created before
		// ReservationToken was required. Keep this path narrow and temporary
		// for mixed-version rollout recovery.
		if token == "" {
			delete(m.data, key)
			m.emitLegacyInFlightRecovery(ctx, key, LegacyInFlightRecoveryRecovered)
			return ports.ErrLegacyInFlightReservationMissingToken
		}
		m.emitLegacyInFlightRecovery(ctx, key, LegacyInFlightRecoveryTokenMismatch)
		return ports.ErrLegacyInFlightTokenMismatch
	}
	if entry.record.ReservationToken != token {
		return ports.ErrLegacyInFlightTokenMismatch
	}
	delete(m.data, key)
	return nil
}

func (m *MemoryStore) emitLegacyInFlightRecovery(ctx context.Context, key string, outcome LegacyInFlightRecoveryOutcome) {
	if m == nil || m.onLegacyInFlightRecovery == nil {
		return
	}
	eventKey := legacyInFlightRecoveryEventKey(key, m.legacyRecoveryRawKey)
	rawKey := ""
	if m.legacyRecoveryRawKey {
		rawKey = strings.TrimSpace(key)
	}
	m.onLegacyInFlightRecovery(ctx, LegacyInFlightRecoveryEvent{
		Key:     eventKey,
		KeyHash: legacyInFlightRecoveryEventKey(key, false),
		RawKey:  rawKey,
		Outcome: outcome,
	})
}

func legacyInFlightRecoveryEventKey(key string, exposeRaw bool) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return legacyInFlightRecoveryUnknownKeyValue
	}
	if exposeRaw {
		return trimmed
	}
	h := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(h[:])
}

func (m *MemoryStore) isExpired(entry memoryEntry) bool {
	if entry.expiresAt.IsZero() {
		return false
	}
	return m.now().After(entry.expiresAt)
}

func cloneRecord(record ports.IdempotencyRecord) ports.IdempotencyRecord {
	out := record
	if record.Header != nil {
		out.Header = record.Header.Clone()
	}
	if record.Body != nil {
		out.Body = append([]byte(nil), record.Body...)
	}
	return out
}
