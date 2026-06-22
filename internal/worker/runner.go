package worker

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/yuricunha/nostos/internal/config"
	"github.com/yuricunha/nostos/internal/database"
	"github.com/yuricunha/nostos/internal/id"
	"github.com/yuricunha/nostos/internal/tasks"
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
	r.poll(ctx, workerID)
	for {
		select {
		case <-ctx.Done():
			if r.logger != nil {
				r.logger.Info("worker stopping")
			}
			return nil
		case <-ticker.C:
			r.poll(ctx, workerID)
		}
	}
}

func (r *Runner) poll(ctx context.Context, workerID string) {
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
	for i := 0; i < r.cfg.Worker.Concurrency; i++ {
		if err := r.tasks.ClaimAndExecute(ctx, workerID); err != nil && r.logger != nil {
			r.logger.Error("failed to execute claimed task", "error", err)
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
