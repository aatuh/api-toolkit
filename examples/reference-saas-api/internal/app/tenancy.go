package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"example.com/reference-saas-api/internal/domain"
)

var ErrForbidden = errors.New("forbidden")

type TenancyService struct {
	mu          sync.Mutex
	nextOrg     int
	nextInvite  int
	orgs        map[string]domain.Organization
	memberships map[string]map[string]domain.Membership
	invitations map[string]invitationRecord
	now         func() time.Time
	newToken    func() (string, error)
	newOrgID    func() (string, error)
	newInviteID func() (string, error)
	store       TenancyStore
}

type invitationRecord struct {
	invitation domain.Invitation
	tokenHash  string
}

type TenancyStore interface {
	CreateOrganization(ctx context.Context, org domain.Organization, owner domain.Membership) error
	ListOrganizations(ctx context.Context, actorID string) ([]domain.Organization, error)
	ListMembers(ctx context.Context, organizationID string) ([]domain.Membership, error)
	CreateInvitation(ctx context.Context, invitation domain.Invitation, tokenHash string) error
	GetInvitation(ctx context.Context, invitationID string) (domain.Invitation, string, bool, error)
	AcceptInvitation(ctx context.Context, invitationID, userID string, acceptedAt time.Time) (domain.Membership, bool, error)
	HasRole(ctx context.Context, organizationID, actorID string, required domain.Role) (bool, error)
}

func NewTenancyService() *TenancyService {
	return &TenancyService{
		orgs:        map[string]domain.Organization{},
		memberships: map[string]map[string]domain.Membership{},
		invitations: map[string]invitationRecord{},
		now:         time.Now,
		newToken:    randomToken,
		newOrgID:    func() (string, error) { return randomPrefixedID("org") },
		newInviteID: func() (string, error) { return randomPrefixedID("inv") },
	}
}

func NewTenancyServiceWithStore(store TenancyStore) *TenancyService {
	service := NewTenancyService()
	service.store = store
	return service
}

