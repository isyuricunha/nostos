package worker

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/isyuricunha/nostos/internal/config"
	"github.com/isyuricunha/nostos/internal/database"
	"github.com/isyuricunha/nostos/internal/id"
	"github.com/isyuricunha/nostos/internal/tasks"
)

type RunnerDeps struct {
	Config config.Config
	Logger *slog.Logger
	Store  *database.Store
	Tasks  *tasks.Service
}

type Runner struct {
	cfg    config.Config
	logger *slog.Logger
	store  *database.Store
	tasks  *tasks.Service
}

func NewRunner(deps RunnerDeps) *Runner {
	return &Runner{
		cfg:    deps.Config,
		logger: deps.Logger,
		store:  deps.Store,
		tasks:  deps.Tasks,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.cfg.Worker.PollInterval)
	defer ticker.Stop()
	workerID := workerID()
	if r.logger != nil {
		r.logger.Info("worker started", "worker_id", workerID, "concurrency", r.cfg.Worker.Concurrency, "poll_interval", r.cfg.Worker.PollInterval.String())
	}
	r.pollCoordinator(ctx)
	var wg sync.WaitGroup
	concurrency := r.cfg.Worker.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	for index := 0; index < concurrency; index++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			r.workerLoop(ctx, workerID, slot)
		}(index)
	}
	defer wg.Wait()
	for {
		select {
		case <-ctx.Done():
			if r.logger != nil {
				r.logger.Info("worker stopping")
			}
			return nil
		case <-ticker.C:
			r.pollCoordinator(ctx)
		}
	}
}

func (r *Runner) pollCoordinator(ctx context.Context) {
	if r.store != nil && r.store.DB != nil {
		_ = r.store.DB.PingContext(ctx)
	}
	if r.tasks == nil {
		return
	}
	if err := r.tasks.RecoverExpiredLeases(ctx); err != nil && r.logger != nil {
		r.logger.Error("failed to recover expired task leases", "error", err)
	}
	if err := r.tasks.EnqueueDueSchedules(ctx); err != nil && r.logger != nil {
		r.logger.Error("failed to enqueue due task schedules", "error", err)
	}
}

func (r *Runner) workerLoop(ctx context.Context, workerID string, slot int) {
	ticker := time.NewTicker(r.cfg.Worker.PollInterval)
	defer ticker.Stop()
	for {
		if r.tasks != nil {
			if err := r.tasks.ClaimAndExecute(ctx, workerID); err != nil && r.logger != nil {
				r.logger.Error("failed to execute claimed task", "error", err, "worker_slot", slot)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func workerID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "worker"
	}
	return hostname + "-" + id.New()
}
