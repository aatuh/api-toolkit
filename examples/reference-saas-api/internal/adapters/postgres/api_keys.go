package postgres

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"example.com/reference-saas-api/internal/domain"
)

var (
	ErrAPIKeyStoreRequired = errors.New("postgres api key store db is required")
	ErrAPIKeyInvalid       = errors.New("postgres api key is invalid")
)

type APIKeyDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type APIKeyStore struct {
	db APIKeyDB
}

func NewAPIKeyStore(db APIKeyDB) *APIKeyStore {
	return &APIKeyStore{db: db}
}

func (s *APIKeyStore) CreateAPIKey(ctx context.Context, key domain.APIKey, hash string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return ErrAPIKeyStoreRequired
	}
	key.ID = strings.TrimSpace(key.ID)
	key.OrganizationID = strings.TrimSpace(key.OrganizationID)
	key.Name = strings.TrimSpace(key.Name)
	key.Prefix = strings.TrimSpace(key.Prefix)
	hashBytes, err := decodeAPIKeyHash(hash)
	if err != nil {
		return err
	}
	if key.ID == "" || key.OrganizationID == "" || key.Name == "" || key.Prefix == "" || len(key.Scopes) == 0 || key.CreatedAt.IsZero() {
		return ErrAPIKeyInvalid
	}
	if _, err := s.db.Exec(ctx,
		"insert into api_keys (id, organization_id, name, prefix, key_hash, scopes, expires_at, created_at) values ($1, $2, $3, $4, $5, $6, $7, $8)",
		key.ID,
		key.OrganizationID,
		key.Name,
		key.Prefix,
		hashBytes,
		append([]string(nil), key.Scopes...),
		nullableTime(key.ExpiresAt),
		key.CreatedAt.UTC(),
	); err != nil {
		return fmt.Errorf("create api key: %w", err)
	}
	return nil
}

func (s *APIKeyStore) ListAPIKeys(ctx context.Context, organizationID string) ([]domain.APIKey, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, ErrAPIKeyStoreRequired
	}
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return nil, ErrAPIKeyInvalid
	}
	rows, err := s.db.Query(ctx,
		"select id, organization_id, name, prefix, scopes, expires_at, last_used_at, revoked_at, created_at from api_keys where organization_id=$1 order by id",
		organizationID,
	)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()
	var out []domain.APIKey
	for rows.Next() {
		key, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list api key rows: %w", err)
	}
	return out, nil
}

func (s *APIKeyStore) GetAPIKeyByHash(ctx context.Context, hash string) (domain.APIKey, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.APIKey{}, false, err
	}
	if s == nil || s.db == nil {
		return domain.APIKey{}, false, ErrAPIKeyStoreRequired
	}
	hashBytes, err := decodeAPIKeyHash(hash)
	if err != nil {
		return domain.APIKey{}, false, err
	}
	row := s.db.QueryRow(ctx,
		"select id, organization_id, name, prefix, scopes, expires_at, last_used_at, revoked_at, created_at from api_keys where key_hash=$1",
		hashBytes,
	)
	key, err := scanAPIKey(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.APIKey{}, false, nil
	}
	if err != nil {
		return domain.APIKey{}, false, fmt.Errorf("get api key by hash: %w", err)
	}
	return key, true, nil
}

func (s *APIKeyStore) RevokeAPIKey(ctx context.Context, organizationID, keyID string, revokedAt time.Time) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if s == nil || s.db == nil {
		return false, ErrAPIKeyStoreRequired
	}
	organizationID = strings.TrimSpace(organizationID)
	keyID = strings.TrimSpace(keyID)
	if organizationID == "" || keyID == "" || revokedAt.IsZero() {
		return false, ErrAPIKeyInvalid
	}
	tag, err := s.db.Exec(ctx,
		"update api_keys set revoked_at=$1 where organization_id=$2 and id=$3 and revoked_at is null",
		revokedAt.UTC(),
		organizationID,
		keyID,
	)
	if err != nil {
		return false, fmt.Errorf("revoke api key: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (s *APIKeyStore) TouchAPIKey(ctx context.Context, keyID string, lastUsedAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return ErrAPIKeyStoreRequired
	}
	keyID = strings.TrimSpace(keyID)
	if keyID == "" || lastUsedAt.IsZero() {
		return ErrAPIKeyInvalid
	}
	if _, err := s.db.Exec(ctx, "update api_keys set last_used_at=$1 where id=$2", lastUsedAt.UTC(), keyID); err != nil {
		return fmt.Errorf("touch api key: %w", err)
	}
	return nil
}

type apiKeyScanner interface {
	Scan(dest ...any) error
}

func scanAPIKey(row apiKeyScanner) (domain.APIKey, error) {
	var (
		key        domain.APIKey
		expiresAt  pgtype.Timestamptz
		lastUsedAt pgtype.Timestamptz
		revokedAt  pgtype.Timestamptz
	)
	if err := row.Scan(&key.ID, &key.OrganizationID, &key.Name, &key.Prefix, &key.Scopes, &expiresAt, &lastUsedAt, &revokedAt, &key.CreatedAt); err != nil {
		return domain.APIKey{}, err
	}
	key.ExpiresAt = nullableTimestamptz(expiresAt)
	key.LastUsedAt = nullableTimestamptz(lastUsedAt)
	key.RevokedAt = nullableTimestamptz(revokedAt)
	key.CreatedAt = key.CreatedAt.UTC()
	return key, nil
}

func decodeAPIKeyHash(hash string) ([]byte, error) {
	hash = strings.TrimSpace(hash)
	decoded, err := hex.DecodeString(hash)
	if err != nil || len(decoded) != sha256Size {
		return nil, ErrAPIKeyInvalid
	}
	return decoded, nil
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func nullableTimestamptz(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}

const sha256Size = 32
