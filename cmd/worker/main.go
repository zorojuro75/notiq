package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

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

	jobRepo := postgres.NewJobRepository(db)

	emailHandler := handlers.NewEmailHandler(jobRepo)
	smsHandler := handlers.NewSMSHandler(jobRepo)
	webhookHandler := handlers.NewWebhookHandler(jobRepo)
	reportHandler := handlers.NewReportHandler(jobRepo)

	pool := worker.NewPool(10, 100)
	pool.Start()

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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	sig := <-quit
	log.Printf("received signal: %s", sig)

	processor.Shutdown()
	pool.Shutdown()
}
