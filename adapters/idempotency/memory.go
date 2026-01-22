package idempotency

import (
	"context"
	"sync"
	"time"

	"github.com/aatuh/api-toolkit/ports"
)

// MemoryStore is an in-memory idempotency store intended for development/testing.
type MemoryStore struct {
	mu   sync.Mutex
	data map[string]memoryEntry
	now  func() time.Time
}

type memoryEntry struct {
	record    ports.IdempotencyRecord
	expiresAt time.Time
}

// NewMemoryStore constructs an in-memory idempotency store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data: make(map[string]memoryEntry),
		now:  time.Now,
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
