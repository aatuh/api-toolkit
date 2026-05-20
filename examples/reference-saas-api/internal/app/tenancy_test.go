package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"example.com/reference-saas-api/internal/domain"
)

func TestTenancyServiceCreatesOrganizationWithOwnerMembership(t *testing.T) {
	service := NewTenancyService()
	service.now = fixedTenancyTime

	org, member, err := service.CreateOrganization(context.Background(), "user_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	if org.ID == "" || org.Name != "Acme" {
		t.Fatalf("organization = %#v", org)
	}
	if member.OrganizationID != org.ID || member.UserID != "user_1" || member.Role != domain.RoleOwner {
		t.Fatalf("membership = %#v", member)
	}
	orgs, err := service.ListOrganizations(context.Background(), "user_1")
	if err != nil {
		t.Fatalf("ListOrganizations() error = %v", err)
	}
	if len(orgs) != 1 || orgs[0].ID != org.ID {
		t.Fatalf("organizations = %#v", orgs)
	}
}

func TestTenancyServiceInvitationHashesTokenAndAcceptsOnce(t *testing.T) {
	service := NewTenancyService()
	service.now = fixedTenancyTime
	service.newToken = func() (string, error) { return "invite-token-value", nil }
	org, _, err := service.CreateOrganization(context.Background(), "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}

	invitation, token, err := service.InviteMember(context.Background(), "owner_1", org.ID, "Member@Example.com", domain.RoleMember)
	if err != nil {
		t.Fatalf("InviteMember() error = %v", err)
	}
	if token != "invite-token-value" {
		t.Fatalf("token = %q", token)
	}
	record := service.invitations[invitation.ID]
	if record.tokenHash == "" || record.tokenHash == token {
		t.Fatalf("invitation token hash was not stored safely: %#v", record)
	}
	if invitation.Email != "member@example.com" || invitation.TokenPrefix == "" {
		t.Fatalf("invitation = %#v", invitation)
	}
	if _, err := service.AcceptInvitation(context.Background(), invitation.ID, "wrong-token", "user_2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong token error = %v, want %v", err, ErrNotFound)
	}
	member, err := service.AcceptInvitation(context.Background(), invitation.ID, token, "user_2")
	if err != nil {
		t.Fatalf("AcceptInvitation() error = %v", err)
	}
	if member.OrganizationID != org.ID || member.UserID != "user_2" || member.Role != domain.RoleMember {
		t.Fatalf("accepted member = %#v", member)
	}
	if _, err := service.AcceptInvitation(context.Background(), invitation.ID, token, "user_3"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("replay accept error = %v, want %v", err, ErrNotFound)
	}
}

func TestTenancyServiceEnforcesRoleChecks(t *testing.T) {
	service := NewTenancyService()
	service.now = fixedTenancyTime
	service.newToken = func() (string, error) { return "invite-token-value", nil }
	org, _, err := service.CreateOrganization(context.Background(), "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	invitation, token, err := service.InviteMember(context.Background(), "owner_1", org.ID, "viewer@example.com", domain.RoleViewer)
	if err != nil {
		t.Fatalf("InviteMember() error = %v", err)
	}
	if _, err := service.AcceptInvitation(context.Background(), invitation.ID, token, "viewer_1"); err != nil {
		t.Fatalf("AcceptInvitation() error = %v", err)
	}
	if _, _, err := service.InviteMember(context.Background(), "viewer_1", org.ID, "member@example.com", domain.RoleMember); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer invite error = %v, want %v", err, ErrForbidden)
	}
	ok, err := service.HasRole(context.Background(), org.ID, "viewer_1", domain.RoleViewer)
	if err != nil {
		t.Fatalf("HasRole() error = %v", err)
	}
	if !ok {
		t.Fatal("viewer should satisfy viewer role")
	}
	ok, err = service.HasRole(context.Background(), org.ID, "viewer_1", domain.RoleAdmin)
	if err != nil {
		t.Fatalf("HasRole() admin error = %v", err)
	}
	if ok {
		t.Fatal("viewer should not satisfy admin role")
	}
}

