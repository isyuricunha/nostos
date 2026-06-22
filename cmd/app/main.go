package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yuricunha/nostos/internal/api"
	"github.com/yuricunha/nostos/internal/auth"
	"github.com/yuricunha/nostos/internal/chat"
	"github.com/yuricunha/nostos/internal/config"
	"github.com/yuricunha/nostos/internal/database"
	"github.com/yuricunha/nostos/internal/health"
	"github.com/yuricunha/nostos/internal/logging"
	"github.com/yuricunha/nostos/internal/providers"
	"github.com/yuricunha/nostos/internal/worker"
)

var (
	version        = "0.1.0-dev"
	buildCommit    = "unknown"
	buildTimestamp = "unknown"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "nostos: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	command := "server"
	if len(args) > 1 {
		command = args[1]
	}

	switch command {
	case "version":
		fmt.Printf("nostos %s commit=%s built=%s\n", version, buildCommit, buildTimestamp)
		return nil
	case "server", "worker", "migrate", "doctor":
	default:
		return fmt.Errorf("unknown command %q; expected server, worker, migrate, doctor, or version", command)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := logging.New(cfg)
	ctx := signalContext()
	defer logger.Info("shutdown complete", "command", command)

	store, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer store.Close()

	if command == "doctor" {
		return runDoctor(ctx, cfg, store)
	}

	if err := database.RunMigrations(ctx, store, cfg.MigrationsDir); err != nil {
		return err
	}
	authRepo := auth.NewSQLRepository(store)
	authService := auth.NewService(authRepo, cfg)
	providerRepo := providers.NewSQLRepository(store)
	providerClient := providers.NewOpenAIClient()
	providerService := providers.NewService(cfg, providerRepo, authRepo, providerClient)
	chatRepo := chat.NewSQLRepository(store)
	chatService := chat.NewService(cfg, chatRepo, providerService, providerClient)
	if err := chatService.CleanupInterruptedRuns(ctx); err != nil {
		return err
	}
	if bootstrapped, err := authService.BootstrapOwner(ctx); err != nil {
		return err
	} else if bootstrapped {
		logger.Info("bootstrap owner created", "email", cfg.Security.BootstrapEmail)
	}

	switch command {
	case "migrate":
		logger.Info("migrations applied", "driver", cfg.Database.Driver)
		return nil
	case "worker":
		return runWorker(ctx, cfg, logger, store)
	case "server":
		return runServer(ctx, cfg, logger, store, authService, providerService, chatService)
	default:
		return nil
	}
}

func runServer(
	ctx context.Context,
	cfg config.Config,
	logger *slog.Logger,
	store *database.Store,
	authService *auth.Service,
	providerService *providers.Service,
	chatService *chat.Service,
) error {
	healthService := health.NewService(store, version, buildCommit, buildTimestamp)
	handler := api.NewRouter(api.RouterDeps{
		Config: cfg,
		Logger: logger,
		Health: healthService,
		Auth: api.AuthDeps{
			Config: cfg,
			Auth:   authService,
		},
		Providers: providerService,
		Chat:      chatService,
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddress(),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      cfg.Chat.DefaultTimeout + 30*time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server listening", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func runWorker(ctx context.Context, cfg config.Config, logger *slog.Logger, store *database.Store) error {
	runner := worker.NewRunner(worker.RunnerDeps{
		Config: cfg,
		Logger: logger,
		Store:  store,
	})
	return runner.Run(ctx)
}

func runDoctor(ctx context.Context, cfg config.Config, store *database.Store) error {
	status := map[string]any{
		"ok":        true,
		"version":   version,
		"commit":    buildCommit,
		"built_at":  buildTimestamp,
		"app_env":   cfg.AppEnv,
		"database":  cfg.Database.Driver,
		"data_dir":  cfg.DataDir,
		"timezone":  cfg.Timezone,
		"migrated":  false,
		"ready":     false,
		"web_dist":  cfg.WebDistDir,
		"log_level": cfg.LogLevel,
	}
	if err := store.DB.PingContext(ctx); err != nil {
		status["ok"] = false
		status["database_error"] = err.Error()
	} else {
		status["ready"] = true
	}
	return logging.WriteJSON(os.Stdout, status)
}

func signalContext() context.Context {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		stop()
	}()
	return ctx
}
