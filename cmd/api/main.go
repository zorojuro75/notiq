package main

import (
	"fmt"
	"log"

	"github.com/zorojuro75/notiq/config"
	httpdelivery "github.com/zorojuro75/notiq/internal/delivery/http"
	"github.com/zorojuro75/notiq/internal/delivery/http/handler"
	"github.com/zorojuro75/notiq/internal/repository/postgres"
	"github.com/zorojuro75/notiq/internal/usecase/job"
	"github.com/zorojuro75/notiq/internal/usecase/webhook"
	"github.com/zorojuro75/notiq/pkg/queue"
	"github.com/zorojuro75/notiq/internal/usecase/admin"
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
	log.Printf("server starting on %s", addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
