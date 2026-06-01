package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"

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
	slog.Info("server starting", "addr", addr)

	if err := router.Run(addr); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
