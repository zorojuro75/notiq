package main

import (
	"fmt"
	"log"

	"github.com/zorojuro75/notiq/config"
	httpdelivery "github.com/zorojuro75/notiq/internal/delivery/http"
	"github.com/zorojuro75/notiq/internal/delivery/http/handler"
	"github.com/zorojuro75/notiq/internal/repository/postgres"
	"github.com/zorojuro75/notiq/internal/usecase/job"
	"github.com/zorojuro75/notiq/pkg/queue"
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

    if err := config.RunMigrations(db); err != nil {
        log.Fatalf("running migrations: %v", err)
    }

    redisClient, err := config.NewRedis(&cfg.Redis)
    if err != nil {
        log.Fatalf("connecting to redis: %v", err)
    }

    // repositories
    jobRepo := postgres.NewJobRepository(db)

    // queue client
    queueClient := queue.NewClient(
        cfg.Redis.Addr,
        cfg.Redis.Password,
        cfg.Redis.DB,
    )
    defer queueClient.Close()

    // use cases
    jobUC := job.NewJobUseCase(jobRepo, queueClient)

    // handlers
    healthHandler := handler.NewHealthHandler(db, redisClient)
    jobHandler := handler.NewJobHandler(jobUC)

    // router
    router := httpdelivery.NewRouter(healthHandler, jobHandler)

    addr := fmt.Sprintf(":%s", cfg.App.Port)
    log.Printf("server starting on %s", addr)

    if err := router.Run(addr); err != nil {
        log.Fatalf("server error: %v", err)
    }
}