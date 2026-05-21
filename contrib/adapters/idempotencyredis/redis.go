package idempotencyredis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/aatuh/api-toolkit/v3/ports"
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
	// Options.LegacyInFlightRecoveryRawKey is explicitly enabled.
	Key string
	// KeyHash always contains the hashed key value when a key is available.
	KeyHash string
	// RawKey is populated only when LegacyInFlightRecoveryRawKey is explicitly enabled.
	RawKey  string
	Outcome LegacyInFlightRecoveryOutcome
}

// LegacyInFlightRecoveryHandler is invoked when a tokenless legacy recovery
// branch is encountered. Keep handlers fast and non-blocking.
type LegacyInFlightRecoveryHandler interface {
	HandleLegacyInFlightRecovery(context.Context, LegacyInFlightRecoveryEvent)
}

// LegacyInFlightRecoveryHandlerFunc adapts a function into a
// LegacyInFlightRecoveryHandler.
type LegacyInFlightRecoveryHandlerFunc func(context.Context, LegacyInFlightRecoveryEvent)

// HandleLegacyInFlightRecovery calls f when f is not nil.
func (f LegacyInFlightRecoveryHandlerFunc) HandleLegacyInFlightRecovery(ctx context.Context, event LegacyInFlightRecoveryEvent) {
	if f != nil {
		f(ctx, event)
	}
}

// Options configures Redis-backed idempotency storage.
type Options struct {
	KeyPrefix string
	// OnLegacyInFlightRecovery receives structured recovery telemetry.
	OnLegacyInFlightRecovery LegacyInFlightRecoveryHandler
	// LegacyInFlightRecoveryRawKey exposes raw idempotency keys in recovery events.
	// Defaults to false so adapter telemetry receives hashed keys.
	LegacyInFlightRecoveryRawKey bool
}

// Store implements ports.IdempotencyStore using Redis.
type Store struct {
	client                   redis.UniversalClient
	prefix                   string
	onLegacyInFlightRecovery LegacyInFlightRecoveryHandler
	legacyRecoveryRawKey     bool
}

var _ ports.ReservationReleasableIdempotencyStore = (*Store)(nil)

// New constructs a Redis-backed idempotency store.
func New(client redis.UniversalClient, opts Options) *Store {
	prefix := opts.KeyPrefix
	if prefix == "" {
		prefix = "idempotency:"
	}
	return &Store{
		client:                   client,
		prefix:                   prefix,
		onLegacyInFlightRecovery: opts.OnLegacyInFlightRecovery,
		legacyRecoveryRawKey:     opts.LegacyInFlightRecoveryRawKey,
	}
}

var releaseReservationScript = redis.NewScript(`
local raw = redis.call("GET", KEYS[1])
if not raw then
  return "missing"
end
local ok, record = pcall(cjson.decode, raw)
if not ok then
  return "malformed"
end
if record["State"] ~= 1 then
  return "preserve"
end
local reservation_token = record["ReservationToken"]
if reservation_token == nil then
  reservation_token = ""
end
if reservation_token == "" then
  if ARGV[1] == "" then
    redis.call("DEL", KEYS[1])
    return "legacy_deleted"
  end
  return "legacy_mismatch"
end
if reservation_token ~= ARGV[1] then
  return "token_mismatch"
end
redis.call("DEL", KEYS[1])
return "deleted"
`)

