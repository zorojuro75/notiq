package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zorojuro75/notiq/config"
	httpdelivery "github.com/zorojuro75/notiq/internal/delivery/http"
	"github.com/zorojuro75/notiq/internal/delivery/http/handler"
	"github.com/zorojuro75/notiq/internal/repository/postgres"
	"github.com/zorojuro75/notiq/internal/usecase/admin"
	"github.com/zorojuro75/notiq/internal/usecase/job"
	"github.com/zorojuro75/notiq/internal/usecase/webhook"
	"github.com/zorojuro75/notiq/pkg/logger"
	"github.com/zorojuro75/notiq/pkg/queue"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}
	logger.Init(cfg.Log.Level, cfg.Log.Format)

	// Admin routes are always mounted behind basic auth. Empty credentials would
	// leave them effectively unprotected, so refuse to start without them.
	if cfg.Admin.Username == "" || cfg.Admin.Password == "" {
		slog.Error("ADMIN_USERNAME and ADMIN_PASSWORD must both be set")
		os.Exit(1)
	}

	slog.Info("starting notiq API")

	db, err := config.NewPostgres(&cfg.DB)
	if err != nil {
		slog.Error("connecting to postgres", "error", err)
		os.Exit(1)
	}

	if err := config.RunMigrations(db); err != nil {
		slog.Error("running migrations", "error", err)
		os.Exit(1)
	}

	redisClient, err := config.NewRedis(&cfg.Redis)
	if err != nil {
		slog.Error("connecting to redis", "error", err)
		os.Exit(1)
	}

	// repositories
	jobRepo := postgres.NewJobRepository(db)
	webhookRepo := postgres.NewWebhookRepository(db)

	// queue client
	queueClient := queue.NewClient(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
	)
	defer queueClient.Close()

    // inspector
	inspector := queue.NewInspector(
        cfg.Redis.Addr, 
        cfg.Redis.Password, 
        cfg.Redis.DB,
    )
	defer inspector.Close()

	// use cases
	jobUC := job.NewJobUseCase(jobRepo, queueClient, inspector)
	webhookUC := webhook.NewWebhookUseCase(webhookRepo)
	adminUC := admin.NewAdminUseCase(jobRepo, queueClient, inspector)

	// handlers
	healthHandler := handler.NewHealthHandler(db, redisClient)
	jobHandler := handler.NewJobHandler(jobUC)
	webhookHandler := handler.NewWebhookHandler(webhookUC)
	adminHandler   := handler.NewAdminHandler(adminUC)

	// router
	router := httpdelivery.NewRouter(healthHandler, jobHandler, webhookHandler, adminHandler, cfg.Admin.Username, cfg.Admin.Password)

	addr := fmt.Sprintf(":%s", cfg.App.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Run the server in the background so main can block on the shutdown signal.
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// signal.NotifyContext cancels ctx when SIGTERM/SIGINT arrives.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	select {
	case err := <-serverErr:
		slog.Error("server error", "error", err)
		os.Exit(1)
	case <-ctx.Done():
		slog.Info("shutdown signal received — draining in-flight requests")
	}

	// Give in-flight requests a bounded window to finish before forcing exit.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed — forcing close", "error", err)
		_ = srv.Close()
		os.Exit(1)
	}

	slog.Info("graceful shutdown complete")
}
