package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/yuricunha/nostos/internal/config"
	"github.com/yuricunha/nostos/internal/database"
)

type RunnerDeps struct {
	Config config.Config
	Logger *slog.Logger
	Store  *database.Store
}

type Runner struct {
	cfg    config.Config
	logger *slog.Logger
	store  *database.Store
}

func NewRunner(deps RunnerDeps) *Runner {
	return &Runner{
		cfg:    deps.Config,
		logger: deps.Logger,
		store:  deps.Store,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.cfg.Worker.PollInterval)
	defer ticker.Stop()
	if r.logger != nil {
		r.logger.Info("worker started", "concurrency", r.cfg.Worker.Concurrency, "poll_interval", r.cfg.Worker.PollInterval.String())
	}
	for {
		select {
		case <-ctx.Done():
			if r.logger != nil {
				r.logger.Info("worker stopping")
			}
			return nil
		case <-ticker.C:
			if r.store != nil && r.store.DB != nil {
				_ = r.store.DB.PingContext(ctx)
			}
		}
	}
}
