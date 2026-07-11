package cedar

import (
	"context"
	"errors"
	"fmt"
	"strings"

	cedarcore "github.com/cedar-policy/cedar-go"

	"github.com/aatuh/api-toolkit/v4/authorization"
)

// Config configures the Cedar adapter.
type Config struct {
	Policies       cedarcore.PolicyIterator
	Entities       cedarcore.EntityGetter
	PrincipalType  string
	ResourceType   string
	ActionType     string
	RequestBuilder RequestBuilder
}

// RequestBuilder converts a policy request into a Cedar request.
type RequestBuilder func(req authorization.PolicyRequest) (cedarcore.Request, error)

// Engine evaluates policies using cedar-go.
type Engine struct {
	policies cedarcore.PolicyIterator
	entities cedarcore.EntityGetter
	build    RequestBuilder
}

// New creates a new Cedar adapter.
func New(cfg Config) (*Engine, error) {
	if cfg.Policies == nil {
		return nil, errors.New("cedar policies are required")
	}
	if cfg.ActionType == "" {
		cfg.ActionType = "Action"
	}
	builder := cfg.RequestBuilder
	if builder == nil {
		builder = defaultRequestBuilder(cfg)
	}
	return &Engine{
		policies: cfg.Policies,
		entities: cfg.Entities,
		build:    builder,
	}, nil
}

// Evaluate evaluates a policy request.
func (e *Engine) Evaluate(ctx context.Context, req authorization.PolicyRequest) (authorization.PolicyDecision, error) {
	if e == nil {
		return authorization.PolicyDecision{}, errors.New("cedar engine is nil")
	}
	cedarReq, err := e.build(req)
	if err != nil {
		return authorization.PolicyDecision{}, err
	}
	decision, diag := cedarcore.Authorize(e.policies, e.entities, cedarReq)
	if len(diag.Errors) > 0 {
		errs := make([]error, 0, len(diag.Errors))
		for _, diagErr := range diag.Errors {
			errs = append(errs, fmt.Errorf("policy %s: %s", diagErr.PolicyID, diagErr.Message))
		}
		return authorization.PolicyDecision{}, errors.Join(errs...)
	}
	return authorization.PolicyDecision{Allow: decision == cedarcore.Allow, Data: diag}, nil
}

func defaultRequestBuilder(cfg Config) RequestBuilder {
	return func(req authorization.PolicyRequest) (cedarcore.Request, error) {
		principal, err := entityUIDFromValue(req.Subject, cfg.PrincipalType)
		if err != nil {
			return cedarcore.Request{}, fmt.Errorf("principal: %w", err)
		}
		resource, err := entityUIDFromValue(req.Resource, cfg.ResourceType)
		if err != nil {
			return cedarcore.Request{}, fmt.Errorf("resource: %w", err)
		}
		action, err := actionUIDFromString(req.Action, cfg.ActionType)
		if err != nil {
			return cedarcore.Request{}, fmt.Errorf("action: %w", err)
		}
		ctxRecord, err := recordFromContext(req.Context)
		if err != nil {
			return cedarcore.Request{}, fmt.Errorf("context: %w", err)
		}
		return cedarcore.Request{
			Principal: principal,
			Action:    action,
			Resource:  resource,
			Context:   ctxRecord,
		}, nil
	}
}

func actionUIDFromString(action, defaultType string) (cedarcore.EntityUID, error) {
	action = strings.TrimSpace(action)
	if action == "" {
		return cedarcore.EntityUID{}, errors.New("action is required")
	}
	uid, err := parseEntityUID(action)
	if err == nil {
		return uid, nil
	}
	if defaultType == "" {
		return cedarcore.EntityUID{}, errors.New("action type is required")
	}
	return cedarcore.NewEntityUID(cedarcore.EntityType(defaultType), cedarcore.String(action)), nil
}

func entityUIDFromValue(value any, defaultType string) (cedarcore.EntityUID, error) {
	switch v := value.(type) {
	case cedarcore.EntityUID:
		if v.IsZero() {
			return cedarcore.EntityUID{}, errors.New("entity uid is empty")
		}
		return v, nil
	case *cedarcore.EntityUID:
		if v == nil || v.IsZero() {
			return cedarcore.EntityUID{}, errors.New("entity uid is empty")
		}
		return *v, nil
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return cedarcore.EntityUID{}, errors.New("entity uid is empty")
		}
		uid, err := parseEntityUID(v)
		if err == nil {
			return uid, nil
		}
		if defaultType == "" {
			return cedarcore.EntityUID{}, errors.New("entity type is required")
		}
		return cedarcore.NewEntityUID(cedarcore.EntityType(defaultType), cedarcore.String(v)), nil
	default:
		return cedarcore.EntityUID{}, fmt.Errorf("unsupported entity uid type %T", value)
	}
}

func parseEntityUID(raw string) (cedarcore.EntityUID, error) {
	var uid cedarcore.EntityUID
	if err := uid.UnmarshalCedar([]byte(raw)); err != nil {
		return cedarcore.EntityUID{}, err
	}
	return uid, nil
}

func recordFromContext(ctx any) (cedarcore.Record, error) {
	if ctx == nil {
		return cedarcore.Record{}, nil
	}
	switch v := ctx.(type) {
	case cedarcore.Record:
		return v, nil
	case cedarcore.RecordMap:
		return cedarcore.NewRecord(v), nil
	case map[string]cedarcore.Value:
		return cedarcore.NewRecord(stringKeyedRecord(v)), nil
	case map[string]any:
		return RecordFromMap(v)
	default:
		return cedarcore.Record{}, fmt.Errorf("unsupported context type %T", ctx)
	}
}

func stringKeyedRecord(input map[string]cedarcore.Value) cedarcore.RecordMap {
	out := make(cedarcore.RecordMap, len(input))
	for k, v := range input {
		out[cedarcore.String(k)] = v
	}
	return out
}
