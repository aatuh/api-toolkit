package identity

import (
	"context"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/ports"
)

// Config controls identity defaults and validation.
type Config struct {
	DefaultLanguage  string
	AllowedLanguages []string
}

type configState struct {
	defaultLanguage string
	allowed         map[string]struct{}
}

// Service encapsulates reusable identity lifecycle logic.
type Service struct {
	repo    Repo
	tx      ports.TxManager
	log     ports.Logger
	clk     ports.Clock
	ids     ports.IDGen
	cfg     configState
	timeout time.Duration
}

// DefaultConfig returns a baseline configuration for identity services.
func DefaultConfig() Config {
	return Config{
		DefaultLanguage:  "en",
		AllowedLanguages: nil,
	}
}

// New constructs a Service with configurable defaults.
func New(repo Repo, tx ports.TxManager, log ports.Logger, clk ports.Clock, ids ports.IDGen, cfg Config) *Service {
	allowed := make(map[string]struct{}, len(cfg.AllowedLanguages))
	for _, lang := range cfg.AllowedLanguages {
		key := strings.ToLower(strings.TrimSpace(lang))
		if key == "" {
			continue
		}
		allowed[key] = struct{}{}
	}
	defaultLang := strings.ToLower(strings.TrimSpace(cfg.DefaultLanguage))
	if defaultLang == "" {
		defaultLang = "en"
	}
	if len(allowed) > 0 {
		allowed[defaultLang] = struct{}{}
	}
	return &Service{
		repo:    repo,
		tx:      tx,
		log:     log,
		clk:     clk,
		ids:     ids,
		cfg:     configState{defaultLanguage: defaultLang, allowed: allowed},
		timeout: 5 * time.Second,
	}
}

// NewDefault constructs a Service with baseline defaults.
func NewDefault(repo Repo, tx ports.TxManager, log ports.Logger, clk ports.Clock, ids ports.IDGen) *Service {
	return New(repo, tx, log, clk, ids, DefaultConfig())
}

// EnsureUser maps an external identity to a local profile, creating one on first login.
func (s *Service) EnsureUser(ctx context.Context, in EnsureInput) (*User, error) {
	provider := normalizeProvider(in.Provider)
	subject := strings.TrimSpace(in.Subject)
	if provider == "" || subject == "" {
		return nil, ErrInvalid
	}
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()

	lang := s.normalizeLanguage(in.Language)
	roles := normalizeRoles(in.DefaultRoles)
	var out *User
	if err := s.tx.WithinTx(ctx, func(txCtx context.Context) error {
		cur, err := s.repo.GetByIdentity(txCtx, provider, subject)
		switch {
		case err == nil:
			updated := applyIdentityUpdates(cur, in, lang)
			if updated {
				cur.UpdatedAt = s.clk.Now()
				if err := s.repo.Update(txCtx, cur); err != nil {
					return err
				}
			}
			roles, err := s.repo.ListRoles(txCtx, cur.ID)
			if err != nil {
				return err
			}
			cur.Roles = roles
			out = cur
			return nil
		case err == ErrNotFound:
			now := s.clk.Now()
			user := &User{
				ID:                s.ids.New(),
				Provider:          provider,
				Subject:           subject,
				Email:             normalizeEmail(in.Email),
				FirstName:         strings.TrimSpace(in.FirstName),
				LastName:          strings.TrimSpace(in.LastName),
				PreferredLanguage: lang,
				CreatedAt:         now,
				UpdatedAt:         now,
			}
			if err := s.repo.Create(txCtx, user); err != nil {
				return err
			}
			if len(roles) > 0 {
				if err := s.repo.ReplaceRoles(txCtx, user.ID, roles, now); err != nil {
					return err
				}
				user.Roles = roles
			}
			out = user
			return nil
		default:
			return err
		}
	}); err != nil {
		return nil, s.mapErr(err)
	}
	return out, nil
}

// Get returns a user by internal ID.
func (s *Service) Get(ctx context.Context, id string) (*User, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrInvalid
	}
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()

	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, s.mapErr(err)
	}
	roles, err := s.repo.ListRoles(ctx, id)
	if err != nil {
		return nil, s.mapErr(err)
	}
	user.Roles = roles
	return user, nil
}

// UpdateProfile mutates allowed profile fields.
func (s *Service) UpdateProfile(ctx context.Context, in UpdateProfileInput) (*User, error) {
	in.UserID = strings.TrimSpace(in.UserID)
	if in.UserID == "" || in.PreferredLanguage == nil {
		return nil, ErrInvalid
	}
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()

	lang := s.normalizeLanguage(*in.PreferredLanguage)
	var out *User
	if err := s.tx.WithinTx(ctx, func(txCtx context.Context) error {
		cur, err := s.repo.GetByID(txCtx, in.UserID)
		if err != nil {
			return err
		}
		if lang != "" && lang != cur.PreferredLanguage {
			cur.PreferredLanguage = lang
			cur.UpdatedAt = s.clk.Now()
			if err := s.repo.Update(txCtx, cur); err != nil {
				return err
			}
		}
		roles, err := s.repo.ListRoles(txCtx, cur.ID)
		if err != nil {
			return err
		}
		cur.Roles = roles
		out = cur
		return nil
	}); err != nil {
		return nil, s.mapErr(err)
	}
	return out, nil
}

// ReplaceRoles overwrites the roles assigned to a user.
func (s *Service) ReplaceRoles(ctx context.Context, in UpdateRolesInput) (*User, error) {
	in.UserID = strings.TrimSpace(in.UserID)
	if in.UserID == "" {
		return nil, ErrInvalid
	}
	roles := normalizeRoles(in.Roles)
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()

	var out *User
	if err := s.tx.WithinTx(ctx, func(txCtx context.Context) error {
		cur, err := s.repo.GetByID(txCtx, in.UserID)
		if err != nil {
			return err
		}
		now := s.clk.Now()
		if err := s.repo.ReplaceRoles(txCtx, in.UserID, roles, now); err != nil {
			return err
		}
		cur.Roles = roles
		cur.UpdatedAt = now
		if err := s.repo.Update(txCtx, cur); err != nil {
			return err
		}
		out = cur
		return nil
	}); err != nil {
		return nil, s.mapErr(err)
	}
	return out, nil
}

func (s *Service) normalizeLanguage(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "" {
		return s.cfg.defaultLanguage
	}
	if len(s.cfg.allowed) > 0 {
		if _, ok := s.cfg.allowed[lang]; !ok {
			return s.cfg.defaultLanguage
		}
	}
	return lang
}

func (s *Service) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, s.timeout)
}

func (s *Service) mapErr(err error) error {
	switch err {
	case nil:
		return nil
	case ErrInvalid, ErrNotFound, ErrConflict:
		return err
	default:
		return ErrInternal
	}
}

func normalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func normalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

func normalizeRoles(input []string) []string {
	if len(input) == 0 {
		return []string{}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(input))
	for _, role := range input {
		key := strings.ToLower(strings.TrimSpace(role))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func applyIdentityUpdates(cur *User, in EnsureInput, lang string) bool {
	updated := false
	if email := normalizeEmail(in.Email); email != "" && email != cur.Email {
		cur.Email = email
		updated = true
	}
	if fn := strings.TrimSpace(in.FirstName); fn != "" && fn != cur.FirstName {
		cur.FirstName = fn
		updated = true
	}
	if ln := strings.TrimSpace(in.LastName); ln != "" && ln != cur.LastName {
		cur.LastName = ln
		updated = true
	}
	if lang != "" && lang != cur.PreferredLanguage {
		cur.PreferredLanguage = lang
		updated = true
	}
	return updated
}