func (s *TenancyService) CreateOrganization(ctx context.Context, actorID, name string) (domain.Organization, domain.Membership, error) {
	if err := ctx.Err(); err != nil {
		return domain.Organization{}, domain.Membership{}, err
	}
	actorID = strings.TrimSpace(actorID)
	name = strings.TrimSpace(name)
	if actorID == "" || name == "" {
		return domain.Organization{}, domain.Membership{}, ErrValidation
	}
	orgID, err := s.newOrgID()
	if err != nil {
		return domain.Organization{}, domain.Membership{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextOrg++
	now := s.now().UTC()
	org := domain.Organization{
		ID:        orgID,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	member := domain.Membership{
		OrganizationID: org.ID,
		UserID:         actorID,
		Role:           domain.RoleOwner,
		CreatedAt:      now,
	}
	if s.store != nil {
		if err := s.store.CreateOrganization(ctx, org, member); err != nil {
			return domain.Organization{}, domain.Membership{}, err
		}
		return org, member, nil
	}
	s.orgs[org.ID] = org
	s.memberships[org.ID] = map[string]domain.Membership{actorID: member}
	return org, member, nil
}

func (s *TenancyService) ListOrganizations(ctx context.Context, actorID string) ([]domain.Organization, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return nil, ErrValidation
	}
	if s.store != nil {
		return s.store.ListOrganizations(ctx, actorID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Organization, 0)
	for orgID, members := range s.memberships {
		if _, ok := members[actorID]; ok {
			if org, exists := s.orgs[orgID]; exists {
				out = append(out, org)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *TenancyService) ListMembers(ctx context.Context, actorID, organizationID string) ([]domain.Membership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	actorID = strings.TrimSpace(actorID)
	organizationID = strings.TrimSpace(organizationID)
	if actorID == "" || organizationID == "" {
		return nil, ErrValidation
	}
	if s.store != nil {
		ok, err := s.store.HasRole(ctx, organizationID, actorID, domain.RoleViewer)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrForbidden
		}
		return s.store.ListMembers(ctx, organizationID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasRoleLocked(organizationID, actorID, domain.RoleViewer) {
		return nil, ErrForbidden
	}
	members := s.memberships[organizationID]
	out := make([]domain.Membership, 0, len(members))
	for _, member := range members {
		out = append(out, member)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UserID < out[j].UserID
	})
	return out, nil
}

func (s *TenancyService) InviteMember(ctx context.Context, actorID, organizationID, email string, role domain.Role) (domain.Invitation, string, error) {
	if err := ctx.Err(); err != nil {
		return domain.Invitation{}, "", err
	}
	actorID = strings.TrimSpace(actorID)
	organizationID = strings.TrimSpace(organizationID)
	email = strings.ToLower(strings.TrimSpace(email))
	if actorID == "" || organizationID == "" || email == "" || !strings.Contains(email, "@") || !role.Valid() {
		return domain.Invitation{}, "", ErrValidation
	}
	if s.store != nil {
		ok, err := s.store.HasRole(ctx, organizationID, actorID, domain.RoleAdmin)
		if err != nil {
			return domain.Invitation{}, "", err
		}
		if !ok {
			return domain.Invitation{}, "", ErrForbidden
		}
		if role == domain.RoleOwner {
			owner, err := s.store.HasRole(ctx, organizationID, actorID, domain.RoleOwner)
			if err != nil {
				return domain.Invitation{}, "", err
			}
			if !owner {
				return domain.Invitation{}, "", ErrForbidden
			}
		}
		token, err := s.newToken()
		if err != nil {
			return domain.Invitation{}, "", err
		}
		invitationID, err := s.newInviteID()
		if err != nil {
			return domain.Invitation{}, "", err
		}
		now := s.now().UTC()
		prefix := token
		if len(prefix) > 8 {
			prefix = prefix[:8]
		}
		invitation := domain.Invitation{
			ID:             invitationID,
			OrganizationID: organizationID,
			Email:          email,
			Role:           role,
			TokenPrefix:    prefix,
			ExpiresAt:      now.Add(7 * 24 * time.Hour),
			CreatedAt:      now,
		}
		if err := s.store.CreateInvitation(ctx, invitation, hashToken(token)); err != nil {
			return domain.Invitation{}, "", err
		}
		return invitation, token, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasRoleLocked(organizationID, actorID, domain.RoleAdmin) {
		return domain.Invitation{}, "", ErrForbidden
	}
	if role == domain.RoleOwner && !s.hasRoleLocked(organizationID, actorID, domain.RoleOwner) {
		return domain.Invitation{}, "", ErrForbidden
	}
	token, err := s.newToken()
	if err != nil {
		return domain.Invitation{}, "", err
	}
	invitationID, err := s.newInviteID()
	if err != nil {
		return domain.Invitation{}, "", err
	}
	s.nextInvite++
	now := s.now().UTC()
	prefix := token
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	invitation := domain.Invitation{
		ID:             invitationID,
		OrganizationID: organizationID,
		Email:          email,
		Role:           role,
		TokenPrefix:    prefix,
		ExpiresAt:      now.Add(7 * 24 * time.Hour),
		CreatedAt:      now,
	}
	s.invitations[invitation.ID] = invitationRecord{
		invitation: invitation,
		tokenHash:  hashToken(token),
	}
	return invitation, token, nil
}

func (s *TenancyService) AcceptInvitation(ctx context.Context, invitationID, token, userID string) (domain.Membership, error) {
	if err := ctx.Err(); err != nil {
		return domain.Membership{}, err
	}
	invitationID = strings.TrimSpace(invitationID)
	token = strings.TrimSpace(token)
	userID = strings.TrimSpace(userID)
	if invitationID == "" || token == "" || userID == "" {
		return domain.Membership{}, ErrValidation
	}
	if s.store != nil {
		invitation, tokenHash, ok, err := s.store.GetInvitation(ctx, invitationID)
		if err != nil {
			return domain.Membership{}, err
		}
		if !ok {
			return domain.Membership{}, ErrNotFound
		}
		now := s.now().UTC()
		if invitation.AcceptedAt != nil || !now.Before(invitation.ExpiresAt) || hashToken(token) != tokenHash {
			return domain.Membership{}, ErrNotFound
		}
		member, ok, err := s.store.AcceptInvitation(ctx, invitationID, userID, now)
		if err != nil {
			return domain.Membership{}, err
		}
		if !ok {
			return domain.Membership{}, ErrNotFound
		}
		return member, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.invitations[invitationID]
	if !ok {
		return domain.Membership{}, ErrNotFound
	}
	now := s.now().UTC()
	if record.invitation.AcceptedAt != nil || !now.Before(record.invitation.ExpiresAt) {
		return domain.Membership{}, ErrNotFound
	}
	if hashToken(token) != record.tokenHash {
		return domain.Membership{}, ErrNotFound
	}
	member := domain.Membership{
		OrganizationID: record.invitation.OrganizationID,
		UserID:         userID,
		Role:           record.invitation.Role,
		CreatedAt:      now,
	}
	if s.memberships[member.OrganizationID] == nil {
		s.memberships[member.OrganizationID] = map[string]domain.Membership{}
	}
	s.memberships[member.OrganizationID][userID] = member
	record.invitation.AcceptedAt = &now
	s.invitations[invitationID] = record
	return member, nil
}

func (s *TenancyService) HasRole(ctx context.Context, organizationID, actorID string, required domain.Role) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	organizationID = strings.TrimSpace(organizationID)
	actorID = strings.TrimSpace(actorID)
	if organizationID == "" || actorID == "" || !required.Valid() {
		return false, ErrValidation
	}
	if s.store != nil {
		return s.store.HasRole(ctx, organizationID, actorID, required)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasRoleLocked(organizationID, actorID, required), nil
}

func (s *TenancyService) hasRoleLocked(organizationID, actorID string, required domain.Role) bool {
	members := s.memberships[organizationID]
	if members == nil {
		return false
	}
	member, ok := members[actorID]
	return ok && member.Role.Allows(required)
}

func randomToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func randomPrefixedID(prefix string) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return strings.TrimSpace(prefix) + "_" + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}
