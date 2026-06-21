package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/zorojuro75/notiq/config"
	"github.com/zorojuro75/notiq/internal/repository/postgres"
	"github.com/zorojuro75/notiq/internal/usecase/notification"
	"github.com/zorojuro75/notiq/internal/worker"
	"github.com/zorojuro75/notiq/internal/worker/handlers"
	"github.com/zorojuro75/notiq/pkg/logger"
	"github.com/zorojuro75/notiq/pkg/queue"
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

	redisClient, err := config.NewRedis(&cfg.Redis)
	if err != nil {
		slog.Error("connecting to redis", "error", err)
		return
	}
	defer redisClient.Close()

	// repositories
	jobRepo := postgres.NewJobRepository(db)
	webhookRepo := postgres.NewWebhookRepository(db)

	// queue client — used by the dispatcher to enqueue webhook-delivery tasks
	queueClient := queue.NewClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	defer queueClient.Close()

	// dispatcher — fans terminal job events out to the owner's webhooks
	dispatcher := notification.NewDispatcher(webhookRepo, queueClient)

	// SSRF guard is on by default; WEBHOOK_ALLOW_PRIVATE=true disables it for
	// local testing against loopback receivers only. It protects every handler
	// that dials a user-controlled URL — both webhook jobs and webhook delivery.
	allowPrivate := strings.EqualFold(os.Getenv("WEBHOOK_ALLOW_PRIVATE"), "true")
	if allowPrivate {
		slog.Warn("WEBHOOK_ALLOW_PRIVATE enabled — webhook SSRF guard is OFF (dev only)")
	}

	// handlers
	emailHandler := handlers.NewEmailHandler(jobRepo, dispatcher)
	smsHandler := handlers.NewSMSHandler(jobRepo, dispatcher)
	webhookHandler := handlers.NewWebhookHandler(jobRepo, dispatcher, allowPrivate)
	reportHandler := handlers.NewReportHandler(jobRepo, dispatcher)
	deliveryHandler := handlers.NewWebhookDeliveryHandler(webhookRepo, allowPrivate)

	// processor — asynq bounds concurrency itself; no separate worker pool needed
	const workerConcurrency = 10
	processor := worker.NewProcessor(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
		workerConcurrency,
		emailHandler,
		smsHandler,
		webhookHandler,
		reportHandler,
		deliveryHandler,
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

		// asynq's Shutdown waits for in-flight handlers to finish and
		// re-queues anything still running past the deadline.
		slog.Info("stopping processor — draining in-flight tasks")
		processor.Shutdown()
		slog.Info("processor stopped")
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
