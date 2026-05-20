package main

import (
    "fmt"
    "log"

    "github.com/zorojuro75/notiq/config"
    httpdelivery "github.com/zorojuro75/notiq/internal/delivery/http"
    "github.com/zorojuro75/notiq/internal/delivery/http/handler"
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

    // handlers
    healthHandler := handler.NewHealthHandler(db, redisClient)

    // router
    router := httpdelivery.NewRouter(healthHandler)

    addr := fmt.Sprintf(":%s", cfg.App.Port)
    log.Printf("server starting on %s", addr)

    if err := router.Run(addr); err != nil {
        log.Fatalf("server error: %v", err)
    }
}