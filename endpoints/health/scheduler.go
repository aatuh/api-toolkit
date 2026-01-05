package health

import (
	"context"
	"sync"
	"time"

	"github.com/aatuh/api-toolkit/ports"
)

// RefreshManager allows scheduling cached health refreshes.
type RefreshManager interface {
	RefreshAll(ctx context.Context) ports.DetailedHealthResponse
}

// SchedulerConfig configures periodic health refreshes.
type SchedulerConfig struct {
	Interval       time.Duration
	Logger         ports.Logger
	OnUpdate       func(ctx context.Context, result ports.DetailedHealthResponse)
	OnStatusChange func(ctx context.Context, prev, next ports.HealthStatus, result ports.DetailedHealthResponse)
}

// Scheduler periodically refreshes health checks to keep cache warm.
type Scheduler struct {
	manager        RefreshManager
	interval       time.Duration
	logger         ports.Logger
	onUpdate       func(ctx context.Context, result ports.DetailedHealthResponse)
	onStatusChange func(ctx context.Context, prev, next ports.HealthStatus, result ports.DetailedHealthResponse)
	mu             sync.Mutex
	lastStatus     ports.HealthStatus
}

// NewScheduler creates a scheduler that refreshes all health checks on an interval.
func NewScheduler(manager RefreshManager, config SchedulerConfig) *Scheduler {
	interval := config.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Scheduler{
		manager:        manager,
		interval:       interval,
		logger:         config.Logger,
		onUpdate:       config.OnUpdate,
		onStatusChange: config.OnStatusChange,
	}
}

// Start runs the scheduler until the context is canceled.
func (s *Scheduler) Start(ctx context.Context) {
	if s == nil || s.manager == nil {
		return
	}
	s.runOnce(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context) {
	result := s.manager.RefreshAll(ctx)
	if s.onUpdate != nil {
		s.onUpdate(ctx, result)
	}

	s.mu.Lock()
	prev := s.lastStatus
	s.lastStatus = result.Status
	s.mu.Unlock()

	if prev != "" && prev != result.Status {
		if s.logger != nil {
			if result.Status == ports.HealthStatusHealthy {
				s.logger.Info("health status recovered", "from", prev)
			} else {
				s.logger.Warn("health status changed", "from", prev, "to", result.Status)
			}
		}
		if s.onStatusChange != nil {
			s.onStatusChange(ctx, prev, result.Status, result)
		}
	}
}
