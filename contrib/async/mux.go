package async

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	// ErrInvalidJobKind reports a missing or unsafe async job kind.
	ErrInvalidJobKind = errors.New("invalid async job kind")
	// ErrHandlerNotFound reports a missing async handler registration.
	ErrHandlerNotFound = errors.New("async handler not found")
)

// HandlerRoute registers a handler for a low-cardinality job kind.
type HandlerRoute struct {
	Kind    string
	Handler Handler
}

// HandlerMux dispatches jobs to handlers by sanitized low-cardinality kind.
type HandlerMux struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

// NewHandlerMux constructs a mux and registers the supplied routes.
func NewHandlerMux(routes ...HandlerRoute) (*HandlerMux, error) {
	mux := &HandlerMux{handlers: map[string]Handler{}}
	for _, route := range routes {
		if err := mux.Register(route.Kind, route.Handler); err != nil {
			return nil, err
		}
	}
	return mux, nil
}

// Register adds or replaces a handler for kind.
func (m *HandlerMux) Register(kind string, handler Handler) error {
	if handler == nil {
		return ErrHandlerNotFound
	}
	kind, err := normalizeJobKind(kind)
	if err != nil {
		return err
	}
	if m == nil {
		return ErrHandlerNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.handlers == nil {
		m.handlers = map[string]Handler{}
	}
	m.handlers[kind] = handler
	return nil
}

// Handle dispatches job to the registered handler for job.Kind.
func (m *HandlerMux) Handle(ctx context.Context, job Job) error {
	if m == nil {
		return ErrHandlerNotFound
	}
	kind, err := normalizeJobKind(job.Kind)
	if err != nil {
		return err
	}
	m.mu.RLock()
	handler := m.handlers[kind]
	m.mu.RUnlock()
	if handler == nil {
		return fmt.Errorf("%w: %s", ErrHandlerNotFound, kind)
	}
	return handler.Handle(ctx, job)
}

// Kinds returns registered job kinds in sorted order.
func (m *HandlerMux) Kinds() []string {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	kinds := make([]string, 0, len(m.handlers))
	for kind := range m.handlers {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

func normalizeJobKind(kind string) (string, error) {
	if strings.TrimSpace(kind) == "" {
		return "", ErrInvalidJobKind
	}
	safe := SafeLabel(kind)
	if safe == "" || safe == "unknown" {
		return "", ErrInvalidJobKind
	}
	return safe, nil
}

var _ Handler = (*HandlerMux)(nil)
