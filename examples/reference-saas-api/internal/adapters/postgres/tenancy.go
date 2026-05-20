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
	ErrTenancyStoreRequired = errors.New("postgres tenancy store db is required")
	ErrTenancyInvalid       = errors.New("postgres tenancy record is invalid")
)

type TenancyDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type TenancyStore struct {
	db TenancyDB
}

func NewTenancyStore(db TenancyDB) *TenancyStore {
	return &TenancyStore{db: db}
}

func (s *TenancyStore) CreateOrganization(ctx context.Context, org domain.Organization, owner domain.Membership) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return ErrTenancyStoreRequired
	}
	org.ID = strings.TrimSpace(org.ID)
	org.Name = strings.TrimSpace(org.Name)
	owner.OrganizationID = strings.TrimSpace(owner.OrganizationID)
	owner.UserID = strings.TrimSpace(owner.UserID)
	if org.ID == "" || org.Name == "" || owner.OrganizationID != org.ID || owner.UserID == "" || owner.Role != domain.RoleOwner || org.CreatedAt.IsZero() || org.UpdatedAt.IsZero() || owner.CreatedAt.IsZero() {
		return ErrTenancyInvalid
	}
	_, err := s.db.Exec(ctx,
		"with inserted_org as (insert into organizations (id, name, created_at, updated_at) values ($1, $2, $3, $4) returning id) "+
			"insert into memberships (organization_id, user_id, role, created_at) select id, $5, $6, $7 from inserted_org",
		org.ID,
		org.Name,
		org.CreatedAt.UTC(),
		org.UpdatedAt.UTC(),
		owner.UserID,
		string(owner.Role),
		owner.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("create organization: %w", err)
	}
	return nil
}

func (s *TenancyStore) ListOrganizations(ctx context.Context, actorID string) ([]domain.Organization, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, ErrTenancyStoreRequired
	}
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return nil, ErrTenancyInvalid
	}
	rows, err := s.db.Query(ctx,
		"select o.id, o.name, o.created_at, o.updated_at from organizations o join memberships m on m.organization_id=o.id where m.user_id=$1 order by o.id",
		actorID,
	)
	if err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}
	defer rows.Close()
	var out []domain.Organization
	for rows.Next() {
		org, err := scanOrganization(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, org)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list organization rows: %w", err)
	}
	return out, nil
}

func (s *TenancyStore) ListMembers(ctx context.Context, organizationID string) ([]domain.Membership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, ErrTenancyStoreRequired
	}
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return nil, ErrTenancyInvalid
	}
	rows, err := s.db.Query(ctx,
		"select organization_id, user_id, role, created_at from memberships where organization_id=$1 order by user_id",
		organizationID,
	)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()
	var out []domain.Membership
	for rows.Next() {
		member, err := scanMembership(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list member rows: %w", err)
	}
	return out, nil
}

func (s *TenancyStore) CreateInvitation(ctx context.Context, invitation domain.Invitation, tokenHash string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return ErrTenancyStoreRequired
	}
	invitation.ID = strings.TrimSpace(invitation.ID)
	invitation.OrganizationID = strings.TrimSpace(invitation.OrganizationID)
	invitation.Email = strings.ToLower(strings.TrimSpace(invitation.Email))
	hashBytes, err := decodeTokenHash(tokenHash)
	if err != nil {
		return err
	}
	if invitation.ID == "" || invitation.OrganizationID == "" || invitation.Email == "" || !strings.Contains(invitation.Email, "@") || !invitation.Role.Valid() || invitation.ExpiresAt.IsZero() || invitation.CreatedAt.IsZero() {
		return ErrTenancyInvalid
	}
	if _, err := s.db.Exec(ctx,
		"insert into invitations (id, organization_id, email, role, token_hash, expires_at, created_at) values ($1, $2, $3, $4, $5, $6, $7)",
		invitation.ID,
		invitation.OrganizationID,
		invitation.Email,
		string(invitation.Role),
		hashBytes,
		invitation.ExpiresAt.UTC(),
		invitation.CreatedAt.UTC(),
	); err != nil {
		return fmt.Errorf("create invitation: %w", err)
	}
	return nil
}

func (s *TenancyStore) GetInvitation(ctx context.Context, invitationID string) (domain.Invitation, string, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.Invitation{}, "", false, err
	}
	if s == nil || s.db == nil {
		return domain.Invitation{}, "", false, ErrTenancyStoreRequired
	}
	invitationID = strings.TrimSpace(invitationID)
	if invitationID == "" {
		return domain.Invitation{}, "", false, ErrTenancyInvalid
	}
	row := s.db.QueryRow(ctx,
		"select id, organization_id, email, role, token_hash, expires_at, accepted_at, created_at from invitations where id=$1",
		invitationID,
	)
	invitation, tokenHash, err := scanInvitation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Invitation{}, "", false, nil
	}
	if err != nil {
		return domain.Invitation{}, "", false, fmt.Errorf("get invitation: %w", err)
	}
	return invitation, tokenHash, true, nil
}

