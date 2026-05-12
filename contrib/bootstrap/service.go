package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/aatuh/api-toolkit/v2/ports"
)

// StartupCheck validates application wiring before the service starts.
type StartupCheck struct {
	Name  string
	Check func(context.Context) error
}

// APIServiceConfig configures a reusable production API composition root.
type APIServiceConfig struct {
	Addr                    string
	Log                     ports.Logger
	Router                  ports.HTTPRouter
	RegisterRoutes          func(ports.HTTPRouter) error
	SystemEndpoints         SystemEndpoints
	Admin                   SystemEndpointAdminOptions
	MiddlewareOrder         []MiddlewareStage
	RequiredMiddlewareOrder []MiddlewareStage
	ServerOptions           []ServerOption
	StartupChecks           []StartupCheck
}

// APIService is a reusable HTTP API composition root for generated services.
type APIService struct {
	addr          string
	log           ports.Logger
	router        ports.HTTPRouter
	serverOptions []ServerOption
}

// NewAPIService validates wiring, builds default router/profile when needed,
// mounts routes and system endpoints, and returns a runnable service.
func NewAPIService(config APIServiceConfig) (*APIService, error) {
	log := config.Log
	if log == nil {
		log = ports.NopLogger{}
	}
	if err := runStartupChecks(context.Background(), config.StartupChecks); err != nil {
		return nil, err
	}
	if err := validateConfiguredMiddlewareOrder(config.MiddlewareOrder, config.RequiredMiddlewareOrder); err != nil {
		return nil, err
	}

	router := config.Router
	if router == nil {
		var err error
		router, err = NewDefaultRouter(log)
		if err != nil {
			return nil, err
		}
	}
	if config.RegisterRoutes != nil {
		if err := config.RegisterRoutes(router); err != nil {
			return nil, fmt.Errorf("register routes: %w", err)
		}
	}
	if hasSystemEndpoints(config.SystemEndpoints) {
		if config.Admin.RequireAdmin == nil {
			return nil, errors.New("api service system endpoints require an admin wrapper")
		}
		if err := MountSystemEndpointsToWithAdmin(router, config.SystemEndpoints, config.Admin); err != nil {
			return nil, err
		}
	}

	addr := strings.TrimSpace(config.Addr)
	if addr == "" {
		addr = ":8080"
	}
	return &APIService{
		addr:          addr,
		log:           log,
		router:        router,
		serverOptions: append([]ServerOption(nil), config.ServerOptions...),
	}, nil
}

// Handler returns the composed HTTP handler.
func (s *APIService) Handler() http.Handler {
	if s == nil || s.router == nil {
		return http.NewServeMux()
	}
	return s.router
}

// Start starts the HTTP server and shuts down when ctx is canceled.
func (s *APIService) Start(ctx context.Context) error {
	if s == nil {
		return errors.New("api service is nil")
	}
	if s.log == nil {
		s.log = ports.NopLogger{}
	}
	s.log.Info("http server starting", "addr", s.addr)
	return runServer(ctx, HardenedServer(s.addr, s.Handler(), s.serverOptions...))
}

func runStartupChecks(ctx context.Context, checks []StartupCheck) error {
	for _, check := range checks {
		if check.Check == nil {
			continue
		}
		name := strings.TrimSpace(check.Name)
		if name == "" {
			name = "unnamed"
		}
		if err := check.Check(ctx); err != nil {
			return fmt.Errorf("startup check %s: %w", name, err)
		}
	}
	return nil
}

func validateConfiguredMiddlewareOrder(order, required []MiddlewareStage) error {
	if len(order) == 0 && len(required) == 0 {
		return nil
	}
	if len(required) == 0 {
		required = StrictAPIMiddlewareOrder()
	}
	if err := ValidateMiddlewareOrder(order, required...); err != nil {
		return fmt.Errorf("middleware order: %w", err)
	}
	return nil
}

func hasSystemEndpoints(endpoints SystemEndpoints) bool {
	return endpoints.Health != nil ||
		endpoints.Docs != nil ||
		endpoints.Version != nil ||
		endpoints.Pprof != nil ||
		endpoints.Metrics != nil
}
