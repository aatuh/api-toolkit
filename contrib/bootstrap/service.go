package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aatuh/api-toolkit/v2/ports"
)

// StartupCheck validates application wiring before the service starts.
type StartupCheck struct {
	Name  string
	Check func(context.Context) error
}

// ShutdownHook releases resources when APIService.Start returns after a
// graceful shutdown or server failure.
type ShutdownHook struct {
	Name string
	Hook func(context.Context) error
}

// BackgroundTask runs with the API service lifecycle. Returning a non-context
// cancellation error fails the service and starts graceful shutdown.
type BackgroundTask struct {
	Name string
	Run  func(context.Context) error
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
	ShutdownHooks           []ShutdownHook
	BackgroundTasks         []BackgroundTask
}

// APIService is a reusable HTTP API composition root for generated services.
type APIService struct {
	addr            string
	log             ports.Logger
	router          ports.HTTPRouter
	serverOptions   []ServerOption
	shutdownHooks   []ShutdownHook
	backgroundTasks []BackgroundTask
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
		addr:            addr,
		log:             log,
		router:          router,
		serverOptions:   append([]ServerOption(nil), config.ServerOptions...),
		shutdownHooks:   append([]ShutdownHook(nil), config.ShutdownHooks...),
		backgroundTasks: append([]BackgroundTask(nil), config.BackgroundTasks...),
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
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	taskErrCh, tasksDone := startBackgroundTasks(runCtx, s.backgroundTasks, s.log)
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- runServer(runCtx, HardenedServer(s.addr, s.Handler(), s.serverOptions...))
	}()

	var err error
	var taskErr error
	if taskErrCh == nil {
		err = <-serverErrCh
	} else {
		select {
		case err = <-serverErrCh:
		case taskErr = <-taskErrCh:
			cancel()
			err = <-serverErrCh
		}
	}
	cancel()
	<-tasksDone
	if taskErr == nil {
		taskErr = receiveBackgroundTaskError(taskErrCh)
	}
	err = errors.Join(err, taskErr)
	hookErr := runShutdownHooks(context.WithoutCancel(ctx), s.shutdownHooks)
	if err != nil {
		if hookErr != nil {
			s.log.Error("shutdown hook failed", "err", hookErr)
		}
		return err
	}
	return hookErr
}

func startBackgroundTasks(ctx context.Context, tasks []BackgroundTask, log ports.Logger) (<-chan error, <-chan struct{}) {
	active := make([]BackgroundTask, 0, len(tasks))
	for _, task := range tasks {
		if task.Run != nil {
			active = append(active, task)
		}
	}
	done := make(chan struct{})
	if len(active) == 0 {
		close(done)
		return nil, done
	}
	firstErr := make(chan error, 1)
	var wg sync.WaitGroup
	for _, task := range active {
		task := task
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := strings.TrimSpace(task.Name)
			if name == "" {
				name = "unnamed"
			}
			if err := task.Run(ctx); err != nil {
				if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
					return
				}
				if errors.Is(err, context.Canceled) {
					return
				}
				wrapped := fmt.Errorf("background task %s: %w", name, err)
				if log != nil {
					log.Error("background task failed", "name", name, "err", err)
				}
				select {
				case firstErr <- wrapped:
				default:
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(done)
	}()
	return firstErr, done
}

func receiveBackgroundTaskError(ch <-chan error) error {
	if ch == nil {
		return nil
	}
	select {
	case err := <-ch:
		return err
	default:
		return nil
	}
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

func runShutdownHooks(ctx context.Context, hooks []ShutdownHook) error {
	if ctx == nil {
		return errors.New("shutdown hook context is required")
	}
	for _, hook := range hooks {
		if hook.Hook == nil {
			continue
		}
		name := strings.TrimSpace(hook.Name)
		if name == "" {
			name = "unnamed"
		}
		hookCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := hook.Hook(hookCtx)
		cancel()
		if err != nil {
			return fmt.Errorf("shutdown hook %s: %w", name, err)
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
