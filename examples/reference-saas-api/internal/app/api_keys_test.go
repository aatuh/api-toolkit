package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"example.com/reference-saas-api/internal/domain"
)

func TestAPIKeyServiceHashesSecretAndRevokes(t *testing.T) {
	tenancy := NewTenancyService()
	tenancy.now = fixedAPIKeyTime
	org, _, err := tenancy.CreateOrganization(context.Background(), "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	service := NewAPIKeyService("test-pepper", tenancy)
	service.now = fixedAPIKeyTime
	service.newSecret = func() (string, error) { return "atk_raw-secret-value", nil }
	expiresAt := fixedAPIKeyTime().Add(time.Hour)

	key, secret, err := service.Create(context.Background(), "owner_1", org.ID, "CI", []string{"widgets:write", "widgets:read", "widgets:read"}, &expiresAt)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if secret != "atk_raw-secret-value" {
		t.Fatalf("secret = %q", secret)
	}
	record := service.keys[key.ID]
	if record.hash == "" || strings.Contains(record.hash, secret) || record.hash == secret {
		t.Fatalf("stored hash is unsafe: %#v", record)
	}
	if key.Prefix == "" || key.Prefix == secret {
		t.Fatalf("prefix = %q secret=%q", key.Prefix, secret)
	}
	if len(key.Scopes) != 2 {
		t.Fatalf("deduped scopes = %#v", key.Scopes)
	}
	publicJSON, err := json.Marshal(key.Public())
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	if strings.Contains(string(publicJSON), secret) {
		t.Fatalf("public API key leaked secret: %s", publicJSON)
	}
	verified, ok, err := service.Verify(context.Background(), secret)
	if err != nil || !ok {
		t.Fatalf("Verify() key=%#v ok=%v err=%v", verified, ok, err)
	}
	if verified.LastUsedAt == nil {
		t.Fatalf("Verify() did not update last_used_at: %#v", verified)
	}
	if err := service.Revoke(context.Background(), "owner_1", org.ID, key.ID); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if _, ok, err := service.Verify(context.Background(), secret); err != nil || ok {
		t.Fatalf("Verify() after revoke ok=%v err=%v", ok, err)
	}
}

func TestAPIKeyServiceRequiresAdminAndPepper(t *testing.T) {
	tenancy := NewTenancyService()
	tenancy.now = fixedAPIKeyTime
	tenancy.newToken = func() (string, error) { return "invite-token-value", nil }
	org, _, err := tenancy.CreateOrganization(context.Background(), "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	invitation, token, err := tenancy.InviteMember(context.Background(), "owner_1", org.ID, "viewer@example.com", "viewer")
	if err != nil {
		t.Fatalf("InviteMember() error = %v", err)
	}
	if _, err := tenancy.AcceptInvitation(context.Background(), invitation.ID, token, "viewer_1"); err != nil {
		t.Fatalf("AcceptInvitation() error = %v", err)
	}
	service := NewAPIKeyService("test-pepper", tenancy)
	if _, _, err := service.Create(context.Background(), "viewer_1", org.ID, "Viewer", []string{"widgets:read"}, nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer create error = %v, want %v", err, ErrForbidden)
	}
	withoutPepper := NewAPIKeyService("", tenancy)
	if _, _, err := withoutPepper.Create(context.Background(), "owner_1", org.ID, "Missing pepper", []string{"widgets:read"}, nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("missing pepper error = %v, want %v", err, ErrValidation)
	}
}

func TestAPIKeyServiceWithStorePersistsHashAndTouchesUsage(t *testing.T) {
	tenancy := NewTenancyService()
	tenancy.now = fixedAPIKeyTime
	org, _, err := tenancy.CreateOrganization(context.Background(), "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	store := newRecordingAPIKeyStore()
	service := NewAPIKeyServiceWithStore("test-pepper", tenancy, store)
	service.now = fixedAPIKeyTime
	service.newID = func() (string, error) { return "key_store_1", nil }
	service.newSecret = func() (string, error) { return "atk_store-secret-value", nil }

	key, secret, err := service.Create(context.Background(), "owner_1", org.ID, "Stored", []string{"widgets:read"}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if key.ID != "key_store_1" {
		t.Fatalf("key ID = %q", key.ID)
	}
	if store.createdHash == "" || strings.Contains(store.createdHash, secret) {
		t.Fatalf("stored hash leaked secret: %q", store.createdHash)
	}
	verified, ok, err := service.Verify(context.Background(), secret)
	if err != nil || !ok {
		t.Fatalf("Verify() key=%#v ok=%v err=%v", verified, ok, err)
	}
	if store.touchedKeyID != key.ID || store.touchedAt.IsZero() || verified.LastUsedAt == nil {
		t.Fatalf("touch tracking key=%q at=%v verified=%#v", store.touchedKeyID, store.touchedAt, verified)
	}
	listed, err := service.List(context.Background(), "owner_1", org.ID)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != key.ID {
		t.Fatalf("List() = %#v", listed)
	}
	if err := service.Revoke(context.Background(), "owner_1", org.ID, key.ID); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if !store.revokedAt.Equal(fixedAPIKeyTime()) {
		t.Fatalf("revokedAt = %v", store.revokedAt)
	}
}

type recordingAPIKeyStore struct {
	keys         map[string]domain.APIKey
	byHash       map[string]string
	createdHash  string
	touchedKeyID string
	touchedAt    time.Time
	revokedAt    time.Time
}

func newRecordingAPIKeyStore() *recordingAPIKeyStore {
	return &recordingAPIKeyStore{
		keys:   map[string]domain.APIKey{},
		byHash: map[string]string{},
	}
}

func (s *recordingAPIKeyStore) CreateAPIKey(_ context.Context, key domain.APIKey, hash string) error {
	s.createdHash = hash
	s.keys[key.ID] = key
	s.byHash[hash] = key.ID
	return nil
}

func (s *recordingAPIKeyStore) ListAPIKeys(_ context.Context, organizationID string) ([]domain.APIKey, error) {
	var out []domain.APIKey
	for _, key := range s.keys {
		if key.OrganizationID == organizationID {
			out = append(out, key)
		}
	}
	return out, nil
}

func (s *recordingAPIKeyStore) GetAPIKeyByHash(_ context.Context, hash string) (domain.APIKey, bool, error) {
	keyID, ok := s.byHash[hash]
	if !ok {
		return domain.APIKey{}, false, nil
	}
	key, ok := s.keys[keyID]
	return key, ok, nil
}

func (s *recordingAPIKeyStore) RevokeAPIKey(_ context.Context, organizationID, keyID string, revokedAt time.Time) (bool, error) {
	key, ok := s.keys[keyID]
	if !ok || key.OrganizationID != organizationID {
		return false, nil
	}
	key.RevokedAt = &revokedAt
	s.keys[keyID] = key
	s.revokedAt = revokedAt
	return true, nil
}

func (s *recordingAPIKeyStore) TouchAPIKey(_ context.Context, keyID string, lastUsedAt time.Time) error {
	key := s.keys[keyID]
	key.LastUsedAt = &lastUsedAt
	s.keys[keyID] = key
	s.touchedKeyID = keyID
	s.touchedAt = lastUsedAt
	return nil
}

func fixedAPIKeyTime() time.Time {
	return time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
}