func (s *TenancyStore) AcceptInvitation(ctx context.Context, invitationID, userID string, acceptedAt time.Time) (domain.Membership, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.Membership{}, false, err
	}
	if s == nil || s.db == nil {
		return domain.Membership{}, false, ErrTenancyStoreRequired
	}
	invitationID = strings.TrimSpace(invitationID)
	userID = strings.TrimSpace(userID)
	if invitationID == "" || userID == "" || acceptedAt.IsZero() {
		return domain.Membership{}, false, ErrTenancyInvalid
	}
	row := s.db.QueryRow(ctx,
		"with accepted as (update invitations set accepted_at=$2 where id=$1 and accepted_at is null returning organization_id, role) "+
			"insert into memberships (organization_id, user_id, role, created_at) "+
			"select organization_id, $3, role, $2 from accepted "+
			"on conflict (organization_id, user_id) do update set role=excluded.role "+
			"returning organization_id, user_id, role, created_at",
		invitationID,
		acceptedAt.UTC(),
		userID,
	)
	member, err := scanMembership(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Membership{}, false, nil
	}
	if err != nil {
		return domain.Membership{}, false, fmt.Errorf("accept invitation: %w", err)
	}
	return member, true, nil
}

func (s *TenancyStore) HasRole(ctx context.Context, organizationID, actorID string, required domain.Role) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if s == nil || s.db == nil {
		return false, ErrTenancyStoreRequired
	}
	organizationID = strings.TrimSpace(organizationID)
	actorID = strings.TrimSpace(actorID)
	if organizationID == "" || actorID == "" || !required.Valid() {
		return false, ErrTenancyInvalid
	}
	var role string
	if err := s.db.QueryRow(ctx, "select role from memberships where organization_id=$1 and user_id=$2", organizationID, actorID).Scan(&role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check role: %w", err)
	}
	got := domain.Role(role)
	if !got.Valid() {
		return false, ErrTenancyInvalid
	}
	return got.Allows(required), nil
}

type tenancyScanner interface {
	Scan(dest ...any) error
}

func scanOrganization(row tenancyScanner) (domain.Organization, error) {
	var org domain.Organization
	if err := row.Scan(&org.ID, &org.Name, &org.CreatedAt, &org.UpdatedAt); err != nil {
		return domain.Organization{}, err
	}
	org.CreatedAt = org.CreatedAt.UTC()
	org.UpdatedAt = org.UpdatedAt.UTC()
	return org, nil
}

func scanMembership(row tenancyScanner) (domain.Membership, error) {
	var (
		member domain.Membership
		role   string
	)
	if err := row.Scan(&member.OrganizationID, &member.UserID, &role, &member.CreatedAt); err != nil {
		return domain.Membership{}, err
	}
	member.Role = domain.Role(role)
	if !member.Role.Valid() {
		return domain.Membership{}, ErrTenancyInvalid
	}
	member.CreatedAt = member.CreatedAt.UTC()
	return member, nil
}

func scanInvitation(row tenancyScanner) (domain.Invitation, string, error) {
	var (
		invitation domain.Invitation
		role       string
		tokenHash  []byte
		acceptedAt pgtype.Timestamptz
	)
	if err := row.Scan(&invitation.ID, &invitation.OrganizationID, &invitation.Email, &role, &tokenHash, &invitation.ExpiresAt, &acceptedAt, &invitation.CreatedAt); err != nil {
		return domain.Invitation{}, "", err
	}
	invitation.Role = domain.Role(role)
	if !invitation.Role.Valid() || len(tokenHash) != sha256HashSize {
		return domain.Invitation{}, "", ErrTenancyInvalid
	}
	invitation.AcceptedAt = tenancyNullableTimestamptz(acceptedAt)
	invitation.ExpiresAt = invitation.ExpiresAt.UTC()
	invitation.CreatedAt = invitation.CreatedAt.UTC()
	return invitation, hex.EncodeToString(tokenHash), nil
}

func decodeTokenHash(hash string) ([]byte, error) {
	decoded, err := hex.DecodeString(strings.TrimSpace(hash))
	if err != nil || len(decoded) != sha256HashSize {
		return nil, ErrTenancyInvalid
	}
	return decoded, nil
}

func tenancyNullableTimestamptz(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}

const sha256HashSize = 32
