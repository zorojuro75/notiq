package main

import (
	"context"
	"log"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"github.com/zorojuro75/notiq/config"
	"github.com/zorojuro75/notiq/internal/repository/postgres"
	"github.com/zorojuro75/notiq/internal/worker"
	"github.com/zorojuro75/notiq/internal/worker/handlers"
	"github.com/zorojuro75/notiq/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	logger.Init(cfg.Log.Level, cfg.Log.Format)

	slog.Info("starting notiq worker")

	db, err := config.NewPostgres(&cfg.DB)
	if err != nil {
		slog.Error("connecting to postgres", "error", err)
		return
	}

	_, err = config.NewRedis(&cfg.Redis)
	if err != nil {
		slog.Error("connecting to redis", "error", err)
		return
	}

	// repositories
	jobRepo := postgres.NewJobRepository(db)

	// handlers
	emailHandler := handlers.NewEmailHandler(jobRepo)
	smsHandler := handlers.NewSMSHandler(jobRepo)
	webhookHandler := handlers.NewWebhookHandler(jobRepo)
	reportHandler := handlers.NewReportHandler(jobRepo)

	// pool
	pool := worker.NewPool(10, 100)
	pool.Start()

	// processor
	processor := worker.NewProcessor(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
		pool,
		emailHandler,
		smsHandler,
		webhookHandler,
		reportHandler,
	)

	if err := processor.Start(); err != nil {
		slog.Error("starting processor", "error", err)
		return
	}

	// signal.NotifyContext cancels ctx when SIGTERM/SIGINT arrives
	// it does not expose which signal fired — use the simpler pattern
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// block here until signal arrives
	<-ctx.Done()

	// ctx.Err() tells us why the context was cancelled
	slog.Info("shutdown signal received", "reason", ctx.Err().Error())

	shutdownTimeout := cfg.Worker.ShutdownTimeout
	if shutdownTimeout == 0 {
		shutdownTimeout = 30 * time.Second
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)

		slog.Info("step 1/2 — stopping processor")
		processor.Shutdown()
		slog.Info("step 1/2 — processor stopped")

		slog.Info("step 2/2 — draining worker pool")
		pool.Shutdown()
		slog.Info("step 2/2 — pool drained")
	}()

	select {
	case <-done:
		slog.Info("graceful shutdown complete")
	case <-shutdownCtx.Done():
		slog.Warn("shutdown timeout — forcing exit",
			"timeout", shutdownTimeout.String(),
			"note", "asynq reaper will re-queue stuck jobs on next startup",
		)
	}
}