// Get returns an idempotency record if present.
func (s *Store) Get(ctx context.Context, key string) (ports.IdempotencyRecord, bool, error) {
	if s == nil || s.client == nil {
		return ports.IdempotencyRecord{}, false, nil
	}
	val, err := s.client.Get(ctx, s.key(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return ports.IdempotencyRecord{}, false, nil
	}
	if err != nil {
		return ports.IdempotencyRecord{}, false, err
	}
	var record ports.IdempotencyRecord
	if err := json.Unmarshal(val, &record); err != nil {
		return ports.IdempotencyRecord{}, false, fmt.Errorf("decode idempotency record: %w", err)
	}
	return record, true, nil
}

// TryBegin reserves a key for an in-flight request.
func (s *Store) TryBegin(ctx context.Context, key string, record ports.IdempotencyRecord, ttl time.Duration) (bool, error) {
	if s == nil || s.client == nil {
		return false, nil
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return false, fmt.Errorf("encode idempotency record: %w", err)
	}
	ok, err := s.client.SetNX(ctx, s.key(key), payload, ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

// Save persists a completed idempotency record.
func (s *Store) Save(ctx context.Context, key string, record ports.IdempotencyRecord, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return nil
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode idempotency record: %w", err)
	}
	return s.client.Set(ctx, s.key(key), payload, ttl).Err()
}

// Release removes an in-flight idempotency reservation by key.
//
// This method preserves the v2 compatibility contract. New middleware uses
// ReleaseReservation when available so tokened reservations are not released by
// unrelated requests.
func (s *Store) Release(ctx context.Context, key string) error {
	return s.release(ctx, key, "", false)
}

// ReleaseReservation removes a stored idempotency reservation when token matches.
func (s *Store) ReleaseReservation(ctx context.Context, key, token string) error {
	return s.release(ctx, key, token, true)
}

func (s *Store) release(ctx context.Context, key, token string, requireToken bool) error {
	if s == nil || s.client == nil {
		return nil
	}
	if requireToken {
		return s.releaseReservationAtomic(ctx, key, token)
	}
	raw, err := s.client.Get(ctx, s.key(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		return err
	}
	var record ports.IdempotencyRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return err
	}
	if record.State != ports.IdempotencyStateInFlight {
		return nil
	}
	if !requireToken {
		return s.client.Del(ctx, s.key(key)).Err()
	}
	if record.ReservationToken == "" {
		// Compatibility path for older records that were created before
		// ReservationToken was required. Keep this path narrow and temporary
		// for mixed-version rollout recovery.
		if token == "" {
			if err := s.client.Del(ctx, s.key(key)).Err(); err != nil {
				return err
			}
			s.emitLegacyInFlightRecovery(ctx, key, LegacyInFlightRecoveryRecovered)
			return ports.ErrLegacyInFlightReservationMissingToken
		}
		s.emitLegacyInFlightRecovery(ctx, key, LegacyInFlightRecoveryTokenMismatch)
		return ports.ErrLegacyInFlightTokenMismatch
	}
	if record.ReservationToken != token {
		return ports.ErrLegacyInFlightTokenMismatch
	}
	return s.client.Del(ctx, s.key(key)).Err()
}

func (s *Store) releaseReservationAtomic(ctx context.Context, key, token string) error {
	result, err := releaseReservationScript.Run(ctx, s.client, []string{s.key(key)}, token).Text()
	if err != nil {
		return err
	}
	switch result {
	case "missing", "preserve", "deleted":
		return nil
	case "legacy_deleted":
		s.emitLegacyInFlightRecovery(ctx, key, LegacyInFlightRecoveryRecovered)
		return ports.ErrLegacyInFlightReservationMissingToken
	case "legacy_mismatch", "token_mismatch":
		if result == "legacy_mismatch" {
			s.emitLegacyInFlightRecovery(ctx, key, LegacyInFlightRecoveryTokenMismatch)
		}
		return ports.ErrLegacyInFlightTokenMismatch
	case "malformed":
		return errors.New("decode idempotency record: malformed redis idempotency record")
	default:
		return fmt.Errorf("unexpected redis release result: %s", result)
	}
}

func (s *Store) key(key string) string {
	return s.prefix + key
}

func (s *Store) emitLegacyInFlightRecovery(ctx context.Context, key string, outcome LegacyInFlightRecoveryOutcome) {
	if s == nil || s.onLegacyInFlightRecovery == nil {
		return
	}
	eventKey := legacyInFlightRecoveryEventKey(key, s.legacyRecoveryRawKey)
	rawKey := ""
	if s.legacyRecoveryRawKey {
		rawKey = strings.TrimSpace(key)
	}
	s.onLegacyInFlightRecovery.HandleLegacyInFlightRecovery(ctx, LegacyInFlightRecoveryEvent{
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