func TestTenancyServiceWithStorePersistsInvitationHash(t *testing.T) {
	store := newRecordingTenancyStore()
	service := NewTenancyServiceWithStore(store)
	service.now = fixedTenancyTime
	service.newOrgID = func() (string, error) { return "org_store_1", nil }
	service.newInviteID = func() (string, error) { return "inv_store_1", nil }
	service.newToken = func() (string, error) { return "stored-invitation-token", nil }

	org, owner, err := service.CreateOrganization(context.Background(), "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	if org.ID != "org_store_1" || owner.Role != domain.RoleOwner {
		t.Fatalf("org=%#v owner=%#v", org, owner)
	}
	invitation, token, err := service.InviteMember(context.Background(), "owner_1", org.ID, "Member@Example.com", domain.RoleMember)
	if err != nil {
		t.Fatalf("InviteMember() error = %v", err)
	}
	if store.invitationHashes[invitation.ID] == "" || strings.Contains(store.invitationHashes[invitation.ID], token) {
		t.Fatalf("stored token hash leaked token: %q", store.invitationHashes[invitation.ID])
	}
	member, err := service.AcceptInvitation(context.Background(), invitation.ID, token, "member_1")
	if err != nil {
		t.Fatalf("AcceptInvitation() error = %v", err)
	}
	if member.OrganizationID != org.ID || member.Role != domain.RoleMember {
		t.Fatalf("accepted member = %#v", member)
	}
	members, err := service.ListMembers(context.Background(), "owner_1", org.ID)
	if err != nil {
		t.Fatalf("ListMembers() error = %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("members = %#v", members)
	}
	ok, err := service.HasRole(context.Background(), org.ID, "member_1", domain.RoleMember)
	if err != nil || !ok {
		t.Fatalf("HasRole() ok=%v err=%v", ok, err)
	}
}

type recordingTenancyStore struct {
	orgs             map[string]domain.Organization
	memberships      map[string]map[string]domain.Membership
	invitations      map[string]domain.Invitation
	invitationHashes map[string]string
}

func newRecordingTenancyStore() *recordingTenancyStore {
	return &recordingTenancyStore{
		orgs:             map[string]domain.Organization{},
		memberships:      map[string]map[string]domain.Membership{},
		invitations:      map[string]domain.Invitation{},
		invitationHashes: map[string]string{},
	}
}

func (s *recordingTenancyStore) CreateOrganization(_ context.Context, org domain.Organization, owner domain.Membership) error {
	s.orgs[org.ID] = org
	if s.memberships[org.ID] == nil {
		s.memberships[org.ID] = map[string]domain.Membership{}
	}
	s.memberships[org.ID][owner.UserID] = owner
	return nil
}

func (s *recordingTenancyStore) ListOrganizations(_ context.Context, actorID string) ([]domain.Organization, error) {
	var out []domain.Organization
	for orgID, members := range s.memberships {
		if _, ok := members[actorID]; ok {
			out = append(out, s.orgs[orgID])
		}
	}
	return out, nil
}

func (s *recordingTenancyStore) ListMembers(_ context.Context, organizationID string) ([]domain.Membership, error) {
	var out []domain.Membership
	for _, member := range s.memberships[organizationID] {
		out = append(out, member)
	}
	return out, nil
}

func (s *recordingTenancyStore) CreateInvitation(_ context.Context, invitation domain.Invitation, tokenHash string) error {
	s.invitations[invitation.ID] = invitation
	s.invitationHashes[invitation.ID] = tokenHash
	return nil
}

func (s *recordingTenancyStore) GetInvitation(_ context.Context, invitationID string) (domain.Invitation, string, bool, error) {
	invitation, ok := s.invitations[invitationID]
	return invitation, s.invitationHashes[invitationID], ok, nil
}

func (s *recordingTenancyStore) AcceptInvitation(_ context.Context, invitationID, userID string, acceptedAt time.Time) (domain.Membership, bool, error) {
	invitation, ok := s.invitations[invitationID]
	if !ok || invitation.AcceptedAt != nil {
		return domain.Membership{}, false, nil
	}
	invitation.AcceptedAt = &acceptedAt
	s.invitations[invitationID] = invitation
	member := domain.Membership{OrganizationID: invitation.OrganizationID, UserID: userID, Role: invitation.Role, CreatedAt: acceptedAt}
	if s.memberships[invitation.OrganizationID] == nil {
		s.memberships[invitation.OrganizationID] = map[string]domain.Membership{}
	}
	s.memberships[invitation.OrganizationID][userID] = member
	return member, true, nil
}

func (s *recordingTenancyStore) HasRole(_ context.Context, organizationID, actorID string, required domain.Role) (bool, error) {
	member, ok := s.memberships[organizationID][actorID]
	return ok && member.Role.Allows(required), nil
}

func fixedTenancyTime() time.Time {
	return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
}
