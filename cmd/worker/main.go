package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/zorojuro75/notiq/config"
	"github.com/zorojuro75/notiq/internal/repository/postgres"
	"github.com/zorojuro75/notiq/internal/worker"
	"github.com/zorojuro75/notiq/internal/worker/handlers"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	db, err := config.NewPostgres(&cfg.DB)
	if err != nil {
		log.Fatalf("connecting to postgres: %v", err)
	}

	_, err = config.NewRedis(&cfg.Redis)
	if err != nil {
		log.Fatalf("connecting to redis: %v", err)
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
		log.Fatalf("starting processor: %v", err)
	}

	// signal.NotifyContext gives us a context that cancels on SIGTERM/SIGINT
	// gets the shutdown signal automatically
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// block here until signal arrives
	<-ctx.Done()

	log.Printf("shutdown signal received — starting graceful shutdown")

	shutdownTimeout := cfg.Worker.ShutdownTimeout
	if shutdownTimeout == 0 {
		shutdownTimeout = 30 * time.Second // safe default
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// run the shutdown sequence in a goroutine so we can race it against the timeout
	done := make(chan struct{})
	go func() {
		defer close(done)

		// no new tasks enter the system after this
		log.Println("step 1/2 — stopping processor")
		processor.Shutdown()
		log.Println("step 1/2 — processor stopped")

		// blocks until every in-flight job finishes
		log.Println("step 2/2 — draining worker pool")
		pool.Shutdown()
		log.Println("step 2/2 — pool drained")
	}()

	select {
	case <-done:
		log.Println("graceful shutdown complete")
	case <-shutdownCtx.Done():
		log.Printf("shutdown timeout after %s — forcing exit (some jobs may still be processing)", shutdownTimeout)
		log.Println("asynq reaper will re-queue stuck jobs on next startup")
	}
}