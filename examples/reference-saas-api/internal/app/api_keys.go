package app

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"

	"example.com/reference-saas-api/internal/domain"
)

type APIKeyService struct {
	mu        sync.Mutex
	next      int
	keys      map[string]apiKeyRecord
	byHash    map[string]string
	pepper    string
	tenancy   *TenancyService
	now       func() time.Time
	newID     func() (string, error)
	newSecret func() (string, error)
	store     APIKeyStore
}

type apiKeyRecord struct {
	key  domain.APIKey
	hash string
}

type APIKeyStore interface {
	CreateAPIKey(ctx context.Context, key domain.APIKey, hash string) error
	ListAPIKeys(ctx context.Context, organizationID string) ([]domain.APIKey, error)
	GetAPIKeyByHash(ctx context.Context, hash string) (domain.APIKey, bool, error)
	RevokeAPIKey(ctx context.Context, organizationID, keyID string, revokedAt time.Time) (bool, error)
	TouchAPIKey(ctx context.Context, keyID string, lastUsedAt time.Time) error
}

func NewAPIKeyService(pepper string, tenancy *TenancyService) *APIKeyService {
	return &APIKeyService{
		keys:      map[string]apiKeyRecord{},
		byHash:    map[string]string{},
		pepper:    strings.TrimSpace(pepper),
		tenancy:   tenancy,
		now:       time.Now,
		newID:     randomAPIKeyID,
		newSecret: randomAPIKeySecret,
	}
}

func NewAPIKeyServiceWithStore(pepper string, tenancy *TenancyService, store APIKeyStore) *APIKeyService {
	service := NewAPIKeyService(pepper, tenancy)
	service.store = store
	return service
}

func (s *APIKeyService) Create(ctx context.Context, actorID, organizationID, name string, scopes []string, expiresAt *time.Time) (domain.APIKey, string, error) {
	if err := ctx.Err(); err != nil {
		return domain.APIKey{}, "", err
	}
	actorID = strings.TrimSpace(actorID)
	organizationID = strings.TrimSpace(organizationID)
	name = strings.TrimSpace(name)
	cleanScopes, ok := normalizeAPIKeyScopes(scopes)
	if actorID == "" || organizationID == "" || name == "" || !ok || strings.TrimSpace(s.pepper) == "" {
		return domain.APIKey{}, "", ErrValidation
	}
	if err := s.requireRole(ctx, actorID, organizationID, domain.RoleAdmin); err != nil {
		return domain.APIKey{}, "", err
	}
	secret, err := s.newSecret()
	if err != nil {
		return domain.APIKey{}, "", err
	}
	keyID, err := s.newID()
	if err != nil {
		return domain.APIKey{}, "", err
	}
	prefix := apiKeyPrefix(secret)
	recordHash := s.hashSecret(secret)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	now := s.now().UTC()
	key := domain.APIKey{
		ID:             keyID,
		OrganizationID: organizationID,
		Name:           name,
		Prefix:         prefix,
		Scopes:         cleanScopes,
		ExpiresAt:      expiresAt,
		CreatedAt:      now,
	}
	if s.store != nil {
		if err := s.store.CreateAPIKey(ctx, key, recordHash); err != nil {
			return domain.APIKey{}, "", err
		}
		return key, secret, nil
	}
	s.keys[key.ID] = apiKeyRecord{key: key, hash: recordHash}
	s.byHash[recordHash] = key.ID
	return key, secret, nil
}

func (s *APIKeyService) List(ctx context.Context, actorID, organizationID string) ([]domain.APIKey, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	actorID = strings.TrimSpace(actorID)
	organizationID = strings.TrimSpace(organizationID)
	if actorID == "" || organizationID == "" {
		return nil, ErrValidation
	}
	if err := s.requireRole(ctx, actorID, organizationID, domain.RoleAdmin); err != nil {
		return nil, err
	}
	if s.store != nil {
		return s.store.ListAPIKeys(ctx, organizationID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.APIKey, 0, len(s.keys))
	for _, record := range s.keys {
		if record.key.OrganizationID == organizationID {
			out = append(out, record.key)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *APIKeyService) Revoke(ctx context.Context, actorID, organizationID, keyID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	actorID = strings.TrimSpace(actorID)
	organizationID = strings.TrimSpace(organizationID)
	keyID = strings.TrimSpace(keyID)
	if actorID == "" || organizationID == "" || keyID == "" {
		return ErrValidation
	}
	if err := s.requireRole(ctx, actorID, organizationID, domain.RoleAdmin); err != nil {
		return err
	}
	if s.store != nil {
		revokedAt := s.now().UTC()
		ok, err := s.store.RevokeAPIKey(ctx, organizationID, keyID, revokedAt)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.keys[keyID]
	if !ok || record.key.OrganizationID != organizationID {
		return ErrNotFound
	}
	if record.key.RevokedAt == nil {
		now := s.now().UTC()
		record.key.RevokedAt = &now
		s.keys[keyID] = record
	}
	return nil
}

func (s *APIKeyService) Verify(ctx context.Context, secret string) (domain.APIKey, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.APIKey{}, false, err
	}
	secret = strings.TrimSpace(secret)
	if secret == "" || strings.TrimSpace(s.pepper) == "" {
		return domain.APIKey{}, false, ErrValidation
	}
	hash := s.hashSecret(secret)
	if s.store != nil {
		key, ok, err := s.store.GetAPIKeyByHash(ctx, hash)
		if err != nil || !ok {
			return domain.APIKey{}, ok, err
		}
		now := s.now().UTC()
		if key.RevokedAt != nil || (key.ExpiresAt != nil && !now.Before(*key.ExpiresAt)) {
			return domain.APIKey{}, false, nil
		}
		key.LastUsedAt = &now
		if err := s.store.TouchAPIKey(ctx, key.ID, now); err != nil {
			return domain.APIKey{}, false, err
		}
		return key, true, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	keyID, ok := s.byHash[hash]
	if !ok {
		return domain.APIKey{}, false, nil
	}
	record := s.keys[keyID]
	now := s.now().UTC()
	if record.key.RevokedAt != nil || (record.key.ExpiresAt != nil && !now.Before(*record.key.ExpiresAt)) {
		return domain.APIKey{}, false, nil
	}
	record.key.LastUsedAt = &now
	s.keys[keyID] = record
	return record.key, true, nil
}

func (s *APIKeyService) requireRole(ctx context.Context, actorID, organizationID string, role domain.Role) error {
	if s.tenancy == nil {
		return ErrForbidden
	}
	ok, err := s.tenancy.HasRole(ctx, organizationID, actorID, role)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

func (s *APIKeyService) hashSecret(secret string) string {
	mac := hmac.New(sha256.New, []byte(s.pepper))
	_, _ = mac.Write([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(mac.Sum(nil))
}

func randomAPIKeyID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "key_" + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func randomAPIKeySecret() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "atk_" + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func apiKeyPrefix(secret string) string {
	secret = strings.TrimSpace(secret)
	if len(secret) <= 12 {
		return secret
	}
	return secret[:12]
}

func normalizeAPIKeyScopes(scopes []string) ([]string, bool) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" || len(scope) > 80 || !safeAPIKeyScope(scope) {
			return nil, false
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	sort.Strings(out)
	return out, len(out) > 0
}

func safeAPIKeyScope(scope string) bool {
	for _, r := range scope {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case ':', '.', '_', '-':
			continue
		default:
			return false
		}
	}
	return true
}
